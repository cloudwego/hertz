/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik Dubbelboer, FastHTTP Authors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package http1

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/hertz/pkg/app/client/retry"
	"github.com/cloudwego/hertz/pkg/common/config"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/dialer"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/client"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

var errDialTimeout = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "dial timeout")

func TestHostClientMaxConnWaitTimeoutWithEarlierDeadline(t *testing.T) {
	var (
		emptyBodyCount uint8
		wg             sync.WaitGroup
		// make deadline reach earlier than conns wait timeout
		timeout = 10 * time.Millisecond
	)

	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return mock.SlowReadDialer(addr)
			}),
			MaxConns:           1,
			MaxConnWaitTimeout: 50 * time.Millisecond,
		},
		Addr: "foobar",
	}

	var errTimeoutCount uint32
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := protocol.AcquireRequest()
			req.SetRequestURI("http://foobar/baz")
			req.Header.SetMethod(consts.MethodPost)
			req.SetBodyString("bar")
			resp := protocol.AcquireResponse()

			if err := c.DoDeadline(context.Background(), req, resp, time.Now().Add(timeout)); err != nil {
				if !errors.Is(err, errs.ErrTimeout) {
					t.Errorf("unexpected error: %s. Expecting %s", err, errs.ErrTimeout)
				}
				atomic.AddUint32(&errTimeoutCount, 1)
			} else {
				if resp.StatusCode() != consts.StatusOK {
					t.Errorf("unexpected status code %d. Expecting %d", resp.StatusCode(), consts.StatusOK)
				}

				body := resp.Body()
				if string(body) != "foo" {
					t.Errorf("unexpected body %q. Expecting %q", body, "abcd")
				}
			}
		}()
	}
	wg.Wait()

	c.connsLock.Lock()
	for {
		w := c.connsWait.popFront()
		if w == nil {
			break
		}
		w.mu.Lock()
		if w.err != nil && !errors.Is(w.err, errs.ErrNoFreeConns) {
			t.Errorf("unexpected error: %s. Expecting %s", w.err, errs.ErrNoFreeConns)
		}
		w.mu.Unlock()
	}
	c.connsLock.Unlock()
	if errTimeoutCount == 0 {
		t.Errorf("unexpected errTimeoutCount: %d. Expecting > 0", errTimeoutCount)
	}

	if emptyBodyCount > 0 {
		t.Fatalf("at least one request body was empty")
	}
}

func TestReadHeaderErr(t *testing.T) {
	ln, _ := net.Listen("tcp", "localhost:0")
	defer ln.Close()
	svr := http.Server{}
	svr.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj := w.(http.Hijacker)
		conn, rw, err := hj.Hijack()
		assert.Nil(t, err)
		defer conn.Close()
		rw.Write([]byte("HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Ty"))
		rw.Flush()
	})
	go svr.Serve(ln)

	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetRequestURI("http://" + ln.Addr().String())

	resp := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(resp)
	c := &HostClient{
		Addr: ln.Addr().String(),
		ClientOptions: &ClientOptions{
			Dialer: dialer.DefaultDialer(),
		},
	}
	err := c.Do(context.Background(), req, resp)
	assert.NotNil(t, err)
}

func TestResponseReadBodyStream(t *testing.T) {
	// small body
	genBody := "abcdef4343"
	s := "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Length: 5\r\n\r\n"
	testContinueReadResponseBodyStream(t, s, genBody, 10, 5, 0, 5)
	testContinueReadResponseBodyStream(t, s, genBody, 1, 5, 0, 0)

	// big body (> 8193)
	s1 := "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Length: 9216\r\nContent-Type: foo/bar\r\n\r\n"
	genBody = strings.Repeat("1", 9*1024)
	testContinueReadResponseBodyStream(t, s1, genBody, 10*1024, 5*1024, 4*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 10*1024, 1*1024, 8*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 10*1024, 9*1024, 0*1024, 0)

	// normal stream
	testContinueReadResponseBodyStream(t, s1, genBody, 1*1024, 5*1024, 4*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 1*1024, 1*1024, 8*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 1*1024, 9*1024, 0*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 5, 5*1024, 4*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 5, 1*1024, 8*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 5, 9*1024, 0, 0)

	// critical point
	testContinueReadResponseBodyStream(t, s1, genBody, 8*1024+1, 5*1024, 4*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 8*1024+1, 1*1024, 8*1024, 0)
	testContinueReadResponseBodyStream(t, s1, genBody, 8*1024+1, 9*1024, 0*1024, 0)

	// chunked body
	s2 := "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n3\r\nabc\r\n5\r\n12345\r\n0\r\n\r\ntrail"
	testContinueReadResponseBodyStream(t, s2, "", 10*1024, 3, 5, 5)
	s3 := "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n3\r\nabc\r\n5\r\n12345\r\n0\r\n\r\n"
	testContinueReadResponseBodyStream(t, s3, "", 10*1024, 3, 5, 0)
}

func testContinueReadResponseBodyStream(t *testing.T, header, body string, maxBodySize, firstRead, leftBytes, bytesLeftInReader int) {
	mr := netpoll.NewReader(bytes.NewBufferString(header + body))
	var r protocol.Response
	if err := resp.ReadHeaderBodyStream(&r, mr, maxBodySize, nil); err != nil {
		t.Fatalf("error when reading request body stream: %s", err)
	}
	fRead := firstRead
	streamRead := make([]byte, fRead)
	sR, _ := r.BodyStream().Read(streamRead)

	if sR != firstRead {
		t.Fatalf("should read %d from stream body, but got %d", firstRead, sR)
	}

	leftB, _ := ioutil.ReadAll(r.BodyStream())
	if len(leftB) != leftBytes {
		t.Fatalf("should left %d bytes from stream body, but left %d", leftBytes, len(leftB))
	}
	if r.Header.ContentLength() > 0 {
		gotBody := append(streamRead, leftB...)
		if !bytes.Equal([]byte(body[:r.Header.ContentLength()]), gotBody) {
			t.Fatalf("body read from stream is not equal to the origin. Got: %s", gotBody)
		}
	}

	left, _ := mr.Next(mr.Len())

	if len(left) != bytesLeftInReader {
		fmt.Printf("##########header:%s,body:%s,%d:max,first:%d,left:%d,leftin:%d\n", header, body, maxBodySize, firstRead, leftBytes, bytesLeftInReader)
		fmt.Printf("##########left: %s\n", left)
		t.Fatalf("should left %d bytes in original reader. got %q", bytesLeftInReader, len(left))
	}
}

type dialerFunc func(network, addr string, timeout time.Duration) (network.Conn, error)

func (f dialerFunc) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	return f(network, address, timeout)
}

func (_ dialerFunc) DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error) {
	return nil, nil
}

func (_ dialerFunc) AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	return nil, nil
}

type slowDialer struct {
	network.Dialer
}

func (s *slowDialer) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	time.Sleep(timeout)
	return nil, errDialTimeout
}

func TestTimeoutPriority(t *testing.T) {
	rtimeoutDialer := dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
		return mock.SlowReadDialer(addr)
	})
	wtimeoutDialer := dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
		return mock.SlowWriteDialer(addr)
	})

	noopRequestOpt := config.RequestOption{F: func(o *config.RequestOptions) {}}

	tests := []struct {
		name        string
		dialer      network.Dialer
		clientOpts  *ClientOptions
		reqOpt      config.RequestOption
		expectDelay time.Duration
		expectedErr error
	}{
		// ReadTimeout cases
		{
			"ReadTimeout_cli_60ms_req_100ms",
			rtimeoutDialer,
			&ClientOptions{ReadTimeout: 60 * time.Millisecond},
			config.WithReadTimeout(100 * time.Millisecond),
			100 * time.Millisecond,
			mock.ErrReadTimeout,
		},
		{
			"ReadTimeout_cli_100ms_req_60ms",
			rtimeoutDialer,
			&ClientOptions{ReadTimeout: 100 * time.Millisecond},
			config.WithReadTimeout(60 * time.Millisecond),
			60 * time.Millisecond,
			mock.ErrReadTimeout,
		},
		{
			"ReadTimeout_cli_unset_req_60ms",
			rtimeoutDialer,
			&ClientOptions{},
			config.WithReadTimeout(60 * time.Millisecond),
			60 * time.Millisecond,
			mock.ErrReadTimeout,
		},
		{
			"ReadTimeout_cli_60ms_req_unset",
			rtimeoutDialer,
			&ClientOptions{ReadTimeout: 60 * time.Millisecond},
			noopRequestOpt,
			60 * time.Millisecond,
			mock.ErrReadTimeout,
		},
		// WriteTimeout cases
		{
			"WriteTimeout_cli_100ms_req_150ms",
			wtimeoutDialer,
			&ClientOptions{WriteTimeout: 100 * time.Millisecond},
			config.WithWriteTimeout(150 * time.Millisecond),
			150 * time.Millisecond,
			mock.ErrWriteTimeout,
		},
		{
			"WriteTimeout_cli_150ms_req_100ms",
			wtimeoutDialer,
			&ClientOptions{WriteTimeout: 150 * time.Millisecond},
			config.WithWriteTimeout(100 * time.Millisecond),
			100 * time.Millisecond,
			mock.ErrWriteTimeout,
		},
		{
			"WriteTimeout_cli_unset_req_120ms",
			wtimeoutDialer,
			&ClientOptions{},
			config.WithWriteTimeout(120 * time.Millisecond),
			120 * time.Millisecond,
			mock.ErrWriteTimeout,
		},
		{
			"WriteTimeout_cli_120ms_req_unset",
			wtimeoutDialer,
			&ClientOptions{WriteTimeout: 120 * time.Millisecond},
			noopRequestOpt,
			120 * time.Millisecond,
			mock.ErrWriteTimeout,
		},
		// DialTimeout cases
		{
			"DialTimeout_cli_60ms_req_100ms",
			&slowDialer{},
			&ClientOptions{DialTimeout: 60 * time.Millisecond},
			config.WithDialTimeout(100 * time.Millisecond),
			100 * time.Millisecond,
			errDialTimeout,
		},
		{
			"DialTimeout_cli_100ms_req_60ms",
			&slowDialer{},
			&ClientOptions{DialTimeout: 100 * time.Millisecond},
			config.WithDialTimeout(60 * time.Millisecond),
			60 * time.Millisecond,
			errDialTimeout,
		},
		{
			"DialTimeout_cli_unset_req_60ms",
			&slowDialer{},
			&ClientOptions{},
			config.WithDialTimeout(60 * time.Millisecond),
			60 * time.Millisecond,
			errDialTimeout,
		},
		{
			"DialTimeout_cli_60ms_req_unset",
			&slowDialer{},
			&ClientOptions{DialTimeout: 60 * time.Millisecond},
			noopRequestOpt,
			60 * time.Millisecond,
			errDialTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.clientOpts.Dialer = tt.dialer
			c := &HostClient{ClientOptions: tt.clientOpts, Addr: "foobar"}

			req := protocol.AcquireRequest()
			req.SetRequestURI("http://foobar/baz")
			req.SetOptions(tt.reqOpt)

			start := time.Now()
			err := c.Do(context.Background(), req, protocol.AcquireResponse())
			duration := time.Since(start)

			assert.DeepEqual(t, tt.expectedErr, err)

			// Check if duration is within expected delay ±30ms
			tolerance := 30 * time.Millisecond
			if !(duration >= tt.expectDelay-tolerance && duration <= tt.expectDelay+tolerance) {
				t.Errorf("Duration %v not within expected %v ±%v", duration, tt.expectDelay, tolerance)
			}
		})
	}
}

func TestDoNonNilReqResp(t *testing.T) {
	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return mock.NewConn("HTTP/1.1 400 OK\nContent-Length: 6\n\n123456"), nil
			}),
		},
	}
	req := protocol.AcquireRequest()
	resp := protocol.AcquireResponse()
	req.SetHost("foobar")
	retry, err := c.doNonNilReqResp(req, resp)
	assert.False(t, retry)
	assert.Nil(t, err)
	assert.DeepEqual(t, resp.StatusCode(), 400)
	assert.DeepEqual(t, resp.Body(), []byte("123456"))
}

func TestDoNonNilReqResp_WriteErr(t *testing.T) {
	c := &HostClient{
		ClientOptions: &ClientOptions{},
	}
	req := protocol.AcquireRequest()
	resp := protocol.AcquireResponse()
	req.SetHost("foobar")
	req.SetConnectionClose() // won't reuse the conn

	// 200 with write err, will return write err
	c.ClientOptions.Dialer = dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
		return &writeErrConn{mock.NewConn("HTTP/1.1 200 OK\nContent-Length: 6\n\n123456")}, nil
	})
	retry, err := c.doNonNilReqResp(req, resp)
	assert.True(t, retry)
	assert.NotNil(t, err)

	c = &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return &writeErrConn{mock.NewConn("HTTP/1.1 400 OK\nContent-Length: 6\n\n123456")}, nil
			}),
		},
	}

	// 400 with write err, will NOT return write err
	c.ClientOptions.Dialer = dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
		return &writeErrConn{mock.NewConn("HTTP/1.1 400 OK\nContent-Length: 6\n\n123456")}, nil
	})
	retry, err = c.doNonNilReqResp(req, resp)
	assert.False(t, retry)
	assert.Nil(t, err)
	assert.DeepEqual(t, resp.StatusCode(), 400)
	assert.DeepEqual(t, resp.Body(), []byte("123456"))
}

func TestDoNonNilReqResp_TLS(t *testing.T) {
	const (
		dialTimeout = 123 * time.Millisecond
		dev         = 10 * time.Millisecond
	)
	conn := mock.NewConn("HTTP/1.1 200 OK\nContent-Length: 5\n\n54321")
	tlsconn := mock.NewTLSConn(conn)
	c := &HostClient{
		IsTLS: true,
		ClientOptions: &ClientOptions{
			DialTimeout: dialTimeout,
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return tlsconn, nil
			}),
		},
	}
	req := protocol.AcquireRequest()
	resp := protocol.AcquireResponse()
	req.SetHost("foobar")

	// HandshakeErr != nil
	tlsconn.HandshakeErr = errors.New("testerr")
	retry, err := c.doNonNilReqResp(req, resp)
	assert.True(t, retry)
	assert.True(t, err == tlsconn.HandshakeErr)
	if diff := conn.GetReadTimeout() - dialTimeout; diff < -dev || diff > dev {
		t.Fatal("unexpected timeout. got", conn.GetReadTimeout(), "expect", dialTimeout)
	}
	assert.True(t, conn.GetReadTimeout() == conn.GetWriteTimeout())

	// HandshakeErr == nil
	tlsconn.HandshakeErr = nil
	retry, err = c.doNonNilReqResp(req, resp)
	assert.False(t, retry)
	assert.Nil(t, err)
	assert.DeepEqual(t, resp.StatusCode(), 200)
	assert.DeepEqual(t, resp.Body(), []byte("54321"))
}

func TestDoNonNilReqResp_Err(t *testing.T) {
	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return peekErrConn{writeErrConn{mock.NewConn("")}}, nil
			}),
		},
	}
	req := protocol.AcquireRequest()
	resp := protocol.AcquireResponse()
	req.SetHost("foobar")
	retry, err := c.doNonNilReqResp(req, resp)
	assert.True(t, retry)
	assert.NotNil(t, err)
	assert.Assert(t, err == errs.ErrConnectionClosed, err) // returned by writeErrConn
}

func doGET(t *testing.T, addr, path string) *protocol.Response {
	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)
	req.SetRequestURI("http://" + addr + path)

	resp := protocol.AcquireResponse()
	c := &HostClient{
		Addr: addr,
		ClientOptions: &ClientOptions{
			Dialer: dialer.DefaultDialer(),
		},
	}
	err := c.Do(context.Background(), req, resp)
	assert.Nil(t, err)
	return resp
}

func TestStreamResponse_EventStream(t *testing.T) {
	ln, _ := net.Listen("tcp", "localhost:0")
	defer ln.Close()
	svr := http.Server{}
	svr.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		for i := 0; i < 5; i++ {
			_, err := w.Write([]byte(fmt.Sprintf("data:%d", i)))
			assert.Nil(t, err)
			f.Flush() // Transfer-Encoding chunked
			time.Sleep(20 * time.Millisecond)
		}
	})
	go svr.Serve(ln)

	resp := doGET(t, ln.Addr().String(), "/")
	defer protocol.ReleaseResponse(resp)
	assert.Assert(t, resp.IsBodyStream())
	r := resp.BodyStream()
	b := make([]byte, 10)
	for i := 0; i < 5; i++ {
		n, err := r.Read(b)
		assert.Nil(t, err)
		assert.Assert(t, string(b[:n]) == fmt.Sprintf("data:%d", i))
	}
}

func TestStreamResponse_ConnUpgrade(t *testing.T) {
	ln, _ := net.Listen("tcp", "localhost:0")
	defer ln.Close()
	svr := http.Server{}
	svr.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
			return
		}
		conn, rw, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		_, err = rw.WriteString("HTTP/1.1 101 Switching Protocols\nConnection: Upgrade\n\n")
		assert.Nil(t, err)
		assert.Nil(t, rw.Flush())
		b := make([]byte, 100)
		for { // echo with "echo:" prefix
			n, err := rw.Read(b)
			if err != nil {
				return
			}
			_, err = rw.Write([]byte("echo:" + string(b[:n])))
			if err != nil {
				return
			}
			_ = rw.Flush()
		}
	})
	go svr.Serve(ln)

	resp := doGET(t, ln.Addr().String(), "/")
	defer protocol.ReleaseResponse(resp)
	assert.DeepEqual(t, resp.StatusCode(), 101)

	s := resp.BodyStream()
	assert.NotNil(t, s)
	conn, err := resp.Hijack()
	assert.Nil(t, err)

	b := make([]byte, 100)
	_, _ = conn.Write(append(b[:0], "hello"...))
	n, err := s.Read(b) // same as conn.Read
	assert.Nil(t, err)
	assert.DeepEqual(t, string(b[:n]), "echo:hello")
}

func TestStateObserve(t *testing.T) {
	syncState := struct {
		mu    sync.Mutex
		state config.ConnPoolState
	}{}
	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return mock.SlowReadDialer(addr)
			}),
			StateObserve: func(hcs config.HostClientState) {
				syncState.mu.Lock()
				defer syncState.mu.Unlock()
				syncState.state = hcs.ConnPoolState()
			},
			ObservationInterval: 50 * time.Millisecond,
		},
		Addr:   "foobar",
		closed: make(chan struct{}),
	}

	c.SetDynamicConfig(&client.DynamicConfig{
		Addr: utils.AddMissingPort(c.Addr, true),
	})

	time.Sleep(500 * time.Millisecond)
	assert.Nil(t, c.Close())
	syncState.mu.Lock()
	assert.DeepEqual(t, "foobar:443", syncState.state.Addr)
	syncState.mu.Unlock()
}

func TestCachedTLSConfig(t *testing.T) {
	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return mock.SlowReadDialer(addr)
			}),
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Addr:  "foobar",
		IsTLS: true,
	}

	cfg1 := c.cachedTLSConfig("foobar")
	cfg2 := c.cachedTLSConfig("baz")
	assert.NotEqual(t, cfg1, cfg2)
	cfg3 := c.cachedTLSConfig("foobar")
	assert.DeepEqual(t, cfg1, cfg3)
}

func TestRetry(t *testing.T) {
	var times int32
	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				times++
				if times < 3 {
					return &retryConn{
						Conn: mock.NewConn(""),
					}, nil
				}
				return mock.NewConn("HTTP/1.1 200 OK\r\nContent-Length: 10\r\nContent-Type: foo/bar\r\n\r\n0123456789"), nil
			}),
			RetryConfig: &retry.Config{
				MaxAttemptTimes: 5,
				Delay:           time.Millisecond * 10,
			},
			RetryIfFunc: func(req *protocol.Request, resp *protocol.Response, err error) bool {
				return resp.Header.ContentLength() != 10
			},
		},
		Addr: "foobar",
	}

	req := protocol.AcquireRequest()
	req.SetRequestURI("http://foobar/baz")
	req.SetOptions(config.WithWriteTimeout(time.Millisecond * 100))
	resp := protocol.AcquireResponse()

	ch := make(chan error, 1)
	go func() {
		ch <- c.Do(context.Background(), req, resp)
	}()
	select {
	case <-time.After(time.Second * 2):
		t.Fatalf("should use writeTimeout in request options")
	case err := <-ch:
		assert.Nil(t, err)
		assert.True(t, times == 3)
		assert.DeepEqual(t, resp.StatusCode(), 200)
		assert.DeepEqual(t, resp.Body(), []byte("0123456789"))
	}
}

// mockConn for getting error when write binary data.
type writeErrConn struct {
	network.Conn
}

func (w writeErrConn) WriteBinary(b []byte) (n int, err error) {
	return 0, errs.ErrConnectionClosed
}

type peekErrConn struct {
	network.Conn
}

func (c peekErrConn) Peek(n int) ([]byte, error) {
	return nil, errors.New("peek err")
}

type retryConn struct {
	network.Conn
}

func (w retryConn) SetWriteTimeout(t time.Duration) error {
	return errors.New("should retry")
}

func TestConnInPoolRetry(t *testing.T) {
	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return mock.NewOneTimeConn("HTTP/1.1 200 OK\r\nContent-Length: 10\r\nContent-Type: foo/bar\r\n\r\n0123456789"), nil
			}),
		},
		Addr: "foobar",
	}

	req := protocol.AcquireRequest()
	req.SetRequestURI("http://foobar/baz")
	req.SetOptions(config.WithWriteTimeout(time.Millisecond * 100))
	resp := protocol.AcquireResponse()

	logbuf := &bytes.Buffer{}
	hlog.SetOutput(logbuf)

	err := c.Do(context.Background(), req, resp)
	assert.Nil(t, err)
	assert.DeepEqual(t, resp.StatusCode(), 200)
	assert.DeepEqual(t, string(resp.Body()), "0123456789")
	assert.True(t, logbuf.String() == "")
	protocol.ReleaseResponse(resp)
	resp = protocol.AcquireResponse()
	err = c.Do(context.Background(), req, resp)
	assert.Nil(t, err)
	assert.DeepEqual(t, resp.StatusCode(), 200)
	assert.DeepEqual(t, string(resp.Body()), "0123456789")
	assert.True(t, strings.Contains(logbuf.String(), "Client connection attempt times: 1"))
}

func TestConnNotRetry(t *testing.T) {
	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return mock.NewBrokenConn(""), nil
			}),
		},
		Addr: "foobar",
	}

	req := protocol.AcquireRequest()
	req.SetRequestURI("http://foobar/baz")
	req.SetOptions(config.WithWriteTimeout(time.Millisecond * 100))
	resp := protocol.AcquireResponse()
	logbuf := &bytes.Buffer{}
	hlog.SetOutput(logbuf)
	err := c.Do(context.Background(), req, resp)
	assert.DeepEqual(t, errs.ErrConnectionClosed, err)
	assert.True(t, logbuf.String() == "")
	protocol.ReleaseResponse(resp)
}

type countCloseConn struct {
	network.Conn
	isClose bool
}

func (c *countCloseConn) Close() error {
	c.isClose = true
	return nil
}

func newCountCloseConn(s string) *countCloseConn {
	return &countCloseConn{
		Conn: mock.NewConn(s),
	}
}

func TestStreamNoContent(t *testing.T) {
	conn := newCountCloseConn("HTTP/1.1 204 Foo Bar\r\nContent-Type: aab\r\nTrailer: Foo\r\nContent-Encoding: deflate\r\nTransfer-Encoding: chunked\r\n\r\n0\r\nFoo: bar\r\n\r\nHTTP/1.2")

	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return conn, nil
			}),
		},
		Addr: "foobar",
	}

	c.ResponseBodyStream = true

	req := protocol.AcquireRequest()
	req.SetRequestURI("http://foobar/baz")
	req.Header.SetConnectionClose(true)
	resp := protocol.AcquireResponse()

	c.Do(context.Background(), req, resp)

	assert.True(t, conn.isClose)
}

func TestDialTimeout(t *testing.T) {
	c := &HostClient{
		ClientOptions: &ClientOptions{
			DialTimeout: time.Second * 10,
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				assert.DeepEqual(t, time.Second*10, timeout)
				return nil, errors.New("test error")
			}),
		},
		Addr: "foobar",
	}

	req := protocol.AcquireRequest()
	req.SetRequestURI("http://foobar/baz")
	resp := protocol.AcquireResponse()

	c.Do(context.Background(), req, resp)
}

func TestContextNil(t *testing.T) {
	defer func() {
		v := recover()
		assert.NotNil(t, v)
		assert.True(t, fmt.Sprint(v) == "ctx is nil")
	}()
	c := &HostClient{}
	c.Do(nil, nil, nil) //nolint:staticcheck // SA1012: do not pass a nil Context
}

func TestCalcimeout(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		deadline time.Time
		timeout  time.Duration
		expected time.Duration
	}{
		{"zero deadline, positive timeout", time.Time{}, 5 * time.Second, 5 * time.Second},
		{"zero deadline, zero timeout", time.Time{}, 0, 0},
		{"zero deadline, negative timeout", time.Time{}, -1 * time.Second, 0},
		{"future deadline, zero timeout", now.Add(10 * time.Second), 0, 10 * time.Second},
		{"future deadline, positive timeout (deadline < timeout)", now.Add(3 * time.Second), 5 * time.Second, 3 * time.Second},
		{"future deadline, positive timeout (deadline > timeout)", now.Add(8 * time.Second), 5 * time.Second, 5 * time.Second},
		{"past deadline, zero timeout", now.Add(-5 * time.Second), 0, -1},
		{"past deadline, positive timeout", now.Add(-5 * time.Second), time.Second, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calcTimeout(tt.deadline, tt.timeout)
			diff := result - tt.expected
			if diff < -50*time.Millisecond || diff > 50*time.Millisecond {
				t.Errorf("calcTimeout(%v, %v) = %v, expected %v",
					tt.deadline, tt.timeout, result, tt.expected)
			}
		})
	}
}

type mockConnClosed struct {
	closed bool
	network.Conn
}

func (m *mockConnClosed) Close() error {
	m.closed = true
	return m.Conn.Close()
}

// mock CRLF attacking
func TestDoNonNilReqResp_releaseConn(t *testing.T) {
	respStr := "HTTP/1.1 400 OK\nContent-Length: 6\n\n123456"
	conn := &mockConnClosed{Conn: mock.NewConn(respStr + respStr)}
	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer: dialerFunc(func(network, addr string, timeout time.Duration) (network.Conn, error) {
				return conn, nil
			}),
		},
	}
	req := protocol.AcquireRequest()
	resp := protocol.AcquireResponse()
	req.SetHost("foobar")
	retry, err := c.doNonNilReqResp(req, resp)
	assert.False(t, retry)
	assert.Nil(t, err)
	assert.DeepEqual(t, resp.StatusCode(), 400)
	assert.DeepEqual(t, resp.Body(), []byte("123456"))
	assert.True(t, conn.closed)
}
