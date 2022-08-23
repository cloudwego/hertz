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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
	"github.com/cloudwego/netpoll"
)

func TestHostClientMaxConnWaitTimeoutWithEarlierDeadline(t *testing.T) {
	var (
		emptyBodyCount uint8
		wg             sync.WaitGroup
		// make deadline reach earlier than conns wait timeout
		timeout = 10 * time.Millisecond
	)

	c := &HostClient{
		ClientOptions: &ClientOptions{
			Addr: "foobar",
			Dialer: newSlowConnDialer(func(network, addr string) (network.Conn, error) {
				return mock.SlowReadDialer(addr)
			}),
			MaxConns:           1,
			MaxConnWaitTimeout: 50 * time.Millisecond,
		},
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
		if w.err != nil && !errors.Is(w.err, errNoFreeConns) {
			t.Errorf("unexpected error: %s. Expecting %s", w.err, errNoFreeConns)
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
	if err := resp.ReadBodyStream(&r, mr, maxBodySize, nil); err != nil {
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

func newSlowConnDialer(dialer func(network, addr string) (network.Conn, error)) network.Dialer {
	return &mockDialer{customDialConn: dialer}
}

type mockDialer struct {
	customDialConn func(network, addr string) (network.Conn, error)
}

func (m *mockDialer) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	return m.customDialConn(network, address)
}

func (m *mockDialer) DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error) {
	return nil, nil
}

func (m *mockDialer) AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	return nil, nil
}
