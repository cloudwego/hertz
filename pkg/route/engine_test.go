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
 * The MIT License (MIT)
 *
 * Copyright (c) 2014 Manuel Martínez-Almeida
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
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package route

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/netpoll"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

func TestNew_Engine(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	router := NewEngine(opt)
	assert.DeepEqual(t, "/", router.basePath)
	assert.DeepEqual(t, router.engine, router)
	assert.DeepEqual(t, 0, len(router.Handlers))
}

func TestEngineUnescape(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	routes := []string{
		"/*all",
		"/cmd/:tool/",
		"/src/*filepath",
		"/search/:query",
		"/info/:user/project/:project",
		"/info/:user",
	}

	for _, r := range routes {
		e.GET(r, func(c context.Context, ctx *app.RequestContext) {
			ctx.String(200, ctx.Param(ctx.Query("key")))
		})
	}

	testRoutes := []struct {
		route string
		key   string
		want  string
	}{
		{"/", "", ""},
		{"/cmd/%E4%BD%A0%E5%A5%BD/", "tool", "你好"},
		{"/src/some/%E4%B8%96%E7%95%8C.png", "filepath", "some/世界.png"},
		{"/info/%E4%BD%A0%E5%A5%BD/project/%E4%B8%96%E7%95%8C", "user", "你好"},
		{"/info/%E4%BD%A0%E5%A5%BD/project/%E4%B8%96%E7%95%8C", "project", "世界"},
	}
	for _, tr := range testRoutes {
		w := performRequest(e, http.MethodGet, tr.route+"?key="+tr.key)
		assert.DeepEqual(t, 200, w.Code)
		assert.DeepEqual(t, tr.want, w.Body.String())
	}
}

func TestEngineUnescapeRaw(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.options.UseRawPath = true

	routes := []string{
		"/*all",
		"/cmd/:tool/",
		"/src/*filepath",
		"/search/:query",
		"/info/:user/project/:project",
		"/info/:user",
	}

	for _, r := range routes {
		e.GET(r, func(c context.Context, ctx *app.RequestContext) {
			ctx.String(200, ctx.Param(ctx.Query("key")))
		})
	}

	testRoutes := []struct {
		route string
		key   string
		want  string
	}{
		{"/", "", ""},
		{"/cmd/test/", "tool", "test"},
		{"/src/some/file.png", "filepath", "some/file.png"},
		{"/src/some/file+test.png", "filepath", "some/file test.png"},
		{"/src/some/file++++%%%%test.png", "filepath", "some/file++++%%%%test.png"},
		{"/src/some/file%2Ftest.png", "filepath", "some/file/test.png"},
		{"/search/someth!ng+in+ünìcodé", "query", "someth!ng in ünìcodé"},
		{"/info/gordon/project/go", "user", "gordon"},
		{"/info/gordon/project/go", "project", "go"},
		{"/info/slash%2Fgordon", "user", "slash/gordon"},
		{"/info/slash%2Fgordon/project/Project%20%231", "user", "slash/gordon"},
		{"/info/slash%2Fgordon/project/Project%20%231", "project", "Project #1"},
		{"/info/slash%%%%", "user", "slash%%%%"},
		{"/info/slash%%%%2Fgordon/project/Project%%%%20%231", "user", "slash%%%%2Fgordon"},
		{"/info/slash%%%%2Fgordon/project/Project%%%%20%231", "project", "Project%%%%20%231"},
	}
	for _, tr := range testRoutes {
		w := performRequest(e, http.MethodGet, tr.route+"?key="+tr.key)
		assert.DeepEqual(t, 200, w.Code)
		assert.DeepEqual(t, tr.want, w.Body.String())
	}
}

func TestConnectionClose(t *testing.T) {
	engine := NewEngine(config.NewOptions(nil))
	atomic.StoreUint32(&engine.status, statusRunning)
	engine.Init()
	engine.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(200, "ok")
	})
	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\nConnection: close\r\n\r\n")
	err := engine.Serve(context.Background(), conn)
	assert.True(t, errors.Is(err, errs.ErrShortConnection))
}

func TestConnectionClose01(t *testing.T) {
	engine := NewEngine(config.NewOptions(nil))
	atomic.StoreUint32(&engine.status, statusRunning)
	engine.Init()
	engine.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		ctx.SetConnectionClose()
		ctx.String(200, "ok")
	})
	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")
	err := engine.Serve(context.Background(), conn)
	assert.True(t, errors.Is(err, errs.ErrShortConnection))
}

func TestIdleTimeout(t *testing.T) {
	engine := NewEngine(config.NewOptions(nil))
	engine.options.IdleTimeout = 0
	atomic.StoreUint32(&engine.status, statusRunning)
	engine.Init()
	engine.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(100 * time.Millisecond)
		ctx.String(200, "ok")
	})

	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")

	ch := make(chan error)
	startCh := make(chan error)
	go func() {
		<-startCh
		ch <- engine.Serve(context.Background(), conn)
	}()
	close(startCh)
	select {
	case err := <-ch:
		if err != nil {
			t.Errorf("err happened: %s", err)
		}
		return
	case <-time.Tick(120 * time.Millisecond):
		t.Errorf("timeout! should have been finished in 120ms...")
	}
}

func TestIdleTimeout01(t *testing.T) {
	engine := NewEngine(config.NewOptions(nil))
	engine.options.IdleTimeout = 1 * time.Second
	atomic.StoreUint32(&engine.status, statusRunning)
	engine.Init()
	atomic.StoreUint32(&engine.status, statusRunning)
	engine.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(10 * time.Millisecond)
		ctx.String(200, "ok")
	})

	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")

	ch := make(chan error)
	startCh := make(chan error)
	go func() {
		<-startCh
		ch <- engine.Serve(context.Background(), conn)
	}()
	close(startCh)
	select {
	case <-ch:
		t.Errorf("cannot return this early! should wait for at least 1s...")
	case <-time.Tick(1 * time.Second):
		return
	}
}

func TestEngine_Routes(t *testing.T) {
	engine := NewEngine(config.NewOptions(nil))
	engine.GET("/", handlerTest1)
	engine.GET("/user", handlerTest2)
	engine.GET("/user/:name/*action", handlerTest1)
	engine.GET("/anonymous1", func(c context.Context, ctx *app.RequestContext) {}) // TestEngine_Routes.func1
	engine.POST("/user", handlerTest2)
	engine.POST("/user/:name/*action", handlerTest2)
	engine.POST("/anonymous2", func(c context.Context, ctx *app.RequestContext) {}) // TestEngine_Routes.func2
	group := engine.Group("/v1")
	{
		group.GET("/user", handlerTest1)
		group.POST("/login", handlerTest2)
	}
	engine.Static("/static", ".")

	list := engine.Routes()

	assert.DeepEqual(t, 11, len(list))

	assertRoutePresent(t, list, RouteInfo{
		Method:  "GET",
		Path:    "/",
		Handler: "github.com/cloudwego/hertz/pkg/route.handlerTest1",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "GET",
		Path:    "/user",
		Handler: "github.com/cloudwego/hertz/pkg/route.handlerTest2",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "GET",
		Path:    "/user/:name/*action",
		Handler: "github.com/cloudwego/hertz/pkg/route.handlerTest1",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "GET",
		Path:    "/v1/user",
		Handler: "github.com/cloudwego/hertz/pkg/route.handlerTest1",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "GET",
		Path:    "/static/*filepath",
		Handler: "github.com/cloudwego/hertz/pkg/app.(*fsHandler).handleRequest-fm",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "GET",
		Path:    "/anonymous1",
		Handler: "github.com/cloudwego/hertz/pkg/route.TestEngine_Routes.func1",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "POST",
		Path:    "/user",
		Handler: "github.com/cloudwego/hertz/pkg/route.handlerTest2",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "POST",
		Path:    "/user/:name/*action",
		Handler: "github.com/cloudwego/hertz/pkg/route.handlerTest2",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "POST",
		Path:    "/anonymous2",
		Handler: "github.com/cloudwego/hertz/pkg/route.TestEngine_Routes.func2",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "POST",
		Path:    "/v1/login",
		Handler: "github.com/cloudwego/hertz/pkg/route.handlerTest2",
	})
	assertRoutePresent(t, list, RouteInfo{
		Method:  "HEAD",
		Path:    "/static/*filepath",
		Handler: "github.com/cloudwego/hertz/pkg/app.(*fsHandler).handleRequest-fm",
	})
}

func handlerTest1(c context.Context, ctx *app.RequestContext) {}

func handlerTest2(c context.Context, ctx *app.RequestContext) {}

func assertRoutePresent(t *testing.T, gets RoutesInfo, want RouteInfo) {
	for _, get := range gets {
		if get.Path == want.Path && get.Method == want.Method && get.Handler == want.Handler {
			return
		}
	}

	t.Errorf("route not found: %v", want)
}

func TestGetNextProto(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	conn := &mockConn{}
	proto, err := e.getNextProto(conn)
	if proto != "h2" {
		t.Errorf("unexpected proto: %#v, expected: %#v", proto, "h2")
	}
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func formatAsDate(t time.Time) string {
	year, month, day := t.Date()
	return fmt.Sprintf("%d/%02d/%02d", year, month, day)
}

func TestRenderHtml(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.Delims("{[{", "}]}")
	e.SetFuncMap(template.FuncMap{
		"formatAsDate": formatAsDate,
	})
	e.LoadHTMLGlob("../common/testdata/template/htmltemplate.html")
	e.GET("/templateName", func(c context.Context, ctx *app.RequestContext) {
		ctx.HTML(http.StatusOK, "htmltemplate.html", map[string]interface{}{
			"now": time.Date(2017, 0o7, 0o1, 0, 0, 0, 0, time.UTC),
		})
	})
	rr := performRequest(e, "GET", "/templateName")
	b, _ := ioutil.ReadAll(rr.Body)
	assert.DeepEqual(t, 200, rr.Code)
	assert.DeepEqual(t, []byte("<h1>Date: 2017/07/01</h1>"), b)
	assert.DeepEqual(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestTransporterName(t *testing.T) {
	SetTransporter(netpoll.NewTransporter)
	assert.DeepEqual(t, "netpoll", GetTransporterName())

	SetTransporter(standard.NewTransporter)
	assert.DeepEqual(t, "standard", GetTransporterName())

	SetTransporter(func(options *config.Options) network.Transporter {
		return &mockTransporter{}
	})
	assert.DeepEqual(t, "route", GetTransporterName())
}

type mockTransporter struct{}

func (m *mockTransporter) ListenAndServe(onData network.OnData) (err error) {
	panic("implement me")
}

func (m *mockTransporter) Close() error {
	panic("implement me")
}

func (m *mockTransporter) Shutdown(ctx context.Context) error {
	panic("implement me")
}

func TestRenderHtmlOfGlobWithAutoRender(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.AutoReloadRender = true
	e := NewEngine(opt)
	e.Delims("{[{", "}]}")
	e.SetFuncMap(template.FuncMap{
		"formatAsDate": formatAsDate,
	})
	e.LoadHTMLGlob("../common/testdata/template/htmltemplate.html")
	e.GET("/templateName", func(c context.Context, ctx *app.RequestContext) {
		ctx.HTML(http.StatusOK, "htmltemplate.html", map[string]interface{}{
			"now": time.Date(2017, 0o7, 0o1, 0, 0, 0, 0, time.UTC),
		})
	})
	rr := performRequest(e, "GET", "/templateName")
	b, _ := ioutil.ReadAll(rr.Body)
	assert.DeepEqual(t, 200, rr.Code)
	assert.DeepEqual(t, []byte("<h1>Date: 2017/07/01</h1>"), b)
	assert.DeepEqual(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestRenderHtmlOfFilesWithAutoRender(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	opt.AutoReloadRender = true
	e := NewEngine(opt)
	e.Delims("{[{", "}]}")
	e.SetFuncMap(template.FuncMap{
		"formatAsDate": formatAsDate,
	})
	e.LoadHTMLFiles("../common/testdata/template/htmltemplate.html")
	e.GET("/templateName", func(c context.Context, ctx *app.RequestContext) {
		ctx.HTML(http.StatusOK, "htmltemplate.html", map[string]interface{}{
			"now": time.Date(2017, 0o7, 0o1, 0, 0, 0, 0, time.UTC),
		})
	})
	rr := performRequest(e, "GET", "/templateName")
	b, _ := ioutil.ReadAll(rr.Body)
	assert.DeepEqual(t, 200, rr.Code)
	assert.DeepEqual(t, []byte("<h1>Date: 2017/07/01</h1>"), b)
	assert.DeepEqual(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
}

type mockConn struct{}

func (m *mockConn) ReadBinary(n int) (p []byte, err error) {
	panic("implement me")
}

func (m *mockConn) Handshake() error {
	return nil
}

func (m *mockConn) ConnectionState() tls.ConnectionState {
	return tls.ConnectionState{
		NegotiatedProtocol: "h2",
	}
}

func (m *mockConn) SetReadTimeout(t time.Duration) error {
	return nil
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	panic("implement me")
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	panic("implement me")
}

func (m *mockConn) Close() error {
	panic("implement me")
}

func (m *mockConn) LocalAddr() net.Addr {
	panic("implement me")
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("126.0.0.5"),
		Port: 8888,
		Zone: "",
	}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	panic("implement me")
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	panic("implement me")
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	panic("implement me")
}

func (m *mockConn) Release() error {
	panic("implement me")
}

func (m *mockConn) Peek(i int) ([]byte, error) {
	panic("implement me")
}

func (m *mockConn) Skip(n int) error {
	panic("implement me")
}

func (m *mockConn) ReadByte() (byte, error) {
	panic("implement me")
}

func (m *mockConn) Next(i int) ([]byte, error) {
	panic("implement me")
}

func (m *mockConn) Len() int {
	panic("implement me")
}

func (m *mockConn) Malloc(n int) (buf []byte, err error) {
	panic("implement me")
}

func (m *mockConn) WriteBinary(b []byte) (n int, err error) {
	panic("implement me")
}

func (m *mockConn) Flush() error {
	panic("implement me")
}
