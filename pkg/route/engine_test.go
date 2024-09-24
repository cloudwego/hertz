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
	"github.com/cloudwego/hertz/pkg/app/server/binding"
	"github.com/cloudwego/hertz/pkg/app/server/registry"
	"github.com/cloudwego/hertz/pkg/common/config"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/suite"
	"github.com/cloudwego/hertz/pkg/route/param"
)

func TestNew_Engine(t *testing.T) {
	defaultTransporter = standard.NewTransporter
	opt := config.NewOptions([]config.Option{})
	router := NewEngine(opt)
	assert.DeepEqual(t, "standard", router.GetTransporterName())
	assert.DeepEqual(t, "/", router.basePath)
	assert.DeepEqual(t, router.engine, router)
	assert.DeepEqual(t, 0, len(router.Handlers))
}

func TestNew_Engine_WithTransporter(t *testing.T) {
	defaultTransporter = newMockTransporter
	opt := config.NewOptions([]config.Option{})
	router := NewEngine(opt)
	assert.DeepEqual(t, "route", router.GetTransporterName())

	defaultTransporter = newMockTransporter
	opt.TransporterNewer = standard.NewTransporter
	router = NewEngine(opt)
	assert.DeepEqual(t, "standard", router.GetTransporterName())
	assert.DeepEqual(t, "route", GetTransporterName())
}

func TestGetTransporterName(t *testing.T) {
	name := getTransporterName(&fakeTransporter{})
	assert.DeepEqual(t, "route", name)
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
			ctx.String(consts.StatusOK, ctx.Param(ctx.Query("key")))
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
		assert.DeepEqual(t, consts.StatusOK, w.Code)
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
			ctx.String(consts.StatusOK, ctx.Param(ctx.Query("key")))
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
		assert.DeepEqual(t, consts.StatusOK, w.Code)
		assert.DeepEqual(t, tr.want, w.Body.String())
	}
}

func TestConnectionClose(t *testing.T) {
	engine := NewEngine(config.NewOptions(nil))
	atomic.StoreUint32(&engine.status, statusRunning)
	engine.Init()
	engine.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusOK, "ok")
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
		ctx.String(consts.StatusOK, "ok")
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
		ctx.String(consts.StatusOK, "ok")
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
		ctx.String(consts.StatusOK, "ok")
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

func TestIdleTimeout03(t *testing.T) {
	engine := NewEngine(config.NewOptions(nil))
	engine.options.IdleTimeout = 0
	engine.transport = standard.NewTransporter(engine.options)
	atomic.StoreUint32(&engine.status, statusRunning)
	engine.Init()
	atomic.StoreUint32(&engine.status, statusRunning)
	engine.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(50 * time.Millisecond)
		ctx.String(consts.StatusOK, "ok")
	})

	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n" +
		"GET /foo HTTP/1.1\r\nHost: google.com\r\nConnection: close\r\n\r\n")

	ch := make(chan error)
	startCh := make(chan error)
	go func() {
		<-startCh
		ch <- engine.Serve(context.Background(), conn)
	}()
	close(startCh)
	select {
	case err := <-ch:
		if !errors.Is(err, errs.ErrShortConnection) {
			t.Errorf("err should be ErrShortConnection, but got %s", err)
		}
		return
	case <-time.Tick(200 * time.Millisecond):
		t.Errorf("timeout! should have been finished in 200ms...")
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
	assert.DeepEqual(t, consts.StatusOK, rr.Code)
	assert.DeepEqual(t, []byte("<h1>Date: 2017/07/01</h1>"), b)
	assert.DeepEqual(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestTransporterName(t *testing.T) {
	SetTransporter(standard.NewTransporter)
	assert.DeepEqual(t, "standard", GetTransporterName())

	SetTransporter(newMockTransporter)
	assert.DeepEqual(t, "route", GetTransporterName())
}

func newMockTransporter(options *config.Options) network.Transporter {
	return &mockTransporter{}
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
	assert.DeepEqual(t, consts.StatusOK, rr.Code)
	assert.DeepEqual(t, []byte("<h1>Date: 2017/07/01</h1>"), b)
	assert.DeepEqual(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestSetClientIPAndSetFormValue(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	e := NewEngine(opt)
	e.SetClientIPFunc(func(ctx *app.RequestContext) string {
		return "1.1.1.1"
	})
	e.SetFormValueFunc(func(requestContext *app.RequestContext, s string) []byte {
		return []byte(s)
	})
	e.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		assert.DeepEqual(t, ctx.ClientIP(), "1.1.1.1")
		assert.DeepEqual(t, string(ctx.FormValue("key")), "key")
	})

	_ = performRequest(e, "GET", "/ping")
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
	assert.DeepEqual(t, consts.StatusOK, rr.Code)
	assert.DeepEqual(t, []byte("<h1>Date: 2017/07/01</h1>"), b)
	assert.DeepEqual(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestSetEngineRun(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.Init()
	assert.True(t, !e.IsRunning())
	e.MarkAsRunning()
	assert.True(t, e.IsRunning())
}

type mockConn struct{}

func (m *mockConn) SetWriteTimeout(t time.Duration) error {
	// TODO implement me
	panic("implement me")
}

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

type fakeTransporter struct{}

func (f *fakeTransporter) Close() error {
	// TODO implement me
	panic("implement me")
}

func (f *fakeTransporter) Shutdown(ctx context.Context) error {
	// TODO implement me
	panic("implement me")
}

func (f *fakeTransporter) ListenAndServe(onData network.OnData) error {
	// TODO implement me
	panic("implement me")
}

type mockBinder struct{}

func (m *mockBinder) Name() string {
	return "test binder"
}

func (m *mockBinder) Bind(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *mockBinder) BindAndValidate(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *mockBinder) BindQuery(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *mockBinder) BindHeader(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *mockBinder) BindPath(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *mockBinder) BindForm(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *mockBinder) BindJSON(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *mockBinder) BindProtobuf(request *protocol.Request, i interface{}) error {
	return nil
}

type mockValidator struct{}

func (m *mockValidator) ValidateStruct(interface{}) error {
	return fmt.Errorf("test mock")
}

func (m *mockValidator) Engine() interface{} {
	return nil
}

func (m *mockValidator) ValidateTag() string {
	return "vd"
}

type mockNonValidator struct{}

func (m *mockNonValidator) ValidateStruct(interface{}) error {
	return fmt.Errorf("test mock")
}

func TestInitBinderAndValidator(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic, %v", r)
		}
	}()
	opt := config.NewOptions([]config.Option{})
	bindConfig := binding.NewBindConfig()
	bindConfig.LooseZeroMode = true
	opt.BindConfig = bindConfig
	binder := &mockBinder{}
	opt.CustomBinder = binder
	validator := &mockValidator{}
	opt.CustomValidator = validator
	NewEngine(opt)
	validateConfig := binding.NewValidateConfig()
	opt.ValidateConfig = validateConfig
	opt.CustomValidator = nil
	NewEngine(opt)
}

func TestInitBinderAndValidatorForPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expect a panic, but get nil")
		}
	}()
	opt := config.NewOptions([]config.Option{})
	bindConfig := binding.NewBindConfig()
	bindConfig.LooseZeroMode = true
	opt.BindConfig = bindConfig
	binder := &mockBinder{}
	opt.CustomBinder = binder
	nonValidator := &mockNonValidator{}
	opt.CustomValidator = nonValidator
	NewEngine(opt)
}

func TestBindConfig(t *testing.T) {
	type Req struct {
		A int `query:"a"`
	}
	opt := config.NewOptions([]config.Option{})
	bindConfig := binding.NewBindConfig()
	bindConfig.LooseZeroMode = false
	opt.BindConfig = bindConfig
	e := NewEngine(opt)
	e.GET("/bind", func(c context.Context, ctx *app.RequestContext) {
		var req Req
		err := ctx.BindAndValidate(&req)
		if err == nil {
			t.Fatal("expect an error")
		}
	})
	performRequest(e, "GET", "/bind?a=")

	bindConfig = binding.NewBindConfig()
	bindConfig.LooseZeroMode = true
	opt.BindConfig = bindConfig
	e = NewEngine(opt)
	e.GET("/bind", func(c context.Context, ctx *app.RequestContext) {
		var req Req
		err := ctx.BindAndValidate(&req)
		if err != nil {
			t.Fatal("unexpected error")
		}
		assert.DeepEqual(t, 0, req.A)
	})
	performRequest(e, "GET", "/bind?a=")
}

type ValidateError struct {
	ErrType, FailField, Msg string
}

// Error implements error interface.
func (e *ValidateError) Error() string {
	if e.Msg != "" {
		return e.ErrType + ": expr_path=" + e.FailField + ", cause=" + e.Msg
	}
	return e.ErrType + ": expr_path=" + e.FailField + ", cause=invalid"
}

func TestValidateConfigSetErrorFactory(t *testing.T) {
	type TestValidate struct {
		B int `query:"b" vd:"$>100"`
	}
	opt := config.NewOptions([]config.Option{})
	CustomValidateErrFunc := func(failField, msg string) error {
		err := ValidateError{
			ErrType:   "validateErr",
			FailField: "[validateFailField]: " + failField,
			Msg:       "[validateErrMsg]: " + msg,
		}

		return &err
	}

	validateConfig := binding.NewValidateConfig()
	validateConfig.SetValidatorErrorFactory(CustomValidateErrFunc)
	opt.ValidateConfig = validateConfig
	e := NewEngine(opt)
	e.GET("/bind", func(c context.Context, ctx *app.RequestContext) {
		var req TestValidate
		err := ctx.BindAndValidate(&req)
		if err == nil {
			t.Fatal("expect an error")
		}
		assert.DeepEqual(t, "validateErr: expr_path=[validateFailField]: B, cause=[validateErrMsg]: ", err.Error())
	})
	performRequest(e, "GET", "/bind?b=1")
}

func TestCustomBinder(t *testing.T) {
	type Req struct {
		A int `query:"a"`
	}
	opt := config.NewOptions([]config.Option{})
	opt.CustomBinder = &mockBinder{}
	e := NewEngine(opt)
	e.GET("/bind", func(c context.Context, ctx *app.RequestContext) {
		var req Req
		err := ctx.BindAndValidate(&req)
		if err != nil {
			t.Fatal("unexpected error")
		}
		assert.NotEqual(t, 2, req.A)
	})
	performRequest(e, "GET", "/bind?a=2")
}

func TestValidateRegValidateFunc(t *testing.T) {
	type Req struct {
		A int `query:"a" vd:"f($)"`
	}
	opt := config.NewOptions([]config.Option{})
	validateConfig := &binding.ValidateConfig{}
	validateConfig.MustRegValidateFunc("f", func(args ...interface{}) error {
		return fmt.Errorf("test error")
	})
	e := NewEngine(opt)
	e.GET("/validate", func(c context.Context, ctx *app.RequestContext) {
		var req Req
		err := ctx.BindAndValidate(&req)
		assert.NotNil(t, err)
		assert.DeepEqual(t, "test error", err.Error())
	})
	performRequest(e, "GET", "/validate?a=2")
}

func TestCustomValidator(t *testing.T) {
	type Req struct {
		A int `query:"a" vd:"d($)"`
	}
	opt := config.NewOptions([]config.Option{})
	validateConfig := &binding.ValidateConfig{}
	validateConfig.MustRegValidateFunc("d", func(args ...interface{}) error {
		return fmt.Errorf("test error")
	})
	opt.CustomValidator = &mockValidator{}
	e := NewEngine(opt)
	e.GET("/validate", func(c context.Context, ctx *app.RequestContext) {
		var req Req
		err := ctx.BindAndValidate(&req)
		assert.NotNil(t, err)
		assert.DeepEqual(t, "test mock", err.Error())
	})
	performRequest(e, "GET", "/validate?a=2")
}

var errTestDeregsitry = fmt.Errorf("test deregsitry error")

type mockDeregsitryErr struct{}

var _ registry.Registry = &mockDeregsitryErr{}

func (e mockDeregsitryErr) Register(*registry.Info) error {
	return nil
}

func (e mockDeregsitryErr) Deregister(*registry.Info) error {
	return errTestDeregsitry
}

func TestEngineShutdown(t *testing.T) {
	defaultTransporter = standard.NewTransporter
	mockCtxCallback := func(ctx context.Context) {}
	// Test case 1: serve not running error
	opt := config.NewOptions(nil)
	opt.Addr = "127.0.0.1:10027"
	engine := NewEngine(opt)
	ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second)
	defer cancel1()
	err := engine.Shutdown(ctx1)
	assert.DeepEqual(t, errStatusNotRunning, err)

	// Test case 2: serve successfully running and shutdown
	engine = NewEngine(opt)
	engine.OnShutdown = []CtxCallback{mockCtxCallback}
	go func() {
		engine.Run()
	}()
	// wait for engine to start
	time.Sleep(1 * time.Second)

	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	err = engine.Shutdown(ctx2)
	assert.Nil(t, err)
	assert.DeepEqual(t, statusClosed, atomic.LoadUint32(&engine.status))

	// Test case 3: serve successfully running and shutdown with deregistry error
	engine = NewEngine(opt)
	engine.OnShutdown = []CtxCallback{mockCtxCallback}
	engine.options.Registry = &mockDeregsitryErr{}
	go func() {
		engine.Run()
	}()
	// wait for engine to start
	time.Sleep(1 * time.Second)

	ctx3, cancel3 := context.WithTimeout(context.Background(), time.Second)
	defer cancel3()
	err = engine.Shutdown(ctx3)
	assert.DeepEqual(t, errTestDeregsitry, err)
	assert.DeepEqual(t, statusShutdown, atomic.LoadUint32(&engine.status))
}

type mockStreamer struct{}

type mockProtocolServer struct{}

func (s *mockStreamer) Serve(c context.Context, conn network.StreamConn) error {
	return nil
}

func (s *mockProtocolServer) Serve(c context.Context, conn network.Conn) error {
	return nil
}

type mockStreamConn struct {
	network.StreamConn
	version string
}

var _ network.StreamConn = &mockStreamConn{}

func (m *mockStreamConn) GetVersion() uint32 {
	return network.Version1
}

func TestEngineServeStream(t *testing.T) {
	engine := &Engine{
		options: &config.Options{
			ALPN: true,
			TLS:  &tls.Config{},
		},
		protocolStreamServers: map[string]protocol.StreamServer{
			suite.HTTP3: &mockStreamer{},
		},
	}

	// Test ALPN path
	conn := &mockStreamConn{version: suite.HTTP3}
	err := engine.ServeStream(context.Background(), conn)
	assert.Nil(t, err)

	// Test default path
	engine.options.ALPN = false
	conn = &mockStreamConn{}
	err = engine.ServeStream(context.Background(), conn)
	assert.Nil(t, err)

	// Test unsupported protocol
	engine.protocolStreamServers = map[string]protocol.StreamServer{}
	conn = &mockStreamConn{}
	err = engine.ServeStream(context.Background(), conn)
	assert.DeepEqual(t, errs.ErrNotSupportProtocol, err)
}

func TestEngineServe(t *testing.T) {
	engine := NewEngine(config.NewOptions(nil))
	engine.protocolServers[suite.HTTP1] = &mockProtocolServer{}
	engine.protocolServers[suite.HTTP2] = &mockProtocolServer{}

	// test H2C path
	ctx := context.Background()
	conn := mock.NewConn("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
	engine.options.H2C = true
	err := engine.Serve(ctx, conn)
	assert.Nil(t, err)

	// test ALPN path
	ctx = context.Background()
	conn = mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")
	engine.options.H2C = false
	engine.options.ALPN = true
	engine.options.TLS = &tls.Config{}
	err = engine.Serve(ctx, conn)
	assert.Nil(t, err)

	// test HTTP1 path
	engine.options.ALPN = false
	err = engine.Serve(ctx, conn)
	assert.Nil(t, err)
}

func TestOndata(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(config.NewOptions(nil))

	// test stream conn
	streamConn := &mockStreamConn{version: suite.HTTP3}
	engine.protocolStreamServers[suite.HTTP3] = &mockStreamer{}
	err := engine.onData(ctx, streamConn)
	assert.Nil(t, err)

	// test conn
	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")
	engine.protocolServers[suite.HTTP1] = &mockProtocolServer{}
	err = engine.onData(ctx, conn)
	assert.Nil(t, err)
}

func TestAcquireHijackConn(t *testing.T) {
	engine := &Engine{
		NoHijackConnPool: false,
	}
	// test conn pool
	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")
	hijackConn := engine.acquireHijackConn(conn)
	assert.NotNil(t, hijackConn)
	assert.NotNil(t, hijackConn.Conn)
	assert.DeepEqual(t, engine, hijackConn.e)
	assert.DeepEqual(t, conn, hijackConn.Conn)

	// test no conn pool
	engine.NoHijackConnPool = true
	hijackConn = engine.acquireHijackConn(conn)
	assert.NotNil(t, hijackConn)
	assert.NotNil(t, hijackConn.Conn)
	assert.DeepEqual(t, engine, hijackConn.e)
	assert.DeepEqual(t, conn, hijackConn.Conn)
}

func TestHandleParamsReassignInHandleFunc(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	routes := []string{
		"/:a/:b/:c",
	}
	for _, r := range routes {
		e.GET(r, func(c context.Context, ctx *app.RequestContext) {
			ctx.Params = make([]param.Param, 1)
			ctx.String(consts.StatusOK, "")
		})
	}
	testRoutes := []string{
		"/aaa/bbb/ccc",
		"/asd/alskja/alkdjad",
		"/asd/alskja/alkdjad",
		"/asd/alskja/alkdjad",
		"/asd/alskja/alkdjad",
		"/alksjdlakjd/ooo/askda",
		"/alksjdlakjd/ooo/askda",
		"/alksjdlakjd/ooo/askda",
	}
	ctx := e.ctxPool.Get().(*app.RequestContext)
	for _, tr := range testRoutes {
		r := protocol.NewRequest(http.MethodGet, tr, nil)
		r.CopyTo(&ctx.Request)
		e.ServeHTTP(context.Background(), ctx)
		ctx.ResetWithoutConn()
	}
}
