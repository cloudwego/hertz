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

package app

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/app/server/binding"
	"github.com/cloudwego/hertz/pkg/app/server/render"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/tracer/traceinfo"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	rConsts "github.com/cloudwego/hertz/pkg/route/consts"
	"github.com/cloudwego/hertz/pkg/route/param"
)

var zeroTCPAddr = &net.TCPAddr{
	IP: net.IPv4zero,
}

type Handler interface {
	ServeHTTP(c context.Context, ctx *RequestContext)
}

type ClientIP func(ctx *RequestContext) string

type ClientIPOptions struct {
	RemoteIPHeaders []string
	TrustedCIDRs    []*net.IPNet
}

var defaultTrustedCIDRs = []*net.IPNet{
	{ // 0.0.0.0/0 (IPv4)
		IP:   net.IP{0x0, 0x0, 0x0, 0x0},
		Mask: net.IPMask{0x0, 0x0, 0x0, 0x0},
	},
	{ // ::/0 (IPv6)
		IP:   net.IP{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Mask: net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	},
}

var defaultClientIPOptions = ClientIPOptions{
	RemoteIPHeaders: []string{"X-Forwarded-For", "X-Real-IP"},
	TrustedCIDRs:    defaultTrustedCIDRs,
}

// ClientIPWithOption used to generate custom ClientIP function and set by engine.SetClientIPFunc
func ClientIPWithOption(opts ClientIPOptions) ClientIP {
	return func(ctx *RequestContext) string {
		RemoteIPHeaders := opts.RemoteIPHeaders
		TrustedCIDRs := opts.TrustedCIDRs

		remoteIPStr, _, err := net.SplitHostPort(strings.TrimSpace(ctx.RemoteAddr().String()))
		if err != nil {
			return ""
		}

		remoteIP := net.ParseIP(remoteIPStr)
		if remoteIP == nil {
			return ""
		}

		trusted := isTrustedProxy(TrustedCIDRs, remoteIP)

		if trusted {
			for _, headerName := range RemoteIPHeaders {
				ip, valid := validateHeader(TrustedCIDRs, ctx.Request.Header.Get(headerName))
				if valid {
					return ip
				}
			}
		}

		return remoteIPStr
	}
}

// isTrustedProxy will check whether the IP address is included in the trusted list according to trustedCIDRs
func isTrustedProxy(trustedCIDRs []*net.IPNet, remoteIP net.IP) bool {
	if trustedCIDRs == nil {
		return false
	}

	for _, cidr := range trustedCIDRs {
		if cidr.Contains(remoteIP) {
			return true
		}
	}
	return false
}

// validateHeader will parse X-Real-IP and X-Forwarded-For header and return the Initial client IP address or an untrusted IP address
func validateHeader(trustedCIDRs []*net.IPNet, header string) (clientIP string, valid bool) {
	if header == "" {
		return "", false
	}
	items := strings.Split(header, ",")
	for i := len(items) - 1; i >= 0; i-- {
		ipStr := strings.TrimSpace(items[i])
		ip := net.ParseIP(ipStr)
		if ip == nil {
			break
		}

		// X-Forwarded-For is appended by proxy
		// Check IPs in reverse order and stop when find untrusted proxy
		if (i == 0) || (!isTrustedProxy(trustedCIDRs, ip)) {
			return ipStr, true
		}
	}
	return "", false
}

var defaultClientIP = ClientIPWithOption(defaultClientIPOptions)

// SetClientIPFunc sets ClientIP function implementation to get ClientIP.
// Deprecated: Use engine.SetClientIPFunc instead of SetClientIPFunc
func SetClientIPFunc(fn ClientIP) {
	defaultClientIP = fn
}

type FormValueFunc func(*RequestContext, string) []byte

var defaultFormValue = func(ctx *RequestContext, key string) []byte {
	v := ctx.QueryArgs().Peek(key)
	if len(v) > 0 {
		return v
	}
	v = ctx.PostArgs().Peek(key)
	if len(v) > 0 {
		return v
	}
	mf, err := ctx.MultipartForm()
	if err == nil && mf.Value != nil {
		vv := mf.Value[key]
		if len(vv) > 0 {
			return []byte(vv[0])
		}
	}
	return nil
}

type RequestContext struct {
	conn     network.Conn
	Request  protocol.Request
	Response protocol.Response

	// Errors is a list of errors attached to all the handlers/middlewares who used this context.
	Errors errors.ErrorChain

	Params     param.Params
	handlers   HandlersChain
	fullPath   string
	index      int8
	HTMLRender render.HTMLRender

	// This mutex protect Keys map.
	mu sync.RWMutex

	// Keys is a key/value pair exclusively for the context of each request.
	Keys map[string]interface{}

	hijackHandler HijackHandler

	finishedMu sync.Mutex

	// finished means the request end.
	finished chan struct{}

	// traceInfo defines the trace information.
	traceInfo traceinfo.TraceInfo

	// enableTrace defines whether enable trace.
	enableTrace bool

	// clientIPFunc get client ip by use custom function.
	clientIPFunc ClientIP

	// clientIPFunc get form value by use custom function.
	formValueFunc FormValueFunc

	binder    binding.Binder
	validator binding.StructValidator
}

// Flush is the shortcut for ctx.Response.GetHijackWriter().Flush().
// Will return nil if the response writer is not hijacked.
func (ctx *RequestContext) Flush() error {
	if ctx.Response.GetHijackWriter() == nil {
		return nil
	}
	return ctx.Response.GetHijackWriter().Flush()
}

func (ctx *RequestContext) SetClientIPFunc(f ClientIP) {
	ctx.clientIPFunc = f
}

func (ctx *RequestContext) SetFormValueFunc(f FormValueFunc) {
	ctx.formValueFunc = f
}

func (ctx *RequestContext) SetBinder(binder binding.Binder) {
	ctx.binder = binder
}

func (ctx *RequestContext) SetValidator(validator binding.StructValidator) {
	ctx.validator = validator
}

func (ctx *RequestContext) GetTraceInfo() traceinfo.TraceInfo {
	return ctx.traceInfo
}

func (ctx *RequestContext) SetTraceInfo(t traceinfo.TraceInfo) {
	ctx.traceInfo = t
}

func (ctx *RequestContext) IsEnableTrace() bool {
	return ctx.enableTrace
}

// SetEnableTrace sets whether enable trace.
//
// NOTE: biz handler must not modify this value, otherwise, it may panic.
func (ctx *RequestContext) SetEnableTrace(enable bool) {
	ctx.enableTrace = enable
}

// NewContext make a pure RequestContext without any http request/response information
//
// Set the Request filed before use it for handlers
func NewContext(maxParams uint16) *RequestContext {
	v := make(param.Params, 0, maxParams)
	ctx := &RequestContext{Params: v, index: -1}
	return ctx
}

// Loop fn for every k/v in Keys
func (ctx *RequestContext) ForEachKey(fn func(k string, v interface{})) {
	ctx.mu.RLock()
	for key, val := range ctx.Keys {
		fn(key, val)
	}
	ctx.mu.RUnlock()
}

func (ctx *RequestContext) SetConn(c network.Conn) {
	ctx.conn = c
}

func (ctx *RequestContext) GetConn() network.Conn {
	return ctx.conn
}

func (ctx *RequestContext) SetHijackHandler(h HijackHandler) {
	ctx.hijackHandler = h
}

func (ctx *RequestContext) GetHijackHandler() HijackHandler {
	return ctx.hijackHandler
}

func (ctx *RequestContext) GetReader() network.Reader {
	return ctx.conn
}

func (ctx *RequestContext) GetWriter() network.Writer {
	return ctx.conn
}

func (ctx *RequestContext) GetIndex() int8 {
	return ctx.index
}

// SetIndex reset the handler's execution index
// Disclaimer: You can loop yourself to deal with this, use wisely.
func (ctx *RequestContext) SetIndex(index int8) {
	ctx.index = index
}

type HandlerFunc func(c context.Context, ctx *RequestContext)

// HandlersChain defines a HandlerFunc array.
type HandlersChain []HandlerFunc

type HandlerNameOperator interface {
	SetHandlerName(handler HandlerFunc, name string)
	GetHandlerName(handler HandlerFunc) string
}

func SetHandlerNameOperator(o HandlerNameOperator) {
	inbuiltHandlerNameOperator = o
}

type inbuiltHandlerNameOperatorStruct struct {
	handlerNames map[uintptr]string
}

func (o *inbuiltHandlerNameOperatorStruct) SetHandlerName(handler HandlerFunc, name string) {
	o.handlerNames[getFuncAddr(handler)] = name
}

func (o *inbuiltHandlerNameOperatorStruct) GetHandlerName(handler HandlerFunc) string {
	return o.handlerNames[getFuncAddr(handler)]
}

type concurrentHandlerNameOperatorStruct struct {
	handlerNames map[uintptr]string
	lock         sync.RWMutex
}

func (o *concurrentHandlerNameOperatorStruct) SetHandlerName(handler HandlerFunc, name string) {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.handlerNames[getFuncAddr(handler)] = name
}

func (o *concurrentHandlerNameOperatorStruct) GetHandlerName(handler HandlerFunc) string {
	o.lock.RLock()
	defer o.lock.RUnlock()
	return o.handlerNames[getFuncAddr(handler)]
}

func SetConcurrentHandlerNameOperator() {
	SetHandlerNameOperator(&concurrentHandlerNameOperatorStruct{handlerNames: map[uintptr]string{}})
}

func init() {
	inbuiltHandlerNameOperator = &inbuiltHandlerNameOperatorStruct{handlerNames: map[uintptr]string{}}
}

var inbuiltHandlerNameOperator HandlerNameOperator

func SetHandlerName(handler HandlerFunc, name string) {
	inbuiltHandlerNameOperator.SetHandlerName(handler, name)
}

func GetHandlerName(handler HandlerFunc) string {
	return inbuiltHandlerNameOperator.GetHandlerName(handler)
}

func getFuncAddr(v interface{}) uintptr {
	return reflect.ValueOf(reflect.ValueOf(v)).Field(1).Pointer()
}

// HijackHandler must process the hijacked connection c.
//
// If KeepHijackedConns is disabled, which is by default,
// the connection c is automatically closed after returning from HijackHandler.
//
// The connection c must not be used after returning from the handler, if KeepHijackedConns is disabled.
//
// When KeepHijackedConns enabled, hertz will not Close() the connection,
// you must do it when you need it. You must not use c in any way after calling Close().
//
// network.Connection provide two options of io: net.Conn and zero-copy read/write
type HijackHandler func(c network.Conn)

// Hijack registers the given handler for connection hijacking.
//
// The handler is called after returning from RequestHandler
// and sending http response. The current connection is passed
// to the handler. The connection is automatically closed after
// returning from the handler.
//
// The server skips calling the handler in the following cases:
//
//   - 'Connection: close' header exists in either request or response.
//   - Unexpected error during response writing to the connection.
//
// The server stops processing requests from hijacked connections.
//
// Server limits such as Concurrency, ReadTimeout, WriteTimeout, etc.
// aren't applied to hijacked connections.
//
// The handler must not retain references to ctx members.
//
// Arbitrary 'Connection: Upgrade' protocols may be implemented
// with HijackHandler. For instance,
//
//   - WebSocket ( https://en.wikipedia.org/wiki/WebSocket )
//   - HTTP/2.0 ( https://en.wikipedia.org/wiki/HTTP/2 )
func (ctx *RequestContext) Hijack(handler HijackHandler) {
	ctx.hijackHandler = handler
}

// Last returns the last handler of the handler chain.
//
// Generally speaking, the last handler is the main handler.
func (c HandlersChain) Last() HandlerFunc {
	if length := len(c); length > 0 {
		return c[length-1]
	}
	return nil
}

func (ctx *RequestContext) Finished() <-chan struct{} {
	ctx.finishedMu.Lock()
	if ctx.finished == nil {
		ctx.finished = make(chan struct{})
	}
	ch := ctx.finished
	ctx.finishedMu.Unlock()
	return ch
}

// GetRequest returns a copy of Request.
func (ctx *RequestContext) GetRequest() (dst *protocol.Request) {
	dst = &protocol.Request{}
	ctx.Request.CopyTo(dst)
	return
}

// GetResponse returns a copy of Response.
func (ctx *RequestContext) GetResponse() (dst *protocol.Response) {
	dst = &protocol.Response{}
	ctx.Response.CopyTo(dst)
	return
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
//
// In case the Key is reset after response, Value() return nil if ctx.Key is nil.
func (ctx *RequestContext) Value(key interface{}) interface{} {
	// this ctx has been reset, return nil.
	if ctx.Keys == nil {
		return nil
	}
	if keyString, ok := key.(string); ok {
		val, _ := ctx.Get(keyString)
		return val
	}
	return nil
}

// Hijacked returns true after Hijack is called.
func (ctx *RequestContext) Hijacked() bool {
	return ctx.hijackHandler != nil
}

// SetBodyStream sets response body stream and, optionally body size.
//
// bodyStream.Close() is called after finishing reading all body data
// if it implements io.Closer.
//
// If bodySize is >= 0, then bodySize bytes must be provided by bodyStream
// before returning io.EOF.
//
// If bodySize < 0, then bodyStream is read until io.EOF.
//
// See also SetBodyStreamWriter.
func (ctx *RequestContext) SetBodyStream(bodyStream io.Reader, bodySize int) {
	ctx.Response.SetBodyStream(bodyStream, bodySize)
}

// Host returns requested host.
//
// The host is valid until returning from RequestHandler.
func (ctx *RequestContext) Host() []byte {
	return ctx.URI().Host()
}

// RemoteAddr returns client address for the given request.
//
// If address is nil, it will return zeroTCPAddr.
func (ctx *RequestContext) RemoteAddr() net.Addr {
	if ctx.conn == nil {
		return zeroTCPAddr
	}
	addr := ctx.conn.RemoteAddr()
	if addr == nil {
		return zeroTCPAddr
	}
	return addr
}

// WriteString appends s to response body.
func (ctx *RequestContext) WriteString(s string) (int, error) {
	ctx.Response.AppendBodyString(s)
	return len(s), nil
}

// SetContentType sets response Content-Type.
func (ctx *RequestContext) SetContentType(contentType string) {
	ctx.Response.Header.SetContentType(contentType)
}

// Path returns requested path.
//
// The path is valid until returning from RequestHandler.
func (ctx *RequestContext) Path() []byte {
	return ctx.URI().Path()
}

// NotModified resets response and sets '304 Not Modified' response status code.
func (ctx *RequestContext) NotModified() {
	ctx.Response.Reset()
	ctx.SetStatusCode(consts.StatusNotModified)
}

// IfModifiedSince returns true if lastModified exceeds 'If-Modified-Since'
// value from the request header.
//
// The function returns true also 'If-Modified-Since' request header is missing.
func (ctx *RequestContext) IfModifiedSince(lastModified time.Time) bool {
	ifModStr := ctx.Request.Header.PeekIfModifiedSinceBytes()
	if len(ifModStr) == 0 {
		return true
	}
	ifMod, err := bytesconv.ParseHTTPDate(ifModStr)
	if err != nil {
		return true
	}
	lastModified = lastModified.Truncate(time.Second)
	return ifMod.Before(lastModified)
}

// URI returns requested uri.
//
// The uri is valid until returning from RequestHandler.
func (ctx *RequestContext) URI() *protocol.URI {
	return ctx.Request.URI()
}

func (ctx *RequestContext) String(code int, format string, values ...interface{}) {
	ctx.Render(code, render.String{Format: format, Data: values})
}

// FullPath returns a matched route full path. For not found routes
// returns an empty string.
//
//	router.GET("/user/:id", func(c context.Context, ctx *app.RequestContext) {
//	    ctx.FullPath() == "/user/:id" // true
//	})
func (ctx *RequestContext) FullPath() string {
	return ctx.fullPath
}

func (ctx *RequestContext) SetFullPath(p string) {
	ctx.fullPath = p
}

// SetStatusCode sets response status code.
func (ctx *RequestContext) SetStatusCode(statusCode int) {
	ctx.Response.SetStatusCode(statusCode)
}

// Write writes p into response body.
func (ctx *RequestContext) Write(p []byte) (int, error) {
	ctx.Response.AppendBody(p)
	return len(p), nil
}

// File writes the specified file into the body stream in an efficient way.
func (ctx *RequestContext) File(filepath string) {
	ServeFile(ctx, filepath)
}

func (ctx *RequestContext) FileFromFS(filepath string, fs *FS) {
	defer func(old string) {
		ctx.Request.URI().SetPath(old)
	}(string(ctx.Request.URI().Path()))

	ctx.Request.URI().SetPath(filepath)

	fs.NewRequestHandler()(context.Background(), ctx)
}

// FileAttachment use an efficient way to write the file to body stream.
//
// When client download the file, it will rename the file as filename
func (ctx *RequestContext) FileAttachment(filepath, filename string) {
	ctx.Response.Header.Set("content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	ServeFile(ctx, filepath)
}

// SetBodyString sets response body to the given value.
func (ctx *RequestContext) SetBodyString(body string) {
	ctx.Response.SetBodyString(body)
}

// SetContentTypeBytes sets response Content-Type.
//
// It is safe modifying contentType buffer after function return.
func (ctx *RequestContext) SetContentTypeBytes(contentType []byte) {
	ctx.Response.Header.SetContentTypeBytes(contentType)
}

// FormFile returns the first file for the provided form key.
func (ctx *RequestContext) FormFile(name string) (*multipart.FileHeader, error) {
	return ctx.Request.FormFile(name)
}

// FormValue returns form value associated with the given key.
//
// The value is searched in the following places:
//
//   - Query string.
//   - POST or PUT body.
//
// There are more fine-grained methods for obtaining form values:
//
//   - QueryArgs for obtaining values from query string.
//   - PostArgs for obtaining values from POST or PUT body.
//   - MultipartForm for obtaining values from multipart form.
//   - FormFile for obtaining uploaded files.
//
// The returned value is valid until returning from RequestHandler.
// Use engine.SetCustomFormValueFunc to change action of FormValue.
func (ctx *RequestContext) FormValue(key string) []byte {
	if ctx.formValueFunc != nil {
		return ctx.formValueFunc(ctx, key)
	}
	return defaultFormValue(ctx, key)
}

func (ctx *RequestContext) multipartFormValue(key string) (string, bool) {
	mf, err := ctx.MultipartForm()
	if err == nil && mf.Value != nil {
		vv := mf.Value[key]
		if len(vv) > 0 {
			return vv[0], true
		}
	}
	return "", false
}

func (ctx *RequestContext) RequestBodyStream() io.Reader {
	return ctx.Request.BodyStream()
}

// MultipartForm returns request's multipart form.
//
// Returns errNoMultipartForm if request's content-type
// isn't 'multipart/form-data'.
//
// All uploaded temporary files are automatically deleted after
// returning from RequestHandler. Either move or copy uploaded files
// into new place if you want retaining them.
//
// Use SaveMultipartFile function for permanently saving uploaded file.
//
// The returned form is valid until returning from RequestHandler.
//
// See also FormFile and FormValue.
func (ctx *RequestContext) MultipartForm() (*multipart.Form, error) {
	return ctx.Request.MultipartForm()
}

// SaveUploadedFile uploads the form file to specific dst.
func (ctx *RequestContext) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// SetConnectionClose sets 'Connection: close' response header.
func (ctx *RequestContext) SetConnectionClose() {
	ctx.Response.SetConnectionClose()
}

// IsGet returns true if request method is GET.
func (ctx *RequestContext) IsGet() bool {
	return ctx.Request.Header.IsGet()
}

// IsHead returns true if request method is HEAD.
func (ctx *RequestContext) IsHead() bool {
	return ctx.Request.Header.IsHead()
}

// IsPost returns true if request method is POST.
func (ctx *RequestContext) IsPost() bool {
	return ctx.Request.Header.IsPost()
}

// Method return request method.
//
// Returned value is valid until returning from RequestHandler.
func (ctx *RequestContext) Method() []byte {
	return ctx.Request.Header.Method()
}

// NotFound resets response and sets '404 Not Found' response status code.
func (ctx *RequestContext) NotFound() {
	ctx.Response.Reset()
	ctx.SetStatusCode(consts.StatusNotFound)
	ctx.SetBodyString(consts.StatusMessage(consts.StatusNotFound))
}

func (ctx *RequestContext) redirect(uri []byte, statusCode int) {
	ctx.Response.Header.SetCanonical(bytestr.StrLocation, uri)
	statusCode = getRedirectStatusCode(statusCode)
	ctx.Response.SetStatusCode(statusCode)
}

func getRedirectStatusCode(statusCode int) int {
	if statusCode == consts.StatusMovedPermanently || statusCode == consts.StatusFound ||
		statusCode == consts.StatusSeeOther || statusCode == consts.StatusTemporaryRedirect ||
		statusCode == consts.StatusPermanentRedirect {
		return statusCode
	}
	return consts.StatusFound
}

// Copy returns a copy of the current context that can be safely used outside
// the request's scope.
//
// NOTE: If you want to pass requestContext to a goroutine, call this method
// to get a copy of requestContext.
func (ctx *RequestContext) Copy() *RequestContext {
	cp := &RequestContext{
		conn:   ctx.conn,
		Params: ctx.Params,
	}
	ctx.Request.CopyTo(&cp.Request)
	ctx.Response.CopyTo(&cp.Response)
	cp.index = rConsts.AbortIndex
	cp.handlers = nil
	cp.Keys = map[string]interface{}{}
	ctx.mu.RLock()
	for k, v := range ctx.Keys {
		cp.Keys[k] = v
	}
	ctx.mu.RUnlock()
	paramCopy := make([]param.Param, len(cp.Params))
	copy(paramCopy, cp.Params)
	cp.Params = paramCopy
	cp.fullPath = ctx.fullPath
	cp.clientIPFunc = ctx.clientIPFunc
	cp.formValueFunc = ctx.formValueFunc
	cp.binder = ctx.binder
	cp.validator = ctx.validator
	return cp
}

// Next should be used only inside middleware.
// It executes the pending handlers in the chain inside the calling handler.
func (ctx *RequestContext) Next(c context.Context) {
	ctx.index++
	for ctx.index < int8(len(ctx.handlers)) {
		ctx.handlers[ctx.index](c, ctx)
		ctx.index++
	}
}

// Handler returns the main handler.
func (ctx *RequestContext) Handler() HandlerFunc {
	return ctx.handlers.Last()
}

// Handlers returns the handler chain.
func (ctx *RequestContext) Handlers() HandlersChain {
	return ctx.handlers
}

func (ctx *RequestContext) SetHandlers(hc HandlersChain) {
	ctx.handlers = hc
}

// HandlerName returns the main handler's name.
//
// For example if the handler is "handleGetUsers()", this function will return "main.handleGetUsers".
func (ctx *RequestContext) HandlerName() string {
	return utils.NameOfFunction(ctx.handlers.Last())
}

func (ctx *RequestContext) ResetWithoutConn() {
	ctx.Params = ctx.Params[0:0]
	ctx.Errors = ctx.Errors[0:0]
	ctx.handlers = nil
	ctx.index = -1
	ctx.fullPath = ""
	ctx.Keys = nil

	if ctx.finished != nil {
		close(ctx.finished)
		ctx.finished = nil
	}

	ctx.Request.ResetWithoutConn()
	ctx.Response.Reset()
	if ctx.IsEnableTrace() {
		ctx.traceInfo.Reset()
	}
}

// Reset resets requestContext.
//
// NOTE: It is an internal function. You should not use it.
func (ctx *RequestContext) Reset() {
	ctx.ResetWithoutConn()
	ctx.conn = nil
}

// Redirect returns an HTTP redirect to the specific location.
// Note that this will not stop the current handler.
// In other words, even if Redirect() is called, the remaining handlers will still be executed and cause unexpected result.
// So it should call Abort to ensure the remaining handlers of this request will not be called.
//
//	ctx.Abort()
//	return
func (ctx *RequestContext) Redirect(statusCode int, uri []byte) {
	ctx.redirect(uri, statusCode)
}

// Header is an intelligent shortcut for ctx.Response.Header.Set(key, value).
// It writes a header in the response.
// If value == "", this method removes the header `ctx.Response.Header.Del(key)`.
func (ctx *RequestContext) Header(key, value string) {
	if value == "" {
		ctx.Response.Header.Del(key)
		return
	}
	ctx.Response.Header.Set(key, value)
}

// Set is used to store a new key/value pair exclusively for this context.
// It also lazy initializes  c.Keys if it was not used previously.
func (ctx *RequestContext) Set(key string, value interface{}) {
	ctx.mu.Lock()
	if ctx.Keys == nil {
		ctx.Keys = make(map[string]interface{})
	}

	ctx.Keys[key] = value
	ctx.mu.Unlock()
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exist it returns (nil, false)
func (ctx *RequestContext) Get(key string) (value interface{}, exists bool) {
	ctx.mu.RLock()
	value, exists = ctx.Keys[key]
	ctx.mu.RUnlock()
	return
}

// MustGet returns the value for the given key if it exists, otherwise it panics.
func (ctx *RequestContext) MustGet(key string) interface{} {
	if value, exists := ctx.Get(key); exists {
		return value
	}
	panic("Key \"" + key + "\" does not exist")
}

// GetString returns the value associated with the key as a string. Return "" when type is error.
func (ctx *RequestContext) GetString(key string) (s string) {
	if val, ok := ctx.Get(key); ok && val != nil {
		s, _ = val.(string)
	}
	return
}

// GetBool returns the value associated with the key as a boolean. Return false when type is error.
func (ctx *RequestContext) GetBool(key string) (b bool) {
	if val, ok := ctx.Get(key); ok && val != nil {
		b, _ = val.(bool)
	}
	return
}

// GetInt returns the value associated with the key as an integer. Return 0 when type is error.
func (ctx *RequestContext) GetInt(key string) (i int) {
	if val, ok := ctx.Get(key); ok && val != nil {
		i, _ = val.(int)
	}
	return
}

// GetInt32 returns the value associated with the key as an integer. Return int32(0) when type is error.
func (ctx *RequestContext) GetInt32(key string) (i32 int32) {
	if val, ok := ctx.Get(key); ok && val != nil {
		i32, _ = val.(int32)
	}
	return
}

// GetInt64 returns the value associated with the key as an integer. Return int64(0) when type is error.
func (ctx *RequestContext) GetInt64(key string) (i64 int64) {
	if val, ok := ctx.Get(key); ok && val != nil {
		i64, _ = val.(int64)
	}
	return
}

// GetUint returns the value associated with the key as an unsigned integer. Return uint(0) when type is error.
func (ctx *RequestContext) GetUint(key string) (ui uint) {
	if val, ok := ctx.Get(key); ok && val != nil {
		ui, _ = val.(uint)
	}
	return
}

// GetUint32 returns the value associated with the key as an unsigned integer. Return uint32(0) when type is error.
func (ctx *RequestContext) GetUint32(key string) (ui32 uint32) {
	if val, ok := ctx.Get(key); ok && val != nil {
		ui32, _ = val.(uint32)
	}
	return
}

// GetUint64 returns the value associated with the key as an unsigned integer. Return uint64(0) when type is error.
func (ctx *RequestContext) GetUint64(key string) (ui64 uint64) {
	if val, ok := ctx.Get(key); ok && val != nil {
		ui64, _ = val.(uint64)
	}
	return
}

// GetFloat32 returns the value associated with the key as a float32. Return float32(0.0) when type is error.
func (ctx *RequestContext) GetFloat32(key string) (f32 float32) {
	if val, ok := ctx.Get(key); ok && val != nil {
		f32, _ = val.(float32)
	}
	return
}

// GetFloat64 returns the value associated with the key as a float64. Return 0.0 when type is error.
func (ctx *RequestContext) GetFloat64(key string) (f64 float64) {
	if val, ok := ctx.Get(key); ok && val != nil {
		f64, _ = val.(float64)
	}
	return
}

// GetTime returns the value associated with the key as time. Return time.Time{} when type is error.
func (ctx *RequestContext) GetTime(key string) (t time.Time) {
	if val, ok := ctx.Get(key); ok && val != nil {
		t, _ = val.(time.Time)
	}
	return
}

// GetDuration returns the value associated with the key as a duration. Return time.Duration(0) when type is error.
func (ctx *RequestContext) GetDuration(key string) (d time.Duration) {
	if val, ok := ctx.Get(key); ok && val != nil {
		d, _ = val.(time.Duration)
	}
	return
}

// GetStringSlice returns the value associated with the key as a slice of strings.
//
// Return []string(nil) when type is error.
func (ctx *RequestContext) GetStringSlice(key string) (ss []string) {
	if val, ok := ctx.Get(key); ok && val != nil {
		ss, _ = val.([]string)
	}
	return
}

// GetStringMap returns the value associated with the key as a map of interfaces.
//
// Return map[string]interface{}(nil) when type is error.
func (ctx *RequestContext) GetStringMap(key string) (sm map[string]interface{}) {
	if val, ok := ctx.Get(key); ok && val != nil {
		sm, _ = val.(map[string]interface{})
	}
	return
}

// GetStringMapString returns the value associated with the key as a map of strings.
//
// Return map[string]string(nil) when type is error.
func (ctx *RequestContext) GetStringMapString(key string) (sms map[string]string) {
	if val, ok := ctx.Get(key); ok && val != nil {
		sms, _ = val.(map[string]string)
	}
	return
}

// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
//
// Return map[string][]string(nil) when type is error.
func (ctx *RequestContext) GetStringMapStringSlice(key string) (smss map[string][]string) {
	if val, ok := ctx.Get(key); ok && val != nil {
		smss, _ = val.(map[string][]string)
	}
	return
}

// Param returns the value of the URL param.
// It is a shortcut for c.Params.ByName(key)
//
//	router.GET("/user/:id", func(c context.Context, ctx *app.RequestContext) {
//	    // a GET request to /user/john
//	    id := ctx.Param("id") // id == "john"
//	})
func (ctx *RequestContext) Param(key string) string {
	return ctx.Params.ByName(key)
}

// Abort prevents pending handlers from being called.
//
// Note that this will not stop the current handler.
// Let's say you have an authorization middleware that validates that the current request is authorized.
// If the authorization fails (ex: the password does not match), call Abort to ensure the remaining handlers
// for this request are not called.
func (ctx *RequestContext) Abort() {
	ctx.index = rConsts.AbortIndex
}

// AbortWithStatus calls `Abort()` and writes the headers with the specified status code.
//
// For example, a failed attempt to authenticate a request could use: context.AbortWithStatus(401).
func (ctx *RequestContext) AbortWithStatus(code int) {
	ctx.SetStatusCode(code)
	ctx.Abort()
}

// AbortWithMsg sets response status code to the given value and sets response body
// to the given message.
//
// Warning: this will reset the response headers and body already set!
func (ctx *RequestContext) AbortWithMsg(msg string, statusCode int) {
	ctx.Response.Reset()
	ctx.SetStatusCode(statusCode)
	ctx.SetContentTypeBytes(bytestr.DefaultContentType)
	ctx.SetBodyString(msg)
	ctx.Abort()
}

// AbortWithStatusJSON calls `Abort()` and then `JSON` internally.
//
// This method stops the chain, writes the status code and return a JSON body.
// It also sets the Content-Type as "application/json".
func (ctx *RequestContext) AbortWithStatusJSON(code int, jsonObj interface{}) {
	ctx.Abort()
	ctx.JSON(code, jsonObj)
}

// Render writes the response headers and calls render.Render to render data.
func (ctx *RequestContext) Render(code int, r render.Render) {
	ctx.SetStatusCode(code)

	if !bodyAllowedForStatus(code) {
		r.WriteContentType(&ctx.Response)
		return
	}

	if err := r.Render(&ctx.Response); err != nil {
		panic(err)
	}
}

// ProtoBuf serializes the given struct as ProtoBuf into the response body.
func (ctx *RequestContext) ProtoBuf(code int, obj interface{}) {
	ctx.Render(code, render.ProtoBuf{Data: obj})
}

// JSON serializes the given struct as JSON into the response body.
//
// It also sets the Content-Type as "application/json".
func (ctx *RequestContext) JSON(code int, obj interface{}) {
	ctx.Render(code, render.JSONRender{Data: obj})
}

// PureJSON serializes the given struct as JSON into the response body.
// PureJSON, unlike JSON, does not replace special html characters with their unicode entities.
func (ctx *RequestContext) PureJSON(code int, obj interface{}) {
	ctx.Render(code, render.PureJSON{Data: obj})
}

// IndentedJSON serializes the given struct as pretty JSON (indented + endlines) into the response body.
// It also sets the Content-Type as "application/json".
func (ctx *RequestContext) IndentedJSON(code int, obj interface{}) {
	ctx.Render(code, render.IndentedJSON{Data: obj})
}

// HTML renders the HTTP template specified by its file name.
//
// It also updates the HTTP code and sets the Content-Type as "text/html".
// See http://golang.org/doc/articles/wiki/
func (ctx *RequestContext) HTML(code int, name string, obj interface{}) {
	instance := ctx.HTMLRender.Instance(name, obj)
	ctx.Render(code, instance)
}

// Data writes some data into the body stream and updates the HTTP code.
func (ctx *RequestContext) Data(code int, contentType string, data []byte) {
	ctx.Render(code, render.Data{
		ContentType: contentType,
		Data:        data,
	})
}

// XML serializes the given struct as XML into the response body.
//
// It also sets the Content-Type as "application/xml".
func (ctx *RequestContext) XML(code int, obj interface{}) {
	ctx.Render(code, render.XML{Data: obj})
}

// AbortWithError calls `AbortWithStatus()` and `Error()` internally.
//
// This method stops the chain, writes the status code and pushes the specified error to `c.Errors`.
// See RequestContext.Error() for more details.
func (ctx *RequestContext) AbortWithError(code int, err error) *errors.Error {
	ctx.AbortWithStatus(code)
	return ctx.Error(err)
}

// IsAborted returns true if the current context has aborted.
func (ctx *RequestContext) IsAborted() bool {
	return ctx.index >= rConsts.AbortIndex
}

// Error attaches an error to the current context. The error is pushed to a list of errors.
//
// It's a good idea to call Error for each error that occurred during the resolution of a request.
// A middleware can be used to collect all the errors and push them to a database together,
// print a log, or append it in the HTTP response.
// Error will panic if err is nil.
func (ctx *RequestContext) Error(err error) *errors.Error {
	if err == nil {
		panic("err is nil")
	}

	parsedError, ok := err.(*errors.Error)
	if !ok {
		parsedError = &errors.Error{
			Err:  err,
			Type: errors.ErrorTypePrivate,
		}
	}

	ctx.Errors = append(ctx.Errors, parsedError)
	return parsedError
}

// ContentType returns the Content-Type header of the request.
func (ctx *RequestContext) ContentType() []byte {
	return ctx.Request.Header.ContentType()
}

// Cookie returns the value of the request cookie key.
func (ctx *RequestContext) Cookie(key string) []byte {
	return ctx.Request.Header.Cookie(key)
}

// SetCookie adds a Set-Cookie header to the Response's headers.
//
//	Parameter introduce:
//	name and value is used to set cookie's name and value, eg. Set-Cookie: name=value
//	maxAge is use to set cookie's expiry date, eg. Set-Cookie: name=value; max-age=1
//	path and domain is used to set the scope of a cookie, eg. Set-Cookie: name=value;domain=localhost; path=/;
//	secure and httpOnly is used to sent cookies securely; eg. Set-Cookie: name=value;HttpOnly; secure;
//	sameSite let servers specify whether/when cookies are sent with cross-site requests; eg. Set-Cookie: name=value;HttpOnly; secure; SameSite=Lax;
//
//	For example:
//	1. ctx.SetCookie("user", "hertz", 1, "/", "localhost",protocol.CookieSameSiteLaxMode, true, true)
//	add response header --->  Set-Cookie: user=hertz; max-age=1; domain=localhost; path=/; HttpOnly; secure; SameSite=Lax;
//	2. ctx.SetCookie("user", "hertz", 10, "/", "localhost",protocol.CookieSameSiteLaxMode, false, false)
//	add response header --->  Set-Cookie: user=hertz; max-age=10; domain=localhost; path=/; SameSite=Lax;
//	3. ctx.SetCookie("", "hertz", 10, "/", "localhost",protocol.CookieSameSiteLaxMode, false, false)
//	add response header --->  Set-Cookie: hertz; max-age=10; domain=localhost; path=/; SameSite=Lax;
//	4. ctx.SetCookie("user", "", 10, "/", "localhost",protocol.CookieSameSiteLaxMode, false, false)
//	add response header --->  Set-Cookie: user=; max-age=10; domain=localhost; path=/; SameSite=Lax;
func (ctx *RequestContext) SetCookie(name, value string, maxAge int, path, domain string, sameSite protocol.CookieSameSite, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	cookie := protocol.AcquireCookie()
	defer protocol.ReleaseCookie(cookie)
	cookie.SetKey(name)
	cookie.SetValue(url.QueryEscape(value))
	cookie.SetMaxAge(maxAge)
	cookie.SetPath(path)
	cookie.SetDomain(domain)
	cookie.SetSecure(secure)
	cookie.SetHTTPOnly(httpOnly)
	cookie.SetSameSite(sameSite)
	ctx.Response.Header.SetCookie(cookie)
}

// UserAgent returns the value of the request user_agent.
func (ctx *RequestContext) UserAgent() []byte {
	return ctx.Request.Header.UserAgent()
}

// Status sets the HTTP response code.
func (ctx *RequestContext) Status(code int) {
	ctx.SetStatusCode(code)
}

// GetHeader returns value from request headers.
func (ctx *RequestContext) GetHeader(key string) []byte {
	return ctx.Request.Header.Peek(key)
}

// GetRawData returns body data.
func (ctx *RequestContext) GetRawData() []byte {
	return ctx.Request.Body()
}

// Body returns body data
func (ctx *RequestContext) Body() ([]byte, error) {
	return ctx.Request.BodyE()
}

// ClientIP tries to parse the headers in [X-Real-Ip, X-Forwarded-For].
// It calls RemoteIP() under the hood. If it cannot satisfy the requirements,
// use engine.SetClientIPFunc to inject your own implementation.
func (ctx *RequestContext) ClientIP() string {
	if ctx.clientIPFunc != nil {
		return ctx.clientIPFunc(ctx)
	}
	return defaultClientIP(ctx)
}

// QueryArgs returns query arguments from RequestURI.
//
// It doesn't return POST'ed arguments - use PostArgs() for this.
// Returned arguments are valid until returning from RequestHandler.
// See also PostArgs, FormValue and FormFile.
func (ctx *RequestContext) QueryArgs() *protocol.Args {
	return ctx.URI().QueryArgs()
}

// PostArgs returns POST arguments.
//
// It doesn't return query arguments from RequestURI - use QueryArgs for this.
// Returned arguments are valid until returning from RequestHandler.
// See also QueryArgs, FormValue and FormFile.
func (ctx *RequestContext) PostArgs() *protocol.Args {
	return ctx.Request.PostArgs()
}

// Query returns the keyed url query value if it exists, otherwise it returns an empty string `("")`.
//
// For example:
//
//	    GET /path?id=1234&name=Manu&value=
//		   c.Query("id") == "1234"
//		   c.Query("name") == "Manu"
//		   c.Query("value") == ""
//		   c.Query("wtf") == ""
func (ctx *RequestContext) Query(key string) string {
	value, _ := ctx.GetQuery(key)
	return value
}

// DefaultQuery returns the keyed url query value if it exists,
// otherwise it returns the specified defaultValue string.
func (ctx *RequestContext) DefaultQuery(key, defaultValue string) string {
	if value, ok := ctx.GetQuery(key); ok {
		return value
	}
	return defaultValue
}

// GetQuery returns the keyed url query value
//
// if it exists `(value, true)` (even when the value is an empty string) will be returned,
// otherwise it returns `("", false)`.
// For example:
//
//	GET /?name=Manu&lastname=
//	("Manu", true) == c.GetQuery("name")
//	("", false) == c.GetQuery("id")
//	("", true) == c.GetQuery("lastname")
func (ctx *RequestContext) GetQuery(key string) (string, bool) {
	return ctx.QueryArgs().PeekExists(key)
}

// PostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns an empty string `("")`.
func (ctx *RequestContext) PostForm(key string) string {
	value, _ := ctx.GetPostForm(key)
	return value
}

// DefaultPostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns the specified defaultValue string.
//
// See: PostForm() and GetPostForm() for further information.
func (ctx *RequestContext) DefaultPostForm(key, defaultValue string) string {
	if value, ok := ctx.GetPostForm(key); ok {
		return value
	}
	return defaultValue
}

// GetPostForm is like PostForm(key). It returns the specified key from a POST urlencoded
// form or multipart form when it exists `(value, true)` (even when the value is an empty string),
// otherwise it returns ("", false).
//
// For example, during a PATCH request to update the user's email:
//
//	    email=mail@example.com  -->  ("mail@example.com", true) := GetPostForm("email") // set email to "mail@example.com"
//		   email=                  -->  ("", true) := GetPostForm("email") // set email to ""
//	                            -->  ("", false) := GetPostForm("email") // do nothing with email
func (ctx *RequestContext) GetPostForm(key string) (string, bool) {
	if v, exists := ctx.PostArgs().PeekExists(key); exists {
		return v, exists
	}
	return ctx.multipartFormValue(key)
}

// bodyAllowedForStatus is a copy of http.bodyAllowedForStatus non-exported function.
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == consts.StatusNoContent:
		return false
	case status == consts.StatusNotModified:
		return false
	}
	return true
}

func (ctx *RequestContext) getBinder() binding.Binder {
	if ctx.binder != nil {
		return ctx.binder
	}
	return binding.DefaultBinder()
}

func (ctx *RequestContext) getValidator() binding.StructValidator {
	if ctx.validator != nil {
		return ctx.validator
	}
	return binding.DefaultValidator()
}

// BindAndValidate binds data from *RequestContext to obj and validates them if needed.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindAndValidate(obj interface{}) error {
	return ctx.getBinder().BindAndValidate(&ctx.Request, obj, ctx.Params)
}

// Bind binds data from *RequestContext to obj.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) Bind(obj interface{}) error {
	return ctx.getBinder().Bind(&ctx.Request, obj, ctx.Params)
}

// Validate validates obj with "vd" tag
// NOTE: obj should be a pointer.
func (ctx *RequestContext) Validate(obj interface{}) error {
	return ctx.getValidator().ValidateStruct(obj)
}

// BindQuery binds query parameters from *RequestContext to obj with 'query' tag. It will only use 'query' tag for binding.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindQuery(obj interface{}) error {
	return ctx.getBinder().BindQuery(&ctx.Request, obj)
}

// BindHeader binds header parameters from *RequestContext to obj with 'header' tag. It will only use 'header' tag for binding.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindHeader(obj interface{}) error {
	return ctx.getBinder().BindHeader(&ctx.Request, obj)
}

// BindPath binds router parameters from *RequestContext to obj with 'path' tag. It will only use 'path' tag for binding.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindPath(obj interface{}) error {
	return ctx.getBinder().BindPath(&ctx.Request, obj, ctx.Params)
}

// BindForm binds form parameters from *RequestContext to obj with 'form' tag. It will only use 'form' tag for binding.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindForm(obj interface{}) error {
	if len(ctx.Request.Body()) == 0 {
		return fmt.Errorf("missing form body")
	}
	return ctx.getBinder().BindForm(&ctx.Request, obj)
}

// BindJSON binds JSON body from *RequestContext.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindJSON(obj interface{}) error {
	return ctx.getBinder().BindJSON(&ctx.Request, obj)
}

// BindProtobuf binds protobuf body from *RequestContext.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindProtobuf(obj interface{}) error {
	return ctx.getBinder().BindProtobuf(&ctx.Request, obj)
}

// BindByContentType will select the binding type on the ContentType automatically.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindByContentType(obj interface{}) error {
	if ctx.Request.Header.IsGet() {
		return ctx.BindQuery(obj)
	}
	ct := utils.FilterContentType(bytesconv.B2s(ctx.Request.Header.ContentType()))
	switch strings.ToLower(ct) {
	case consts.MIMEApplicationJSON:
		return ctx.BindJSON(obj)
	case consts.MIMEPROTOBUF:
		return ctx.BindProtobuf(obj)
	case consts.MIMEApplicationHTMLForm, consts.MIMEMultipartPOSTForm:
		return ctx.BindForm(obj)
	default:
		return fmt.Errorf("unsupported bind content-type for '%s'", ct)
	}
}

// VisitAllQueryArgs calls f for each existing query arg.
//
// f must not retain references to key and value after returning.
// Make key and/or value copies if you need storing them after returning.
func (ctx *RequestContext) VisitAllQueryArgs(f func(key, value []byte)) {
	ctx.QueryArgs().VisitAll(f)
}

// VisitAllPostArgs calls f for each existing post arg.
//
// f must not retain references to key and value after returning.
// Make key and/or value copies if you need storing them after returning.
func (ctx *RequestContext) VisitAllPostArgs(f func(key, value []byte)) {
	ctx.Request.PostArgs().VisitAll(f)
}

// VisitAllHeaders calls f for each request header.
//
// f must not retain references to key and/or value after returning.
// Copy key and/or value contents before returning if you need retaining them.
//
// To get the headers in order they were received use VisitAllInOrder.
func (ctx *RequestContext) VisitAllHeaders(f func(key, value []byte)) {
	ctx.Request.Header.VisitAll(f)
}

// VisitAllCookie calls f for each request cookie.
//
// f must not retain references to key and/or value after returning.
func (ctx *RequestContext) VisitAllCookie(f func(key, value []byte)) {
	ctx.Request.Header.VisitAllCookie(f)
}
