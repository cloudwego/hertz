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

package client

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/config/retry"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/dialer"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1"
	"github.com/cloudwego/hertz/pkg/protocol/http1/req"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
	"github.com/cloudwego/hertz/pkg/route"
)

var (
	errTooManyRedirects = errors.New("too many redirects detected when doing the request")
	errNoFreeConns      = errors.New("no free connections available to host")
)

func TestCloseIdleConnections(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10000"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))

	if _, _, err := c.Get(context.Background(), nil, "http://google.com"); err != nil {
		t.Fatal(err)
	}

	connsLen := func() int {
		c.mLock.Lock()
		defer c.mLock.Unlock()

		if _, ok := c.m["google.com"]; !ok {
			return 0
		}
		return c.m["google.com"].ConnectionCount()
	}

	if conns := connsLen(); conns > 1 {
		t.Errorf("expected 1 conns got %d", conns)
	}

	c.CloseIdleConnections()

	if conns := connsLen(); conns > 0 {
		t.Errorf("expected 0 conns got %d", conns)
	}
}

func TestClientInvalidURI(t *testing.T) {
	t.Parallel()

	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10001"
	opt.Network = "unix"
	requests := int64(0)
	engine := route.NewEngine(opt)
	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
		atomic.AddInt64(&requests, 1)
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))
	req, res := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(res)
	}()
	req.Header.SetMethod(consts.MethodGet)
	req.SetRequestURI("http://example.com\r\n\r\nGET /\r\n\r\n")
	err := c.Do(context.Background(), req, res)
	if err == nil {
		t.Fatal("expected error (missing required Host header in request)")
	}
	if n := atomic.LoadInt64(&requests); n != 0 {
		t.Fatalf("0 requests expected, got %d", n)
	}
}

func TestClientGetWithBody(t *testing.T) {
	t.Parallel()

	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10002"
	opt.Network = "unix"
	engine := route.NewEngine(opt)
	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
		body := ctx.Request.Body()
		ctx.Write(body) //nolint:errcheck
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))
	req, res := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(res)
	}()
	req.Header.SetMethod(consts.MethodGet)
	req.SetRequestURI("http://example.com")
	req.SetBodyString("test")
	err := c.Do(context.Background(), req, res)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Body()) == 0 {
		t.Fatal("missing request body")
	}
}

func TestClientURLAuth(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"foo:bar@": "Basic Zm9vOmJhcg==",
		"foo:@":    "Basic Zm9vOg==",
		":@":       "",
		"@":        "",
		"":         "",
	}
	ch := make(chan string, 1)

	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10003"
	opt.Network = "unix"
	engine := route.NewEngine(opt)
	engine.GET("/foo/bar", func(c context.Context, ctx *app.RequestContext) {
		ch <- string(ctx.Request.Header.Peek(consts.HeaderAuthorization))
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))
	for up, expected := range cases {
		req := protocol.AcquireRequest()
		req.Header.SetMethod(consts.MethodGet)
		req.SetRequestURI("http://" + up + "example.com/foo/bar")

		if err := c.Do(context.Background(), req, nil); err != nil {
			t.Fatal(err)
		}

		val := <-ch

		if val != expected {
			t.Fatalf("wrong %s header: %s expected %s", consts.HeaderAuthorization, val, expected)
		}
	}
}

func TestClientNilResp(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10004"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))

	req := protocol.AcquireRequest()
	req.Header.SetMethod(consts.MethodGet)
	req.SetRequestURI("http://example.com")
	if err := c.Do(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.DoTimeout(context.Background(), req, nil, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestClientParseConn(t *testing.T) {
	t.Parallel()
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10005"
	engine := route.NewEngine(opt)
	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	time.Sleep(time.Millisecond * 500)

	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))
	req, res := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(res)
	}()
	req.SetRequestURI("http://" + opt.Addr + "")
	if err := c.Do(context.Background(), req, res); err != nil {
		t.Fatal(err)
	}

	if res.RemoteAddr().Network() != opt.Network {
		t.Fatalf("req RemoteAddr parse network fail: %s, hope: %s", res.RemoteAddr().Network(), opt.Network)
	}
	if opt.Addr != res.RemoteAddr().String() {
		t.Fatalf("req RemoteAddr parse addr fail: %s, hope: %s", res.RemoteAddr().String(), opt.Addr)
	}

	if !regexp.MustCompile(`^127\.0\.0\.1:[0-9]{4,5}$`).MatchString(res.LocalAddr().String()) {
		t.Fatalf("res LocalAddr addr match fail: %s, hope match: %s", res.LocalAddr().String(), "^127.0.0.1:[0-9]{4,5}$")
	}
}

func TestClientPostArgs(t *testing.T) {
	t.Parallel()
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10006"
	opt.Network = "unix"
	engine := route.NewEngine(opt)
	engine.POST("/", func(c context.Context, ctx *app.RequestContext) {
		body := ctx.Request.Body()
		if len(body) == 0 {
			return
		}
		ctx.Write(body) //nolint:errcheck
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)
	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))
	req, res := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(res)
	}()
	args := req.PostArgs()
	args.Add("addhttp2", "support")
	args.Add("fast", "http")
	req.Header.SetMethod(consts.MethodPost)
	req.SetRequestURI("http://make.hertz.great?again")
	err := c.Do(context.Background(), req, res)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Body()) == 0 {
		t.Fatal("cannot set args as body")
	}
}

func TestClientHeaderCase(t *testing.T) {
	t.Parallel()

	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10007"
	opt.Network = "unix"
	engine := route.NewEngine(opt)
	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
		zw := ctx.GetWriter()
		zw.WriteBinary([]byte("HTTP/1.1 200 OK\r\n" + //nolint:errcheck
			"content-type: text/plain\r\n" +
			"transfer-encoding: chunked\r\n\r\n" +
			"24\r\nThis is the data in the first chunk \r\n" +
			"1B\r\nand this is the second one \r\n" +
			"0\r\n\r\n",
		))
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Second)

	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}), WithDisableHeaderNamesNormalizing(true))
	code, body, err := c.Get(context.Background(), nil, "http://example.com")
	if err != nil {
		t.Error(err)
	} else if code != 200 {
		t.Errorf("expected status code 200 got %d", code)
	} else if string(body) != "This is the data in the first chunk and this is the second one " {
		t.Errorf("wrong body: %q", body)
	}
}

func TestClientReadTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	timeout := false
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "localhost:10008"
	engine := route.NewEngine(opt)

	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
		if timeout {
			time.Sleep(time.Minute)
		} else {
			timeout = true
		}
	})
	go engine.Run()
	time.Sleep(time.Millisecond * 500)

	c := &http1.HostClient{
		ClientOptions: &http1.ClientOptions{
			ReadTimeout: time.Second * 4,
			RetryConfig: &retry.RetryConfig{MaxIdempotentCallAttempts: 1},
			Dial: func(addr string) (network.Conn, error) {
				return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
			},
		},
	}
	dialer.SetDialer(standard.NewDialer())

	req := protocol.AcquireRequest()
	res := protocol.AcquireResponse()

	req.SetRequestURI("http://" + opt.Addr)
	req.Header.SetMethod(consts.MethodGet)

	// Setting Connection: Close will make the connection be returned to the pool.
	req.SetConnectionClose()

	if err := c.Do(context.Background(), req, res); err != nil {
		t.Fatal(err)
	}

	protocol.ReleaseRequest(req)
	protocol.ReleaseResponse(res)

	done := make(chan struct{})
	go func() {
		req := protocol.AcquireRequest()
		res := protocol.AcquireResponse()

		req.SetRequestURI("http://" + opt.Addr)
		req.Header.SetMethod(consts.MethodGet)
		req.SetConnectionClose()

		if err := c.Do(context.Background(), req, res); !errors.Is(err, errs.ErrTimeout) {
			t.Errorf("expected ErrTimeout got %#v", err)
		}

		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(res)
		close(done)
	}()

	select {
	case <-done:
		// It is abnormal when waiting time exceeds the value of readTimeout times the number of retries.
		// Give it extra 2 seconds just to be sure.
	case <-time.After(c.ReadTimeout*time.Duration(c.RetryConfig.MaxIdempotentCallAttempts) + time.Second*2):
		t.Fatal("Client.ReadTimeout didn't work")
	}
}

func TestClientDefaultUserAgent(t *testing.T) {
	t.Parallel()

	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10009"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
		ctx.Data(200, "text/plain; charset=utf-8", ctx.UserAgent())
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))
	req := protocol.AcquireRequest()
	res := protocol.AcquireResponse()

	req.SetRequestURI("http://example.com")
	req.Header.SetMethod(consts.MethodGet)

	err := c.Do(context.Background(), req, res)
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Body()) != string(bytestr.DefaultUserAgent) {
		t.Fatalf("User-Agent defers %q != %q", string(res.Body()), bytestr.DefaultUserAgent)
	}
}

func TestClientSetUserAgent(t *testing.T) {
	t.Parallel()

	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10010"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
		ctx.Data(200, "text/plain; charset=utf-8", ctx.UserAgent())
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	userAgent := "I'm not hertz"
	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}), WithName(userAgent))
	req := protocol.AcquireRequest()
	res := protocol.AcquireResponse()

	req.SetRequestURI("http://example.com")

	err := c.Do(context.Background(), req, res)
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Body()) != userAgent {
		t.Fatalf("User-Agent defers %q != %q", string(res.Body()), userAgent)
	}
}

func TestClientNoUserAgent(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10011"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
		ctx.Data(200, "text/plain; charset=utf-8", ctx.UserAgent())
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)
	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}), WithNoDefaultUserAgentHeader(true))

	req := protocol.AcquireRequest()
	res := protocol.AcquireResponse()

	req.SetRequestURI("http://example.com")

	err := c.Do(context.Background(), req, res)
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Body()) != "" {
		t.Fatalf("User-Agent wrong %q != %q", string(res.Body()), "")
	}
}

func TestClientDoWithCustomHeaders(t *testing.T) {
	t.Parallel()

	ch := make(chan error)
	uri := "/foo/bar/baz?a=b&cd=12"
	headers := map[string]string{
		"Foo":          "bar",
		"Host":         "xxx.com",
		"Content-Type": "asdfsdf",
		"a-b-c-d-f":    "",
	}
	body := "request body"
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10012"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.POST("/foo/bar/baz", func(c context.Context, ctx *app.RequestContext) {
		zw := ctx.GetWriter()

		if string(ctx.Request.Header.Method()) != consts.MethodPost {
			ch <- fmt.Errorf("unexpected request method: %q. Expecting %q", ctx.Request.Header.Method(), consts.MethodPost)
			return
		}
		reqURI := ctx.Request.RequestURI()
		if string(reqURI) != uri {
			ch <- fmt.Errorf("unexpected request uri: %q. Expecting %q", reqURI, uri)
			return
		}
		for k, v := range headers {
			hv := ctx.Request.Header.Peek(k)
			if string(hv) != v {
				ch <- fmt.Errorf("unexpected value for header %q: %q. Expecting %q", k, hv, v)
				return
			}
		}
		cl := ctx.Request.Header.ContentLength()
		if cl != len(body) {
			ch <- fmt.Errorf("unexpected content-length %d. Expecting %d", cl, len(body))
			return
		}
		reqBody := ctx.Request.Body()
		if string(reqBody) != body {
			ch <- fmt.Errorf("unexpected request body: %q. Expecting %q", reqBody, body)
			return
		}

		var r protocol.Response
		if err := resp.Write(&r, zw); err != nil {
			ch <- fmt.Errorf("cannot send response: %s", err)
			return
		}
		if err := zw.Flush(); err != nil {
			ch <- fmt.Errorf("cannot flush response: %s", err)
			return
		}

		ch <- nil
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	// make sure that the client sends all the request headers and body.
	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))

	var req protocol.Request
	req.Header.SetMethod(consts.MethodPost)
	req.SetRequestURI(uri)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.SetBodyString(body)

	var resp protocol.Response

	err := c.DoTimeout(context.Background(), &req, &resp, time.Second)
	if err != nil {
		t.Fatalf("error when doing request: %s", err)
	}

	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestClientDoTimeoutDisablePathNormalizing(t *testing.T) {
	t.Parallel()
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10013"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.Use(func(c context.Context, ctx *app.RequestContext) {
		uri := ctx.URI()
		uri.DisablePathNormalizing = true
		ctx.Response.Header.Set("received-uri", string(uri.FullURI()))
	})

	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)
	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}), WithDisablePathNormalizing(true))

	urlWithEncodedPath := "http://example.com/encoded/Y%2BY%2FY%3D/stuff"

	var req protocol.Request
	req.SetRequestURI(urlWithEncodedPath)
	var resp protocol.Response
	for i := 0; i < 5; i++ {
		if err := c.DoTimeout(context.Background(), &req, &resp, time.Second); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		hv := resp.Header.Peek("received-uri")
		if string(hv) != urlWithEncodedPath {
			t.Fatalf("request uri was normalized: %q. Expecting %q", hv, urlWithEncodedPath)
		}
	}
}

func TestHostClientPendingRequests(t *testing.T) {
	t.Parallel()

	const concurrency = 10
	doneCh := make(chan struct{})
	readyCh := make(chan struct{}, concurrency)
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10014"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.GET("/baz", func(c context.Context, ctx *app.RequestContext) {
		readyCh <- struct{}{}
		<-doneCh
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Second)

	c := &http1.HostClient{
		ClientOptions: &http1.ClientOptions{
			Addr: "foobar",
			Dial: func(addr string) (network.Conn, error) {
				return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
			},
		},
	}

	pendingRequests := c.PendingRequests()
	if pendingRequests != 0 {
		t.Fatalf("non-zero pendingRequests: %d", pendingRequests)
	}

	resultCh := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			req := protocol.AcquireRequest()
			req.SetRequestURI("http://foobar/baz")
			req.Header.SetMethod(consts.MethodGet)
			resp := protocol.AcquireResponse()

			if err := c.DoTimeout(context.Background(), req, resp, 10*time.Second); err != nil {
				resultCh <- fmt.Errorf("unexpected error: %s", err)
				return
			}

			if resp.StatusCode() != consts.StatusOK {
				resultCh <- fmt.Errorf("unexpected status code %d. Expecting %d", resp.StatusCode(), consts.StatusOK)
				return
			}
			resultCh <- nil
		}()
	}

	// wait until all the requests reach server
	for i := 0; i < concurrency; i++ {
		select {
		case <-readyCh:
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}

	pendingRequests = c.PendingRequests()
	if pendingRequests != concurrency {
		t.Fatalf("unexpected pendingRequests: %d. Expecting %d", pendingRequests, concurrency)
	}

	// unblock request handlers on the server and wait until all the requests are finished.
	close(doneCh)
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-resultCh:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}

	pendingRequests = c.PendingRequests()
	if pendingRequests != 0 {
		t.Fatalf("non-zero pendingRequests: %d", pendingRequests)
	}
}

func TestHostClientMaxConnsWithDeadline(t *testing.T) {
	var (
		emptyBodyCount uint8
		timeout        = 50 * time.Millisecond
		wg             sync.WaitGroup
	)
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10015"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.POST("/baz", func(c context.Context, ctx *app.RequestContext) {
		if len(ctx.Request.Body()) == 0 {
			emptyBodyCount++
		}

		ctx.WriteString("foo") //nolint:errcheck
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c := &http1.HostClient{
		ClientOptions: &http1.ClientOptions{
			Addr: "foobar",
			Dial: func(addr string) (network.Conn, error) {
				return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
			},
			MaxConns: 1,
		},
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := protocol.AcquireRequest()
			req.SetRequestURI("http://foobar/baz")
			req.Header.SetMethod(consts.MethodPost)
			req.SetBodyString("bar")
			resp := protocol.AcquireResponse()

			for {
				if err := c.DoDeadline(context.Background(), req, resp, time.Now().Add(timeout)); err != nil {
					if err.Error() == errNoFreeConns.Error() {
						time.Sleep(time.Millisecond * 500)
						continue
					}
					t.Errorf("unexpected error: %s", err)
				}
				break
			}

			if resp.StatusCode() != consts.StatusOK {
				t.Errorf("unexpected status code %d. Expecting %d", resp.StatusCode(), consts.StatusOK)
			}

			body := resp.Body()
			if string(body) != "foo" {
				t.Errorf("unexpected body %q. Expecting %q", body, "abcd")
			}
		}()
	}
	wg.Wait()

	if emptyBodyCount > 0 {
		t.Fatalf("at least one request body was empty")
	}
}

func TestHostClientMaxConnDuration(t *testing.T) {
	t.Parallel()

	connectionCloseCount := uint32(0)
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10016"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.GET("/bbb/cc", func(c context.Context, ctx *app.RequestContext) {
		ctx.WriteString("abcd") //nolint:errcheck
		if ctx.Request.ConnectionClose() {
			atomic.AddUint32(&connectionCloseCount, 1)
		}
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c := &http1.HostClient{
		ClientOptions: &http1.ClientOptions{
			Addr: "foobar",
			Dial: func(addr string) (network.Conn, error) {
				return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
			},
			MaxConnDuration: 10 * time.Millisecond,
		},
	}

	for i := 0; i < 5; i++ {
		statusCode, body, err := c.Get(context.Background(), nil, "http://aaaa.com/bbb/cc")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if statusCode != consts.StatusOK {
			t.Fatalf("unexpected status code %d. Expecting %d", statusCode, consts.StatusOK)
		}
		if string(body) != "abcd" {
			t.Fatalf("unexpected body %q. Expecting %q", body, "abcd")
		}
		time.Sleep(c.MaxConnDuration)
	}

	if atomic.LoadUint32(&connectionCloseCount) == 0 {
		t.Fatalf("expecting at least one 'Connection: close' request header")
	}
}

func TestHostClientMultipleAddrs(t *testing.T) {
	t.Parallel()
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10017"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.GET("/baz/aaa", func(c context.Context, ctx *app.RequestContext) {
		ctx.Write(ctx.Host()) //nolint:errcheck
		ctx.SetConnectionClose()
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	dialsCount := make(map[string]int)
	c := &http1.HostClient{
		ClientOptions: &http1.ClientOptions{
			Addr: "foo,bar,baz",
			Dial: func(addr string) (network.Conn, error) {
				dialsCount[addr]++
				return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
			},
		},
	}

	for i := 0; i < 9; i++ {
		statusCode, body, err := c.Get(context.Background(), nil, "http://foobar/baz/aaa?bbb=ddd")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if statusCode != consts.StatusOK {
			t.Fatalf("unexpected status code %d. Expecting %d", statusCode, consts.StatusOK)
		}
		if string(body) != "foobar" {
			t.Fatalf("unexpected body %q. Expecting %q", body, "foobar")
		}
	}

	if len(dialsCount) != 3 {
		t.Fatalf("unexpected dialsCount size %d. Expecting 3", len(dialsCount))
	}
	for _, k := range []string{"foo", "bar", "baz"} {
		if dialsCount[k] != 3 {
			t.Fatalf("unexpected dialsCount for %q. Expecting 3", k)
		}
	}
}

func TestClientFollowRedirects(t *testing.T) {
	t.Parallel()
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10018"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	handler := func(c context.Context, ctx *app.RequestContext) {
		switch string(ctx.Path()) {
		case "/foo":
			u := ctx.URI()
			u.Update("/xy?z=wer")
			ctx.Redirect(consts.StatusFound, u.FullURI())
		case "/xy":
			u := ctx.URI()
			u.Update("/bar")
			ctx.Redirect(consts.StatusFound, u.FullURI())
		default:
			ctx.SetContentType("text/plain")
			ctx.Response.SetBody(ctx.Path())
		}
	}
	engine.GET("/foo", handler)
	engine.GET("/xy", handler)
	engine.GET("/bar", handler)
	engine.GET("/aaab/sss", handler)

	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c := &http1.HostClient{
		ClientOptions: &http1.ClientOptions{
			Addr: "xxx",
			Dial: func(addr string) (network.Conn, error) {
				return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
			},
		},
	}

	for i := 0; i < 10; i++ {
		statusCode, body, err := c.GetTimeout(context.Background(), nil, "http://xxx/foo", time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if statusCode != consts.StatusOK {
			t.Fatalf("unexpected status code: %d", statusCode)
		}
		if string(body) != "/bar" {
			t.Fatalf("unexpected response %q. Expecting %q", body, "/bar")
		}
	}

	for i := 0; i < 10; i++ {
		statusCode, body, err := c.Get(context.Background(), nil, "http://xxx/aaab/sss")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if statusCode != consts.StatusOK {
			t.Fatalf("unexpected status code: %d", statusCode)
		}
		if string(body) != "/aaab/sss" {
			t.Fatalf("unexpected response %q. Expecting %q", body, "/aaab/sss")
		}
	}

	for i := 0; i < 10; i++ {
		req := protocol.AcquireRequest()
		resp := protocol.AcquireResponse()

		req.SetRequestURI("http://xxx/foo")

		err := c.DoRedirects(context.Background(), req, resp, 16)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if statusCode := resp.StatusCode(); statusCode != consts.StatusOK {
			t.Fatalf("unexpected status code: %d", statusCode)
		}

		if body := string(resp.Body()); body != "/bar" {
			t.Fatalf("unexpected response %q. Expecting %q", body, "/bar")
		}

		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(resp)
	}

	req := protocol.AcquireRequest()
	resp := protocol.AcquireResponse()

	req.SetRequestURI("http://xxx/foo")

	err := c.DoRedirects(context.Background(), req, resp, 0)
	if have, want := err, errTooManyRedirects; have.Error() != want.Error() {
		t.Fatalf("want error: %v, have %v", want, have)
	}

	protocol.ReleaseRequest(req)
	protocol.ReleaseResponse(resp)
}

func TestHostClientMaxConnWaitTimeoutSuccess(t *testing.T) {
	var (
		emptyBodyCount uint8
		wg             sync.WaitGroup
	)
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10019"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.POST("/baz", func(c context.Context, ctx *app.RequestContext) {
		if len(ctx.Request.Body()) == 0 {
			emptyBodyCount++
		}
		time.Sleep(5 * time.Millisecond)
		ctx.WriteString("foo") //nolint:errcheck
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c := &http1.HostClient{
		ClientOptions: &http1.ClientOptions{
			Addr: "foobar",
			Dial: func(addr string) (network.Conn, error) {
				return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
			},
			MaxConns:           1,
			MaxConnWaitTimeout: 200 * time.Millisecond,
		},
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := protocol.AcquireRequest()
			req.SetRequestURI("http://foobar/baz")
			req.Header.SetMethod(consts.MethodPost)
			req.SetBodyString("bar")
			resp := protocol.AcquireResponse()

			if err := c.Do(context.Background(), req, resp); err != nil {
				t.Errorf("unexpected error: %s", err)
			}

			if resp.StatusCode() != consts.StatusOK {
				t.Errorf("unexpected status code %d. Expecting %d", resp.StatusCode(), consts.StatusOK)
			}

			body := resp.Body()
			if string(body) != "foo" {
				t.Errorf("unexpected body %q. Expecting %q", body, "abcd")
			}
		}()
	}
	wg.Wait()

	if c.WantConnectionCount() > 0 {
		t.Errorf("connsWait has %v items remaining", c.WantConnectionCount())
	}

	if emptyBodyCount > 0 {
		t.Fatalf("at least one request body was empty")
	}
}

func TestHostClientMaxConnWaitTimeoutError(t *testing.T) {
	var (
		emptyBodyCount uint8
		wg             sync.WaitGroup
	)
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10020"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.POST("/baz", func(c context.Context, ctx *app.RequestContext) {
		if len(ctx.Request.Body()) == 0 {
			emptyBodyCount++
		}
		time.Sleep(5 * time.Millisecond)
		ctx.WriteString("foo") //nolint:errcheck
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c := &http1.HostClient{
		ClientOptions: &http1.ClientOptions{
			Addr: "foobar",
			Dial: func(addr string) (network.Conn, error) {
				return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
			},
			MaxConns:           1,
			MaxConnWaitTimeout: 10 * time.Millisecond,
		},
	}

	var errNoFreeConnsCount uint32
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := protocol.AcquireRequest()
			req.SetRequestURI("http://foobar/baz")
			req.Header.SetMethod(consts.MethodPost)
			req.SetBodyString("bar")
			resp := protocol.AcquireResponse()

			if err := c.Do(context.Background(), req, resp); err != nil {
				if err.Error() != errNoFreeConns.Error() {
					t.Errorf("unexpected error: %s. Expecting %s", err.Error(), errNoFreeConns.Error())
				}
				atomic.AddUint32(&errNoFreeConnsCount, 1)
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

	if c.WantConnectionCount() > 0 {
		t.Errorf("connsWait has %v items remaining", c.WantConnectionCount())
	}
	if errNoFreeConnsCount == 0 {
		t.Errorf("unexpected errorCount: %d. Expecting > 0", errNoFreeConnsCount)
	}

	if emptyBodyCount > 0 {
		t.Fatalf("at least one request body was empty")
	}
}

func TestNewClient(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10022"
	engine := route.NewEngine(opt)
	engine.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.SetBodyString("pong")
	})
	go engine.Run()
	time.Sleep(time.Millisecond * 500)

	client, err := NewClient(WithDialTimeout(2 * time.Second))
	if err != nil {
		t.Fatal(err)
		return
	}
	status, resp, err := client.Get(context.Background(), nil, "http://127.0.0.1:10022/ping")
	if err != nil {
		t.Fatal(err)
		return
	}
	if status != consts.StatusOK {
		t.Errorf("return http status=%v", status)
	}
	t.Logf("resp=%v\n", string(resp))
}

func TestUseShortConnection(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10023"
	engine := route.NewEngine(opt)
	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	time.Sleep(time.Millisecond * 500)

	c, _ := NewClient(WithKeepAlive(false))
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := c.Get(context.Background(), nil, "http://127.0.0.1:10023"); err != nil {
				t.Error(err)
				return
			}
		}()
	}
	wg.Wait()
	connsLen := func() int {
		c.mLock.Lock()
		defer c.mLock.Unlock()

		if _, ok := c.m["127.0.0.1:10023"]; !ok {
			return 0
		}

		return c.m["127.0.0.1:10023"].ConnectionCount()
	}

	if conns := connsLen(); conns > 0 {
		t.Errorf("expected 0 conns got %d", conns)
	}
}

func TestPostWithFormData(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10025"
	engine := route.NewEngine(opt)
	engine.POST("/", func(c context.Context, ctx *app.RequestContext) {
		var ans string
		ctx.PostArgs().VisitAll(func(key, value []byte) {
			ans = ans + string(key) + "=" + string(value) + "&"
		})
		ans = strings.TrimRight(ans, "&")
		ctx.Data(200, "text/plain; charset=utf-8", []byte(ans))
	})
	go engine.Run()

	time.Sleep(100 * time.Millisecond)
	client, _ := NewClient()
	req := protocol.AcquireRequest()
	rsp := protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(rsp)
	}()
	postParam := map[string][]string{
		"a": {"c", "d", "e"},
		"b": {"c"},
		"c": {"f"},
	}
	req.SetFormData(map[string]string{
		"a": "c",
		"b": "c",
	})
	req.SetFormDataFromValues(url.Values{
		"a": []string{"d", "e"},
		"c": []string{"f"},
	})
	req.SetRequestURI("http://127.0.0.1:10025")
	req.SetMethod(consts.MethodPost)
	err := client.Do(context.Background(), req, rsp)
	if err != nil {
		t.Error(err)
	}
	for k, v := range postParam {
		for _, kv := range v {
			if !strings.Contains(string(rsp.Body()), k+"="+kv) {
				t.Errorf("miss %v=%v", k, kv)
			}
		}
	}
}

func TestPostWithMultipartField(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10026"
	engine := route.NewEngine(opt)
	engine.POST("/", func(c context.Context, ctx *app.RequestContext) {
		if string(ctx.FormValue("a")) != "1" {
			t.Errorf("field a want 1, got %v", string(ctx.FormValue("a")))
		}
		if string(ctx.FormValue("b")) != "2" {
			t.Errorf("field b want 2, got %v", string(ctx.FormValue("b")))
		}
		t.Log(req.GetHTTP1Request(&ctx.Request).String())
	})
	go engine.Run()

	time.Sleep(100 * time.Millisecond)
	client, _ := NewClient()
	req := protocol.AcquireRequest()
	rsp := protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(rsp)
	}()
	data := map[string]string{
		"a": "1",
		"b": "2",
	}
	req.SetMethod(consts.MethodPost)
	req.SetRequestURI("http://127.0.0.1:10026")
	req.SetMultipartFormData(data)
	req.SetMultipartFormData(map[string]string{
		"c": "3",
	})
	err := client.DoTimeout(context.Background(), req, rsp, 1*time.Second)
	if err != nil {
		t.Error(err)
	}
}

func TestSetFiles(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10027"
	engine := route.NewEngine(opt)
	engine.POST("/", func(c context.Context, ctx *app.RequestContext) {
		form, _ := ctx.MultipartForm()
		files := form.File["files"]
		// Upload the file to specific dst.
		for _, file := range files {
			ctx.SaveUploadedFile(file, filepath.Base(file.Filename))
		}
		file1, _ := ctx.FormFile("file_1")
		ctx.SaveUploadedFile(file1, filepath.Base(file1.Filename))
		file2, _ := ctx.FormFile("file_2")
		ctx.SaveUploadedFile(file2, filepath.Base(file2.Filename))
		ctx.String(consts.StatusOK, fmt.Sprintf("%d files uploaded!", len(files)+2))
	})
	go engine.Run()

	time.Sleep(100 * time.Millisecond)
	client, _ := NewClient()
	req := protocol.AcquireRequest()
	rsp := protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(rsp)
	}()
	req.SetMethod(consts.MethodPost)
	req.SetRequestURI("http://127.0.0.1:10027")
	files := []string{"../../common/testdata/test.txt", "../../common/testdata/proto/test.proto", "../../common/testdata/test.png", "../../common/testdata/proto/test.pb.go"}
	defer func() {
		for _, file := range files {
			os.Remove(filepath.Base(file))
		}
	}()
	req.SetFile("files", files[0])
	req.SetFile("files", files[1])
	req.SetFiles(map[string]string{
		"file_1": files[2],
		"file_2": files[3],
	})
	err := client.DoTimeout(context.Background(), req, rsp, 1*time.Second)
	if err != nil {
		t.Error(err)
	}
}

func TestSetMultipartFields(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10028"
	engine := route.NewEngine(opt)
	engine.POST("/", func(c context.Context, ctx *app.RequestContext) {
		t.Log(req.GetHTTP1Request(&ctx.Request).String())
		if string(ctx.FormValue("a")) != "1" {
			t.Errorf("field a want 1, got %v", string(ctx.FormValue("a")))
		}
		if string(ctx.FormValue("b")) != "2" {
			t.Errorf("field b want 2, got %v", string(ctx.FormValue("b")))
		}
		file1, _ := ctx.FormFile("file_1")
		ctx.SaveUploadedFile(file1, filepath.Base(file1.Filename))
		file2, _ := ctx.FormFile("file_2")
		ctx.SaveUploadedFile(file2, filepath.Base(file2.Filename))
		ctx.String(consts.StatusOK, fmt.Sprintf("%d files uploaded!", 2))
	})
	go engine.Run()

	time.Sleep(100 * time.Millisecond)
	client, _ := NewClient(WithDialTimeout(50 * time.Millisecond))
	req := protocol.AcquireRequest()
	rsp := protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(rsp)
	}()
	jsonStr1 := `{"input": {"name": "Uploaded document 1", "_filename" : ["file1.txt"]}}`
	jsonStr2 := `{"input": {"name": "Uploaded document 2", "_filename" : ["file2.txt"]}}`
	files := []string{"upload-file-1.json", "upload-file-2.json"}
	fields := []*protocol.MultipartField{
		{
			Param:       "file_1",
			FileName:    files[0],
			ContentType: "application/json",
			Reader:      strings.NewReader(jsonStr1),
		},
		{
			Param:       "file_2",
			FileName:    files[1],
			ContentType: "application/json",
			Reader:      strings.NewReader(jsonStr2),
		},
	}
	defer func() {
		for _, file := range files {
			os.Remove(filepath.Base(file))
		}
	}()
	req.SetMultipartFields(fields...)
	req.SetMultipartFormData(map[string]string{"a": "1", "b": "2"})
	req.SetRequestURI("http://127.0.0.1:10028")
	req.SetMethod(consts.MethodPost)
	err := client.DoTimeout(context.Background(), req, rsp, 1*time.Second)
	if err != nil {
		t.Error(err)
	}
}

func TestClientReadResponseBodyStream(t *testing.T) {
	part1 := "abcdef"
	part2 := "ghij"

	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10033"
	engine := route.NewEngine(opt)
	engine.POST("/", func(ctx context.Context, c *app.RequestContext) {
		c.String(consts.StatusOK, part1+part2)
	})
	go engine.Run()
	time.Sleep(100 * time.Millisecond)

	client, _ := NewClient(WithResponseBodyStream(true))
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(resp)
	}()
	req.SetRequestURI("http://127.0.0.1:10033")
	req.SetMethod(consts.MethodPost)
	err := client.Do(context.Background(), req, resp)
	if err != nil {
		t.Errorf("client Do error=%v", err.Error())
	}
	bodyStream := resp.BodyStream()
	if bodyStream == nil {
		t.Errorf("bodystream is nil")
	}
	// Read part1 body bytes
	p := make([]byte, len(part1))
	r, err := bodyStream.Read(p)
	if err != nil {
		t.Errorf("read from bodystream error=%v", err.Error())
	}
	if string(p) != part1 {
		t.Errorf("read len=%v, read content=%v; want len=%v, want content=%v", r, string(p), len(part1), part1)
	}
	left, _ := ioutil.ReadAll(bodyStream)
	if string(left) != part2 {
		t.Errorf("left len=%v, left content=%v; want len=%v, want content=%v", len(left), string(left), len(part2), part2)
	}
}

func TestWithBasicAuth(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10034"
	engine := route.NewEngine(opt)
	engine.GET("/", func(c context.Context, ctx *app.RequestContext) {
		auth := ctx.GetHeader(consts.HeaderAuthorization)
		if len(auth) < 6 {
			ctx.SetStatusCode(consts.StatusUnauthorized)
			return
		}
		password, err := base64.StdEncoding.DecodeString(string(auth[6:]))
		if err != nil || string(password) != "myuser:basicauth" {
			ctx.SetStatusCode(consts.StatusUnauthorized)
			return
		}
	})
	go engine.Run()
	time.Sleep(100 * time.Millisecond)
	client, _ := NewClient()
	req := protocol.AcquireRequest()
	rsp := protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(rsp)
	}()

	// Success
	req.SetBasicAuth("myuser", "basicauth")
	req.SetRequestURI("http://127.0.0.1:10034")
	req.SetMethod(consts.MethodGet)
	err := client.Do(context.Background(), req, rsp)
	if err != nil {
		t.Error(err)
	}
	if rsp.StatusCode() == consts.StatusUnauthorized {
		t.Error("unexpected status code=401")
	}

	// Fail
	req.Reset()
	rsp.Reset()
	req.SetRequestURI("http://127.0.0.1:10034")
	req.SetMethod(consts.MethodGet)
	err = client.Do(context.Background(), req, rsp)
	if err != nil {
		t.Error(err)
	}
	if rsp.StatusCode() != consts.StatusUnauthorized {
		t.Errorf("unexpected status code: %v, expected 401", rsp.StatusCode())
	}
}

func TestClientProxyWithStandardDialer(t *testing.T) {
	testCases := []struct{ httpsSite, httpsProxy bool }{
		{false, false},
		{false, true},
		{true, false},
		{true, true},
	}
	for _, testCase := range testCases {
		httpsSite := testCase.httpsSite
		httpsProxy := testCase.httpsProxy
		t.Run(fmt.Sprintf("httpsSite=%v, httpsProxy=%v", httpsSite, httpsProxy), func(t *testing.T) {
			siteCh := make(chan *http.Request, 1)
			h1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				siteCh <- r
			})
			proxyCh := make(chan *http.Request, 1)
			h2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				proxyCh <- r
				if r.Method == "CONNECT" {
					hijacker, ok := w.(http.Hijacker)
					if !ok {
						t.Errorf("hijack not allowed")
						return
					}
					clientConn, _, err := hijacker.Hijack()
					if err != nil {
						t.Errorf("hijacking failed")
						return
					}
					res := &http.Response{
						StatusCode: http.StatusOK,
						Proto:      "HTTP/1.1",
						ProtoMajor: 1,
						ProtoMinor: 1,
						Header:     make(http.Header),
					}
					targetConn, err := net.Dial("tcp", r.URL.Host)
					if err != nil {
						t.Errorf("net.Dial(%q) failed: %v", r.URL.Host, err)
						return
					}

					if err := res.Write(clientConn); err != nil {
						t.Errorf("Writing 200 OK failed: %v", err)
						return
					}
					go io.Copy(targetConn, clientConn)
					go func() {
						io.Copy(clientConn, targetConn)
						targetConn.Close()
					}()
				}
			})
			var ts *httptest.Server
			if httpsSite {
				ts = httptest.NewTLSServer(h1)
			} else {
				ts = httptest.NewServer(h1)
			}
			var proxyServer *httptest.Server
			if httpsProxy {
				proxyServer = httptest.NewTLSServer(h2)
			} else {
				proxyServer = httptest.NewServer(h2)
			}
			pu := protocol.ParseURI(proxyServer.URL)

			// If neither server is HTTPS or both are, then c may be derived from either.
			// If only one server is HTTPS, c must be derived from that server in order
			// to ensure that it is configured to use the fake root CA from testcert.go.
			dialer.SetDialer(standard.NewDialer())
			var cOpt config.ClientOption
			if httpsProxy {
				cOpt = WithTLSConfig(proxyServer.Client().Transport.(*http.Transport).TLSClientConfig)
			} else if httpsSite {
				cOpt = WithTLSConfig(ts.Client().Transport.(*http.Transport).TLSClientConfig)
			}
			var c *Client
			if httpsProxy || httpsSite {
				c, _ = NewClient(cOpt)
			} else {
				c, _ = NewClient()
			}
			c.SetProxy(protocol.ProxyURI(pu))
			req, rsp := protocol.AcquireRequest(), protocol.AcquireResponse()
			defer func() {
				protocol.ReleaseRequest(req)
				protocol.ReleaseResponse(rsp)
			}()
			req.SetRequestURI(ts.URL)
			req.SetMethod(consts.MethodHead)
			err := c.Do(context.Background(), req, rsp)
			if err != nil {
				t.Error(err)
			}
			var got *http.Request
			select {
			case got = <-proxyCh:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout connecting to http proxy")
			}
			ts.Close()
			proxyServer.Close()

			if httpsSite {
				// First message should be a CONNECT to ask for a socket to the real server,
				if got.Method != "CONNECT" {
					t.Errorf("Wrong method for secure proxying: %q", got.Method)
				}
				gotHost := got.URL.Host
				pu, err := url.Parse(ts.URL)
				if err != nil {
					t.Fatal("Invalid site URL")
				}
				if wantHost := pu.Host; gotHost != wantHost {
					t.Errorf("Got CONNECT host %q, want %q", gotHost, wantHost)
				}

				// The next message on the channel should be from the site's server.
				next := <-siteCh
				if next.Method != "HEAD" {
					t.Errorf("Wrong method at destination: %s", next.Method)
				}
				if nextURL := next.URL.String(); nextURL != "/" {
					t.Errorf("Wrong URL at destination: %s", nextURL)
				}
			} else {
				if got.Method != "HEAD" {
					t.Errorf("Wrong method for destination: %q", got.Method)
				}
				gotURL := got.URL.String()
				wantURL := ts.URL + "/"
				if gotURL != wantURL {
					t.Errorf("Got URL %q, want %q", gotURL, wantURL)
				}
			}
		})
	}
}

func TestClientProxyWithNetpollDialer(t *testing.T) {
	testCases := []struct{ httpsSite, httpsProxy bool }{
		{false, false},
		{true, false},
		{false, true},
		{false, true},
	}
	for _, testCase := range testCases {
		httpsSite := testCase.httpsSite
		httpsProxy := testCase.httpsProxy
		t.Run(fmt.Sprintf("httpsSite=%v, httpsProxy=%v", httpsSite, httpsProxy), func(t *testing.T) {
			siteCh := make(chan *http.Request, 1)
			h1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				siteCh <- r
			})
			proxyCh := make(chan *http.Request, 1)
			h2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				proxyCh <- r
			})
			var ts *httptest.Server
			if httpsSite {
				ts = httptest.NewTLSServer(h1)
			} else {
				ts = httptest.NewServer(h1)
			}
			var proxyServer *httptest.Server
			if httpsProxy {
				proxyServer = httptest.NewTLSServer(h2)
			} else {
				proxyServer = httptest.NewServer(h2)
			}
			pu := protocol.ParseURI(proxyServer.URL)
			// If neither server is HTTPS or both are, then c may be derived from either.
			// If only one server is HTTPS, c must be derived from that server in order
			// to ensure that it is configured to use the fake root CA from testcert.go.

			c, _ := NewClient()
			c.SetProxy(protocol.ProxyURI(pu))
			req, rsp := protocol.AcquireRequest(), protocol.AcquireResponse()
			defer func() {
				protocol.ReleaseRequest(req)
				protocol.ReleaseResponse(rsp)
			}()
			req.SetRequestURI(ts.URL)
			req.SetMethod(consts.MethodHead)
			err := c.Do(context.Background(), req, rsp)
			if err != nil {
				t.Log(err)
				if !httpsSite && !httpsProxy {
					t.Fatal(err)
				}
				return
			}
			var got *http.Request
			select {
			case got = <-proxyCh:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout connecting to http proxy")
			}
			ts.Close()
			proxyServer.Close()

			if got.Method != "HEAD" {
				t.Errorf("Wrong method for destination: %q", got.Method)
			}
			gotURL := got.URL.String()
			wantURL := ts.URL + "/"
			if gotURL != wantURL {
				t.Errorf("Got URL %q, want %q", gotURL, wantURL)
			}
		})
	}
}

func TestClientMiddleware(t *testing.T) {
	client, _ := NewClient()
	mw0 := func(next Endpoint) Endpoint {
		return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
			req.SetRequestURI("middleware0")
			return next(ctx, req, resp)
		}
	}
	mw1 := func(next Endpoint) Endpoint {
		return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
			if string(req.RequestURI()) != "middleware0" {
				t.Errorf("Wrong request URI: %s, expected %v", req.RequestURI(), "middleware0")
			}
			req.SetRequestURI("middleware1")
			return next(ctx, req, resp)
		}
	}
	mw2 := func(next Endpoint) Endpoint {
		return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
			if string(req.RequestURI()) != "middleware1" {
				t.Errorf("Wrong request URI: %s, expected %v", req.RequestURI(), "middleware1")
			}
			return nil
		}
	}
	client.Use(mw0)
	client.Use(mw1)
	client.Use(mw2)

	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	err := client.Do(context.Background(), req, resp)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func TestClientReadResponseBodyStreamWithDoubleRequest(t *testing.T) {
	part1 := ""
	for i := 0; i < 8192; i++ {
		part1 += "a"
	}
	part2 := "ghij"

	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10035"
	engine := route.NewEngine(opt)
	engine.POST("/", func(ctx context.Context, c *app.RequestContext) {
		c.String(consts.StatusOK, part1+part2)
	})
	go engine.Run()
	time.Sleep(100 * time.Millisecond)

	client, _ := NewClient(WithResponseBodyStream(true))
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(resp)
	}()
	req.SetRequestURI("http://127.0.0.1:10035")
	req.SetMethod(consts.MethodPost)
	err := client.Do(context.Background(), req, resp)
	if err != nil {
		t.Errorf("client Do error=%v", err.Error())
	}
	bodyStream := resp.BodyStream()
	if bodyStream == nil {
		t.Errorf("bodystream is nil")
	}

	// Read part1 body bytes
	p := make([]byte, len(part1))
	r, err := bodyStream.Read(p)
	if err != nil {
		t.Errorf("read from bodystream error=%v", err.Error())
	}
	if string(p) != part1 {
		t.Errorf("read len=%v, read content=%v; want len=%v, want content=%v", r, string(p), len(part1), part1)
	}

	// send another request and read all bodystream
	req1, resp1 := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req1)
		protocol.ReleaseResponse(resp1)
	}()
	req1.SetRequestURI("http://127.0.0.1:10035")
	req1.SetMethod(consts.MethodPost)
	err = client.Do(context.Background(), req1, resp1)
	if err != nil {
		t.Errorf("client Do error=%v", err.Error())
	}
	bodyStream1 := resp1.BodyStream()
	if bodyStream1 == nil {
		t.Errorf("bodystream1 is nil")
	}
	data, _ := ioutil.ReadAll(bodyStream1)
	if string(data) != part1+part2 {
		t.Errorf("read len=%v, read content=%v; want len=%v, want content=%v", len(data), data, len(part1+part2), part1+part2)
	}

	// read left bodystream
	left, _ := ioutil.ReadAll(bodyStream)
	if string(left) != part2 {
		t.Errorf("left len=%v, left content=%v; want len=%v, want content=%v", len(left), string(left), len(part2), part2)
	}
}

func TestClientDomainPort(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "unix-test-10021"
	opt.Network = "unix"
	engine := route.NewEngine(opt)
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	expectedAddrMap := map[string]struct{}{
		"example1.com:443":  {},
		"example2.com:8888": {},
		"example3.com:80":   {},
	}
	time.Sleep(time.Millisecond * 500)
	c, _ := NewClient(WithDialFunc(func(addr string) (network.Conn, error) {
		if _, ok := expectedAddrMap[addr]; !ok {
			t.Fatalf("not expected addr:%s", addr)
		}
		return dialer.DialConnection(opt.Network, opt.Addr, time.Second, nil)
	}))

	_, _, err := c.Get(context.Background(), nil, "https://example1.com")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.Get(context.Background(), nil, "https://example2.com:8888")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = c.Get(context.Background(), nil, "http://example3.com")
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientRetry(t *testing.T) {
	t.Parallel()
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:10036"
	engine := route.NewEngine(opt)
	engine.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.SetBodyString("pong")
	})
	go engine.Run()
	time.Sleep(time.Millisecond * 500)

	client, err := NewClient(WithDialTimeout(2 * time.Second))
	if err != nil {
		t.Fatal(err)
		return
	}
	startTime := time.Now().UnixNano()
	retryCfg, err := retry.NewRetryConfig(
		retry.WithMaxIdempotentCallAttempts(3),
		retry.WithDelay(1*time.Second),
		retry.WithMaxDelay(10*time.Second),
		retry.WithDelayPolicy(retry.CombineDelay(retry.FixedDelay, retry.BackOffDelay)),
		retry.WithRetryIf(func(req *protocol.Request, resp *protocol.Response, err error) bool {
			return regexp.MustCompile("connection has been closed").MatchString(fmt.Sprintln(err))
		}),
	)
	if err != nil {
		t.Fatal(err)
		return
	}
	client.SetRetryConfig(retryCfg)
	_, resp, err := client.Get(context.Background(), nil, "http://127.0.0.1:1234/ping")
	if err != nil {
		if time.Duration(time.Now().UnixNano()-startTime) > 8*time.Second && time.Duration(time.Now().UnixNano()-startTime) < 9*time.Second {
			t.Logf("Retry triggered : resp=%v\nerr=%v\n", string(resp), fmt.Sprintln(err))
		} else if time.Duration(time.Now().UnixNano()-startTime) < 1*time.Second { // Compatible without triggering retry
			t.Logf("Retry not triggered : resp=%v\nerr=%v\n", string(resp), fmt.Sprintln(err))
		} else {
			t.Fatal(err)
		}
		return
	}
	t.Logf("resp=%v\nerr=%v\n", string(resp), fmt.Sprintln(err))
}
