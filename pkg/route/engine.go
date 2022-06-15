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
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"io"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/internal/nocopy"
	internalStats "github.com/cloudwego/hertz/internal/stats"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server/render"
	"github.com/cloudwego/hertz/pkg/common/config"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/tracer"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	"github.com/cloudwego/hertz/pkg/common/tracer/traceinfo"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1"
	"github.com/cloudwego/hertz/pkg/protocol/http1/factory"
	"github.com/cloudwego/hertz/pkg/protocol/suite"
)

var (
	defaultTransporter = standard.NewTransporter

	errInitFailed       = errs.NewPrivate("engine has been init already")
	errAlreadyRunning   = errs.NewPrivate("engine is already running")
	errStatusNotRunning = errs.NewPrivate("engine is not running")

	default404Body = []byte("404 page not found")
	default405Body = []byte("405 method not allowed")
	default400Body = []byte("400 bad request")
)

type hijackConn struct {
	network.Conn
	e *Engine
}

type CtxCallback func(ctx context.Context)

// RouteInfo represents a request route's specification which contains method and path and its handler.
type RouteInfo struct {
	Method      string
	Path        string
	Handler     string
	HandlerFunc app.HandlerFunc
}

// RoutesInfo defines a RouteInfo array.
type RoutesInfo []RouteInfo

type Engine struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	// engine name
	Name       string
	serverName atomic.Value

	// Options for route and protocol server
	options *config.Options

	// route
	RouterGroup
	trees MethodTrees

	maxParams uint16

	allNoMethod app.HandlersChain
	allNoRoute  app.HandlersChain
	noRoute     app.HandlersChain
	noMethod    app.HandlersChain

	// For render HTML
	delims     render.Delims
	funcMap    template.FuncMap
	htmlRender render.HTMLRender

	// hijack
	hijackConnPool sync.Pool
	// KeepHijackedConns is an opt-in disable of connection
	// close by hertz after connections' HijackHandler returns.
	// This allows to save goroutines, e.g. when hertz used to upgrade
	// http connections to WS and connection goes to another handler,
	// which will close it when needed.
	KeepHijackedConns bool

	// underlying transport
	transport network.Transporter

	// trace
	tracerCtl   tracer.Controller
	enableTrace bool

	// protocol layer management
	protocolSuite   *suite.Config
	protocolServers map[string]protocol.Server

	// RequestContext pool
	ctxPool sync.Pool

	// Function to handle panics recovered from http handlers.
	// It should be used to generate a error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecovered panics.
	PanicHandler app.HandlerFunc

	// ContinueHandler is called after receiving the Expect 100 Continue Header
	//
	// https://www.w3.org/Protocols/rfc2616/rfc2616-sec8.html#sec8.2.3
	// https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html#sec10.1.1
	// Using ContinueHandler a server can make decisioning on whether or not
	// to read a potentially large request body based on the headers
	//
	// The default is to automatically read request bodies of Expect 100 Continue requests
	// like they are normal requests
	ContinueHandler func(header *protocol.RequestHeader) bool

	// Indicates the engine status (Init/Running/Shutdown/Closed).
	status uint32

	// Hook functions get triggered sequentially before engine start to shutdown
	OnShutdown []CtxCallback
}

func (engine *Engine) IsTraceEnable() bool {
	return engine.enableTrace
}

func (engine *Engine) GetCtxPool() *sync.Pool {
	return &engine.ctxPool
}

func (engine *Engine) GetOptions() *config.Options {
	return engine.options
}

func SetTransporter(transporter func(options *config.Options) network.Transporter) {
	defaultTransporter = transporter
}

func GetTransporterName() (tName string) {
	defer func() {
		err := recover()
		if err != nil {
			tName = "unknown"
		}
	}()
	fName := runtime.FuncForPC(reflect.ValueOf(defaultTransporter).Pointer()).Name()
	fSlice := strings.Split(fName, "/")
	name := fSlice[len(fSlice)-1]
	fSlice = strings.Split(name, ".")
	tName = fSlice[0]
	return
}

func (engine *Engine) IsStreamRequestBody() bool {
	return engine.options.StreamRequestBody
}

func (engine *Engine) IsRunning() bool {
	return atomic.LoadUint32(&engine.status) == statusRunning
}

func (engine *Engine) HijackConnHandle(c network.Conn, h app.HijackHandler) {
	engine.hijackConnHandler(c, h)
}

func (engine *Engine) GetTracer() tracer.Controller {
	return engine.tracerCtl
}

const (
	_ uint32 = iota
	statusInitialized
	statusRunning
	statusShutdown
	statusClosed
)

// NewContext make a pure RequestContext without any http request/response information
//
// Set the Request filed before use it for handlers
func (engine *Engine) NewContext() *app.RequestContext {
	return app.NewContext(engine.maxParams)
}

// Shutdown starts the server's graceful exit by next steps:
//
// 1. Trigger OnShutdown hooks concurrently, but don't wait them
// 2. Close the net listener, which means new connection won't be accepted
// 3. Wait all connections get closed:
// 	One connection gets closed after reaching out the shorter time of processing
//	one request (in hand or next incoming), idleTimeout or ExitWaitTime
// 4. Exit
func (engine *Engine) Shutdown(ctx context.Context) (err error) {
	if engine.status != statusRunning {
		return errStatusNotRunning
	}
	if !atomic.CompareAndSwapUint32(&engine.status, statusRunning, statusShutdown) {
		return
	}

	// trigger hooks if any
	for i := range engine.OnShutdown {
		go func(index int) {
			engine.OnShutdown[index](ctx)
		}(i)
	}

	// call transport shutdown
	if err := engine.transport.Shutdown(ctx); err != ctx.Err() {
		return err
	}
	return
}

func (engine *Engine) Run() (err error) {
	if err = engine.Init(); err != nil {
		return err
	}

	if !atomic.CompareAndSwapUint32(&engine.status, statusInitialized, statusRunning) {
		return errAlreadyRunning
	}
	defer atomic.StoreUint32(&engine.status, statusClosed)

	return engine.listenAndServe()
}

func (engine *Engine) Init() error {
	if !h2Enable(engine.options) {
		engine.protocolSuite.Delete(suite.HTTP2)
	}

	// add built-in http1 server by default
	if !engine.HasServer(suite.HTTP1) {
		engine.AddProtocol(suite.HTTP1, factory.NewServerFactory(newHttp1OptionFromEngine(engine)))
	}

	serverMap, err := engine.protocolSuite.LoadAll(engine)
	if err != nil {
		return errs.New(err, errs.ErrorTypePrivate, "LoadAll protocol suite error")
	}

	engine.protocolServers = serverMap

	if engine.alpnEnable() {
		engine.options.TLS.NextProtos = append(engine.options.TLS.NextProtos, suite.HTTP1)
		engine.options.TLS.NextProtos = append(engine.options.TLS.NextProtos, suite.HTTP2)
	}

	if !atomic.CompareAndSwapUint32(&engine.status, 0, statusInitialized) {
		return errInitFailed
	}
	return nil
}

func (engine *Engine) alpnEnable() bool {
	return engine.options.TLS != nil && engine.options.ALPN
}

func (engine *Engine) listenAndServe() error {
	return engine.transport.ListenAndServe(engine.onData)
}

func (c *hijackConn) Close() error {
	if !c.e.KeepHijackedConns {
		// when we do not keep hijacked connections,
		// it is closed in hijackConnHandler.
		return nil
	}

	conn := c.Conn
	c.e.releaseHijackConn(c)
	return conn.Close()
}

func (engine *Engine) getNextProto(conn network.Conn) (proto string, err error) {
	if tlsConn, ok := conn.(network.ConnTLSer); ok {
		if engine.options.ReadTimeout > 0 {
			if err := conn.SetReadTimeout(engine.options.ReadTimeout); err != nil {
				hlog.Errorf("HERTZ: BUG: error in SetReadDeadline=%s: error=%s", engine.options.ReadTimeout, err)
			}
		}
		err = tlsConn.Handshake()
		if err == nil {
			proto = tlsConn.ConnectionState().NegotiatedProtocol
		}
	}
	return
}

func (engine *Engine) onData(c context.Context, conn network.Conn) (err error) {
	err = engine.Serve(c, conn)
	return
}

func errProcess(conn io.Closer, err error) {
	if err == nil {
		return
	}

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	// Quiet close the connection
	if errors.Is(err, errs.ErrIdleTimeout) {
		return
	}

	// Do not process the hijack connection error
	if errors.Is(err, errs.ErrHijacked) {
		err = nil
		return
	}

	// Get remote address
	rip := getRemoteAddrFromCloser(conn)

	// Handle Specific error
	if ie, ok := conn.(network.HandleSpecificError); ok {
		if ie.HandleSpecificError(err, rip) {
			return
		}
	}
	// other errors
	hlog.Errorf("HERTZ: Error=%s, remoteAddr=%s", err.Error(), rip)
}

func getRemoteAddrFromCloser(conn io.Closer) string {
	if c, ok := conn.(network.Conn); ok {
		if addr := c.RemoteAddr(); addr != nil {
			return addr.String()
		}
	}
	return ""
}

func (engine *Engine) Close() error {
	return engine.transport.Close()
}

func (engine *Engine) GetServerName() []byte {
	v := engine.serverName.Load()
	var serverName []byte
	if v == nil {
		serverName = []byte(engine.Name)
		if len(serverName) == 0 {
			serverName = bytestr.DefaultServerName
		}
		engine.serverName.Store(serverName)
	} else {
		serverName = v.([]byte)
	}
	return serverName
}

func (engine *Engine) Serve(c context.Context, conn network.Conn) (err error) {
	defer func() {
		errProcess(conn, err)
	}()

	// H2C path
	if engine.options.H2C {
		// protocol sniffer
		buf, _ := conn.Peek(len(bytestr.StrClientPreface))
		if bytes.Equal(buf, bytestr.StrClientPreface) && engine.protocolServers[suite.HTTP2] != nil {
			return engine.protocolServers[suite.HTTP2].Serve(c, conn)
		}
		hlog.Warnf("HERTZ: HTTP2 server is not loaded, request is going to fallback to HTTP1 server")
	}

	// ALPN path
	if engine.options.ALPN && engine.options.TLS != nil {
		proto, err1 := engine.getNextProto(conn)
		if err1 != nil {
			// The client closes the connection when handshake. So just ignore it.
			if err1 == io.EOF {
				return nil
			}
			if re, ok := err1.(tls.RecordHeaderError); ok && re.Conn != nil && utils.TLSRecordHeaderLooksLikeHTTP(re.RecordHeader) {
				io.WriteString(re.Conn, "HTTP/1.0 400 Bad Request\r\n\r\nClient sent an HTTP request to an HTTPS server.\n")
				re.Conn.Close()
				return re
			}
			return err1
		}
		if server, ok := engine.protocolServers[proto]; ok {
			return server.Serve(c, conn)
		}
	}

	// HTTP1 path
	err = engine.protocolServers[suite.HTTP1].Serve(c, conn)

	return
}

func NewEngine(opt *config.Options) *Engine {
	engine := &Engine{
		trees: make(MethodTrees, 0, 9),
		RouterGroup: RouterGroup{
			Handlers: nil,
			basePath: "/",
			root:     true,
		},
		transport:       defaultTransporter(opt),
		tracerCtl:       &internalStats.Controller{},
		protocolServers: make(map[string]protocol.Server),
		enableTrace:     true,
		options:         opt,
	}
	engine.RouterGroup.engine = engine

	traceLevel := initTrace(engine)

	// prepare RequestContext pool
	engine.ctxPool.New = func() interface{} {
		ctx := engine.allocateContext()
		if engine.enableTrace {
			ti := traceinfo.NewTraceInfo()
			ti.Stats().SetLevel(traceLevel)
			ctx.SetTraceInfo(ti)
		}
		return ctx
	}

	// Init protocolSuite
	engine.protocolSuite = suite.New()

	return engine
}

func initTrace(engine *Engine) stats.Level {
	for _, ti := range engine.options.Tracers {
		if tracer, ok := ti.(tracer.Tracer); ok {
			engine.tracerCtl.Append(tracer)
		}
	}

	if !engine.tracerCtl.HasTracer() {
		engine.enableTrace = false
	}

	traceLevel := stats.LevelDetailed
	if tl, ok := engine.options.TraceLevel.(stats.Level); ok {
		traceLevel = tl
	}
	return traceLevel
}

func h2Enable(opt *config.Options) bool {
	return opt.H2C || (opt.TLS != nil && opt.ALPN)
}

func debugPrintRoute(httpMethod, absolutePath string, handlers app.HandlersChain) {
	nuHandlers := len(handlers)
	handlerName := app.GetHandlerName(handlers.Last())
	if handlerName == "" {
		handlerName = utils.NameOfFunction(handlers.Last())
	}
	hlog.Debugf("HERTZ: Method=%-6s absolutePath=%-25s --> handlerName=%s (num=%d handlers)", httpMethod, absolutePath, handlerName, nuHandlers)
}

func (engine *Engine) addRoute(method, path string, handlers app.HandlersChain) {
	if len(path) == 0 {
		panic("path should not be ''")
	}
	utils.Assert(path[0] == '/', "path must begin with '/'")
	utils.Assert(method != "", "HTTP method can not be empty")
	utils.Assert(len(handlers) > 0, "there must be at least one handler")

	debugPrintRoute(method, path, handlers)
	methodRouter := engine.trees.get(method)
	if methodRouter == nil {
		methodRouter = &router{method: method, root: &node{}, hasTsrHandler: make(map[string]bool)}
		engine.trees = append(engine.trees, methodRouter)
	}
	methodRouter.addRoute(path, handlers)

	// Update maxParams
	if paramsCount := countParams(path); paramsCount > engine.maxParams {
		engine.maxParams = paramsCount
	}
}

func (engine *Engine) PrintRoute(method string) {
	root := engine.trees.get(method)
	printNode(root.root, 0)
}

// debug use
func printNode(node *node, level int) {
	fmt.Println("node.prefix: " + node.prefix)
	fmt.Println("node.ppath: " + node.ppath)
	fmt.Printf("level: %#v\n\n", level)
	for i := 0; i < len(node.children); i++ {
		printNode(node.children[i], level+1)
	}
}

func (engine *Engine) recv(ctx *app.RequestContext) {
	if rcv := recover(); rcv != nil {
		engine.PanicHandler(context.Background(), ctx)
	}
}

// ServeHTTP makes the router implement the Handler interface.
func (engine *Engine) ServeHTTP(c context.Context, ctx *app.RequestContext) {
	if engine.PanicHandler != nil {
		defer engine.recv(ctx)
	}

	rPath := string(ctx.Request.URI().Path())
	httpMethod := bytesconv.B2s(ctx.Request.Header.Method())
	unescape := false
	if engine.options.UseRawPath {
		rPath = string(ctx.Request.URI().PathOriginal())
		unescape = engine.options.UnescapePathValues
	}

	if engine.options.RemoveExtraSlash {
		rPath = utils.CleanPath(rPath)
	}

	// Follow RFC7230#section-5.3
	if rPath == "" || rPath[0] != '/' {
		serveError(c, ctx, consts.StatusBadRequest, default400Body)
		return
	}

	// Find root of the tree for the given HTTP method
	t := engine.trees
	paramsPointer := &ctx.Params
	for i, tl := 0, len(t); i < tl; i++ {
		if t[i].method != httpMethod {
			continue
		}
		// Find route in tree
		value := t[i].find(rPath, paramsPointer, unescape)

		if value.handlers != nil {
			ctx.SetHandlers(value.handlers)
			ctx.SetFullPath(value.fullPath)
			ctx.Next(c)
			return
		}
		if httpMethod != consts.MethodConnect && rPath != "/" {
			if value.tsr && engine.options.RedirectTrailingSlash {
				redirectTrailingSlash(ctx)
				return
			}
			if engine.options.RedirectFixedPath && redirectFixedPath(ctx, t[i].root, engine.options.RedirectFixedPath) {
				return
			}
		}
		break
	}

	if engine.options.HandleMethodNotAllowed {
		for _, tree := range engine.trees {
			if tree.method == httpMethod {
				continue
			}
			if value := tree.find(rPath, paramsPointer, unescape); value.handlers != nil {
				ctx.SetHandlers(engine.allNoMethod)
				serveError(c, ctx, consts.StatusMethodNotAllowed, default405Body)
				return
			}
		}
	}
	ctx.SetHandlers(engine.allNoRoute)
	serveError(c, ctx, consts.StatusNotFound, default404Body)
}

func (engine *Engine) allocateContext() *app.RequestContext {
	ctx := engine.NewContext()
	ctx.Request.SetMaxKeepBodySize(engine.options.MaxKeepBodySize)
	ctx.Response.SetMaxKeepBodySize(engine.options.MaxKeepBodySize)
	return ctx
}

func serveError(c context.Context, ctx *app.RequestContext, code int, defaultMessage []byte) {
	ctx.SetStatusCode(code)
	ctx.Next(c)
	if ctx.Response.StatusCode() == code {
		// if body exists(maybe customized by users), leave it alone.
		if ctx.Response.HasBodyBytes() || ctx.Response.IsBodyStream() {
			return
		}
		ctx.Response.Header.Set("Content-Type", "text/plain")
		ctx.Response.SetBody(defaultMessage)
	}
}

func trailingSlashURL(ts string) string {
	tmpURI := ts + "/"
	if length := len(ts); length > 1 && ts[length-1] == '/' {
		tmpURI = ts[:length-1]
	}
	return tmpURI
}

func redirectTrailingSlash(c *app.RequestContext) {
	p := bytesconv.B2s(c.Request.URI().Path())
	if prefix := utils.CleanPath(bytesconv.B2s(c.Request.Header.Peek("X-Forwarded-Prefix"))); prefix != "." {
		p = prefix + "/" + p
	}

	tmpURI := trailingSlashURL(p)

	query := c.Request.URI().QueryString()

	if len(query) > 0 {
		tmpURI = tmpURI + "?" + bytesconv.B2s(query)
	}

	c.Request.SetRequestURI(tmpURI)
	redirectRequest(c)
}

func redirectRequest(c *app.RequestContext) {
	code := consts.StatusMovedPermanently // Permanent redirect, request with GET method
	if bytesconv.B2s(c.Request.Header.Method()) != consts.MethodGet {
		code = consts.StatusTemporaryRedirect
	}

	c.Redirect(code, c.Request.URI().RequestURI())
}

func redirectFixedPath(c *app.RequestContext, root *node, trailingSlash bool) bool {
	rPath := bytesconv.B2s(c.Request.URI().Path())
	if fixedPath, ok := root.findCaseInsensitivePath(utils.CleanPath(rPath), trailingSlash); ok {
		c.Request.SetRequestURI(bytesconv.B2s(fixedPath))
		redirectRequest(c)
		return true
	}
	return false
}

// NoRoute adds handlers for NoRoute. It return a 404 code by default.
func (engine *Engine) NoRoute(handlers ...app.HandlerFunc) {
	engine.noRoute = handlers
	engine.rebuild404Handlers()
}

// NoMethod sets the handlers called when the HTTP method does not match.
func (engine *Engine) NoMethod(handlers ...app.HandlerFunc) {
	engine.noMethod = handlers
	engine.rebuild405Handlers()
}

func (engine *Engine) rebuild404Handlers() {
	engine.allNoRoute = engine.combineHandlers(engine.noRoute)
}

func (engine *Engine) rebuild405Handlers() {
	engine.allNoMethod = engine.combineHandlers(engine.noMethod)
}

// Use attaches a global middleware to the router. ie. the middleware attached though Use() will be
// included in the handlers chain for every single request. Even 404, 405, static files...
//
// For example, this is the right place for a logger or error management middleware.
func (engine *Engine) Use(middleware ...app.HandlerFunc) IRoutes {
	engine.RouterGroup.Use(middleware...)
	engine.rebuild404Handlers()
	engine.rebuild405Handlers()
	return engine
}

// LoadHTMLGlob loads HTML files identified by glob pattern
// and associates the result with HTML renderer.
func (engine *Engine) LoadHTMLGlob(pattern string) {
	left := engine.delims.Left
	right := engine.delims.Right
	templ := template.Must(template.New("").Delims(left, right).Funcs(engine.funcMap).ParseGlob(pattern))

	engine.SetHTMLTemplate(templ)
}

// LoadHTMLFiles loads a slice of HTML files
// and associates the result with HTML renderer.
func (engine *Engine) LoadHTMLFiles(files ...string) {
	templ := template.Must(template.New("").Delims(engine.delims.Left, engine.delims.Right).Funcs(engine.funcMap).ParseFiles(files...))
	engine.SetHTMLTemplate(templ)
}

// SetHTMLTemplate associate a template with HTML renderer.
func (engine *Engine) SetHTMLTemplate(templ *template.Template) {
	engine.htmlRender = render.HTMLProduction{Template: templ.Funcs(engine.funcMap)}
}

// SetFuncMap sets the funcMap used for template.funcMap.
func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

// Delims sets template left and right delims and returns a Engine instance.
func (engine *Engine) Delims(left, right string) *Engine {
	engine.delims = render.Delims{Left: left, Right: right}
	return engine
}

func (engine *Engine) acquireHijackConn(c network.Conn) *hijackConn {
	v := engine.hijackConnPool.Get()
	if v == nil {
		hjc := &hijackConn{
			Conn: c,
			e:    engine,
		}
		return hjc
	}
	hjc := v.(*hijackConn)
	hjc.Conn = c
	return hjc
}

func (engine *Engine) releaseHijackConn(hjc *hijackConn) {
	hjc.Conn = nil
	engine.hijackConnPool.Put(hjc)
}

func (engine *Engine) hijackConnHandler(c network.Conn, h app.HijackHandler) {
	hjc := engine.acquireHijackConn(c)
	h(hjc)

	if !engine.KeepHijackedConns {
		c.Close()
		engine.releaseHijackConn(hjc)
	}
}

// Routes returns a slice of registered routes, including some useful information, such as:
// the http method, path and the handler name.
func (engine *Engine) Routes() (routes RoutesInfo) {
	for _, tree := range engine.trees {
		routes = iterate(tree.method, routes, tree.root)
	}

	return routes
}

func (engine *Engine) AddProtocol(protocol string, factory suite.ServerFactory) {
	engine.protocolSuite.Add(protocol, factory)
}

func (engine *Engine) HasServer(name string) bool {
	return engine.protocolSuite.Get(name) != nil
}

// iterate iterates the method tree by depth firstly.
func iterate(method string, routes RoutesInfo, root *node) RoutesInfo {
	if len(root.handlers) > 0 {
		handlerFunc := root.handlers.Last()
		routes = append(routes, RouteInfo{
			Method:      method,
			Path:        root.ppath,
			Handler:     utils.NameOfFunction(handlerFunc),
			HandlerFunc: handlerFunc,
		})
	}

	for _, child := range root.children {
		routes = iterate(method, routes, child)
	}

	if root.paramChild != nil {
		routes = iterate(method, routes, root.paramChild)
	}

	if root.anyChild != nil {
		routes = iterate(method, routes, root.anyChild)
	}
	return routes
}

// for built-in http1 impl only.
func newHttp1OptionFromEngine(engine *Engine) *http1.Option {
	return &http1.Option{
		StreamRequestBody:            engine.options.StreamRequestBody,
		GetOnly:                      engine.options.GetOnly,
		DisablePreParseMultipartForm: engine.options.DisablePreParseMultipartForm,
		DisableKeepalive:             engine.options.DisableKeepalive,
		NoDefaultServerHeader:        engine.options.NoDefaultServerHeader,
		MaxRequestBodySize:           engine.options.MaxRequestBodySize,
		IdleTimeout:                  engine.options.IdleTimeout,
		ReadTimeout:                  engine.options.ReadTimeout,
		ServerName:                   engine.GetServerName(),
		ContinueHandler:              engine.ContinueHandler,
		TLS:                          engine.options.TLS,
		HTMLRender:                   engine.htmlRender,
		EnableTrace:                  engine.IsTraceEnable(),
		HijackConnHandle:             engine.HijackConnHandle,
	}
}
