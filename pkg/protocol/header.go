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

package protocol

import (
	"bytes"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/internal/nocopy"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	ServerDate     atomic.Value
	ServerDateOnce sync.Once // serverDateOnce.Do(updateServerDate)
)

type RequestHeader struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	disableNormalizing   bool
	connectionClose      bool
	noDefaultContentType bool

	// These two fields have been moved close to other bool fields
	// for reducing RequestHeader object size.
	cookiesCollected bool

	contentLength      int
	contentLengthBytes []byte

	method      []byte
	requestURI  []byte
	host        []byte
	contentType []byte

	userAgent []byte
	mulHeader [][]byte
	protocol  string

	h       []argsKV
	bufKV   argsKV
	trailer *Trailer

	cookies []argsKV

	// stores an immutable copy of headers as they were received from the
	// wire.
	rawHeaders []byte
}

func (h *RequestHeader) SetRawHeaders(r []byte) {
	h.rawHeaders = r
}

// ResponseHeader represents HTTP response header.
//
// It is forbidden copying ResponseHeader instances.
// Create new instances instead and use CopyTo.
//
// ResponseHeader instance MUST NOT be used from concurrently running
// goroutines.
type ResponseHeader struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	disableNormalizing   bool
	connectionClose      bool
	noDefaultContentType bool
	noDefaultDate        bool

	statusCode         int
	contentLength      int
	contentLengthBytes []byte
	contentEncoding    []byte

	contentType []byte
	server      []byte
	mulHeader   [][]byte
	protocol    string

	h       []argsKV
	bufKV   argsKV
	trailer *Trailer

	cookies []argsKV

	headerLength int
}

// SetHeaderLength sets the size of header for tracer.
func (h *ResponseHeader) SetHeaderLength(length int) {
	h.headerLength = length
}

// GetHeaderLength gets the size of header for tracer.
func (h *ResponseHeader) GetHeaderLength() int {
	return h.headerLength
}

// SetContentRange sets 'Content-Range: bytes startPos-endPos/contentLength'
// header.
func (h *ResponseHeader) SetContentRange(startPos, endPos, contentLength int) {
	b := h.bufKV.value[:0]
	b = append(b, bytestr.StrBytes...)
	b = append(b, ' ')
	b = bytesconv.AppendUint(b, startPos)
	b = append(b, '-')
	b = bytesconv.AppendUint(b, endPos)
	b = append(b, '/')
	b = bytesconv.AppendUint(b, contentLength)
	h.bufKV.value = b

	h.SetCanonical(bytestr.StrContentRange, h.bufKV.value)
}

func (h *ResponseHeader) NoDefaultContentType() bool {
	return h.noDefaultContentType
}

// SetConnectionClose sets 'Connection: close' header.
func (h *ResponseHeader) SetConnectionClose(close bool) {
	h.connectionClose = close
}

func (h *ResponseHeader) PeekArgBytes(key []byte) []byte {
	return peekArgBytes(h.h, key)
}

// Deprecated: Use ResponseHeader.SetProtocol(consts.HTTP11) instead
//
//	Now SetNoHTTP11(true) equal to SetProtocol(consts.HTTP10)
//		SetNoHTTP11(false) equal to SetProtocol(consts.HTTP11)
func (h *ResponseHeader) SetNoHTTP11(b bool) {
	if b {
		h.protocol = consts.HTTP10
		return
	}

	h.protocol = consts.HTTP11
}

// Cookie fills cookie for the given cookie.Key.
//
// Returns false if cookie with the given cookie.Key is missing.
func (h *ResponseHeader) Cookie(cookie *Cookie) bool {
	v := peekArgBytes(h.cookies, cookie.Key())
	if v == nil {
		return false
	}
	cookie.ParseBytes(v) //nolint:errcheck
	return true
}

// FullCookie returns complete cookie bytes
func (h *ResponseHeader) FullCookie() []byte {
	return h.Peek(consts.HeaderSetCookie)
}

// IsHTTP11 returns true if the response is HTTP/1.1.
func (h *ResponseHeader) IsHTTP11() bool {
	return h.protocol == consts.HTTP11
}

// SetContentType sets Content-Type header value.
func (h *ResponseHeader) SetContentType(contentType string) {
	h.contentType = append(h.contentType[:0], contentType...)
}

func (h *ResponseHeader) GetHeaders() []argsKV {
	return h.h
}

// Reset clears response header.
func (h *ResponseHeader) Reset() {
	h.disableNormalizing = false
	h.Trailer().disableNormalizing = false
	h.noDefaultContentType = false
	h.noDefaultDate = false
	h.ResetSkipNormalize()
}

// CopyTo copies all the headers to dst.
func (h *ResponseHeader) CopyTo(dst *ResponseHeader) {
	dst.Reset()

	dst.disableNormalizing = h.disableNormalizing
	dst.connectionClose = h.connectionClose
	dst.noDefaultContentType = h.noDefaultContentType
	dst.noDefaultDate = h.noDefaultDate

	dst.statusCode = h.statusCode
	dst.contentLength = h.contentLength
	dst.contentLengthBytes = append(dst.contentLengthBytes[:0], h.contentLengthBytes...)
	dst.contentEncoding = append(dst.contentEncoding[:0], h.contentEncoding...)
	dst.contentType = append(dst.contentType[:0], h.contentType...)
	dst.server = append(dst.server[:0], h.server...)
	dst.h = copyArgs(dst.h, h.h)
	dst.cookies = copyArgs(dst.cookies, h.cookies)
	dst.protocol = h.protocol
	dst.headerLength = h.headerLength
	h.Trailer().CopyTo(dst.Trailer())
}

// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
//
// the Content-Type, Content-Length, Connection, Cookie,
// Transfer-Encoding, Host and User-Agent headers can only be set once
// and will overwrite the previous value.
func (h *RequestHeader) Add(key, value string) {
	if h.setSpecialHeader(bytesconv.S2b(key), bytesconv.S2b(value)) {
		return
	}

	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.h = appendArg(h.h, bytesconv.B2s(k), value, ArgsHasValue)
}

// VisitAll calls f for each header.
//
// f must not retain references to key and/or value after returning.
// Copy key and/or value contents before returning if you need retaining them.
func (h *ResponseHeader) VisitAll(f func(key, value []byte)) {
	if len(h.contentLengthBytes) > 0 {
		f(bytestr.StrContentLength, h.contentLengthBytes)
	}
	contentType := h.ContentType()
	if len(contentType) > 0 {
		f(bytestr.StrContentType, contentType)
	}
	contentEncoding := h.ContentEncoding()
	if len(contentEncoding) > 0 {
		f(bytestr.StrContentEncoding, contentEncoding)
	}
	server := h.Server()
	if len(server) > 0 {
		f(bytestr.StrServer, server)
	}
	if len(h.cookies) > 0 {
		visitArgs(h.cookies, func(k, v []byte) {
			f(bytestr.StrSetCookie, v)
		})
	}
	if !h.Trailer().Empty() {
		f(bytestr.StrTrailer, h.Trailer().GetBytes())
	}
	visitArgs(h.h, f)
	if h.ConnectionClose() {
		f(bytestr.StrConnection, bytestr.StrClose)
	}
}

// IsHTTP11 returns true if the request is HTTP/1.1.
func (h *RequestHeader) IsHTTP11() bool {
	return h.protocol == consts.HTTP11
}

func (h *RequestHeader) SetProtocol(p string) {
	h.protocol = p
}

func (h *RequestHeader) GetProtocol() string {
	return h.protocol
}

// Deprecated: Use RequestHeader.SetProtocol(consts.HTTP11) instead
//
//	Now SetNoHTTP11(true) equal to SetProtocol(consts.HTTP10)
//		SetNoHTTP11(false) equal to SetProtocol(consts.HTTP11)
func (h *RequestHeader) SetNoHTTP11(b bool) {
	if b {
		h.protocol = consts.HTTP10
		return
	}

	h.protocol = consts.HTTP11
}

func (h *RequestHeader) InitBufValue(size int) {
	if size > cap(h.bufKV.value) {
		h.bufKV.value = make([]byte, 0, size)
	}
}

func (h *RequestHeader) GetBufValue() []byte {
	return h.bufKV.value
}

// HasAcceptEncodingBytes returns true if the header contains
// the given Accept-Encoding value.
func (h *RequestHeader) HasAcceptEncodingBytes(acceptEncoding []byte) bool {
	ae := h.peek(bytestr.StrAcceptEncoding)
	n := bytes.Index(ae, acceptEncoding)
	if n < 0 {
		return false
	}
	b := ae[n+len(acceptEncoding):]
	if len(b) > 0 && b[0] != ',' {
		return false
	}
	if n == 0 {
		return true
	}
	return ae[n-1] == ' '
}

func (h *RequestHeader) PeekIfModifiedSinceBytes() []byte {
	return h.peek(bytestr.StrIfModifiedSince)
}

// RequestURI returns RequestURI from the first HTTP request line.
func (h *RequestHeader) RequestURI() []byte {
	requestURI := h.requestURI
	if len(requestURI) == 0 {
		requestURI = bytestr.StrSlash
	}
	return requestURI
}

func (h *RequestHeader) PeekArgBytes(key []byte) []byte {
	return peekArgBytes(h.h, key)
}

// RawHeaders returns raw header key/value bytes.
//
// Depending on server configuration, header keys may be normalized to
// capital-case in place.
//
// This copy is set aside during parsing, so empty slice is returned for all
// cases where parsing did not happen. Similarly, request line is not stored
// during parsing and can not be returned.
//
// The slice is not safe to use after the handler returns.
func (h *RequestHeader) RawHeaders() []byte {
	return h.rawHeaders
}

// AppendBytes appends request header representation to dst and returns
// the extended dst.
func (h *RequestHeader) AppendBytes(dst []byte) []byte {
	dst = append(dst, h.Method()...)
	dst = append(dst, ' ')
	dst = append(dst, h.RequestURI()...)
	dst = append(dst, ' ')
	dst = append(dst, bytestr.StrHTTP11...)
	dst = append(dst, bytestr.StrCRLF...)

	userAgent := h.UserAgent()
	if len(userAgent) > 0 {
		dst = appendHeaderLine(dst, bytestr.StrUserAgent, userAgent)
	}

	host := h.Host()
	if len(host) > 0 {
		dst = appendHeaderLine(dst, bytestr.StrHost, host)
	}

	contentType := h.ContentType()
	if len(contentType) == 0 && !h.IgnoreBody() && !h.noDefaultContentType {
		contentType = bytestr.StrPostArgsContentType
	}
	if len(contentType) > 0 {
		dst = appendHeaderLine(dst, bytestr.StrContentType, contentType)
	}
	if len(h.contentLengthBytes) > 0 {
		dst = appendHeaderLine(dst, bytestr.StrContentLength, h.contentLengthBytes)
	}

	for i, n := 0, len(h.h); i < n; i++ {
		kv := &h.h[i]
		dst = appendHeaderLine(dst, kv.key, kv.value)
	}

	if !h.Trailer().Empty() {
		dst = appendHeaderLine(dst, bytestr.StrTrailer, h.Trailer().GetBytes())
	}

	// there is no need in h.collectCookies() here, since if cookies aren't collected yet,
	// they all are located in h.h.
	n := len(h.cookies)
	if n > 0 {
		dst = append(dst, bytestr.StrCookie...)
		dst = append(dst, bytestr.StrColonSpace...)
		dst = appendRequestCookieBytes(dst, h.cookies)
		dst = append(dst, bytestr.StrCRLF...)
	}

	if h.ConnectionClose() {
		dst = appendHeaderLine(dst, bytestr.StrConnection, bytestr.StrClose)
	}

	return append(dst, bytestr.StrCRLF...)
}

// Header returns request header representation.
//
// The returned representation is valid until the next call to RequestHeader methods.
func (h *RequestHeader) Header() []byte {
	h.bufKV.value = h.AppendBytes(h.bufKV.value[:0])
	return h.bufKV.value
}

// IsPut returns true if request method is PUT.
func (h *RequestHeader) IsPut() bool {
	return bytes.Equal(h.Method(), bytestr.StrPut)
}

// IsHead returns true if request method is HEAD.
func (h *RequestHeader) IsHead() bool {
	return bytes.Equal(h.Method(), bytestr.StrHead)
}

// IsPost returns true if request method is POST.
func (h *RequestHeader) IsPost() bool {
	return bytes.Equal(h.Method(), bytestr.StrPost)
}

// IsDelete returns true if request method is DELETE.
func (h *RequestHeader) IsDelete() bool {
	return bytes.Equal(h.Method(), bytestr.StrDelete)
}

// IsConnect returns true if request method is CONNECT.
func (h *RequestHeader) IsConnect() bool {
	return bytes.Equal(h.Method(), bytestr.StrConnect)
}

func (h *RequestHeader) IgnoreBody() bool {
	return h.IsGet() || h.IsHead()
}

// ContentLength returns Content-Length header value.
//
// It may be negative:
// -1 means Transfer-Encoding: chunked.
func (h *RequestHeader) ContentLength() int {
	return h.contentLength
}

// SetHost sets Host header value.
func (h *RequestHeader) SetHost(host string) {
	h.host = append(h.host[:0], host...)
}

// SetStatusCode sets response status code.
func (h *ResponseHeader) SetStatusCode(statusCode int) {
	checkWriteHeaderCode(statusCode)
	h.statusCode = statusCode
}

func checkWriteHeaderCode(code int) {
	// For now, we only emit a warning for bad codes.
	// In the future we might block things over 599 or under 100
	if code < 100 || code > 599 {
		hlog.SystemLogger().Warnf("Invalid StatusCode code %v, status code should not be under 100 or over 599.\n"+
			"For more info: https://www.rfc-editor.org/rfc/rfc9110.html#name-status-codes", code)
	}
}

func (h *ResponseHeader) ResetSkipNormalize() {
	h.protocol = ""
	h.connectionClose = false

	h.statusCode = 0
	h.contentLength = 0
	h.contentLengthBytes = h.contentLengthBytes[:0]
	h.contentEncoding = h.contentEncoding[:0]

	h.contentType = h.contentType[:0]
	h.server = h.server[:0]

	h.h = h.h[:0]
	h.cookies = h.cookies[:0]
	h.Trailer().ResetSkipNormalize()
	h.mulHeader = h.mulHeader[:0]
}

// ContentLength returns Content-Length header value.
//
// It may be negative:
// -1 means Transfer-Encoding: chunked.
// -2 means Transfer-Encoding: identity.
func (h *ResponseHeader) ContentLength() int {
	return h.contentLength
}

// Set sets the given 'key: value' header.
//
// Use Add for setting multiple header values under the same key.
func (h *ResponseHeader) Set(key, value string) {
	initHeaderKV(&h.bufKV, key, value, h.disableNormalizing)
	h.SetCanonical(h.bufKV.key, h.bufKV.value)
}

// Add adds the given 'key: value' header.
//
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
//
// the Content-Type, Content-Length, Connection, Server, Set-Cookie,
// Transfer-Encoding and Date headers can only be set once and will
// overwrite the previous value.
func (h *ResponseHeader) Add(key, value string) {
	if h.setSpecialHeader(bytesconv.S2b(key), bytesconv.S2b(value)) {
		return
	}

	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.h = appendArg(h.h, bytesconv.B2s(k), value, ArgsHasValue)
}

// SetContentLength sets Content-Length header value.
//
// Content-Length may be negative:
// -1 means Transfer-Encoding: chunked.
// -2 means Transfer-Encoding: identity.
func (h *ResponseHeader) SetContentLength(contentLength int) {
	if h.MustSkipContentLength() {
		return
	}
	h.contentLength = contentLength
	if contentLength >= 0 {
		h.contentLengthBytes = bytesconv.AppendUint(h.contentLengthBytes[:0], contentLength)
		h.h = delAllArgsBytes(h.h, bytestr.StrTransferEncoding)
	} else {
		h.contentLengthBytes = h.contentLengthBytes[:0]
		value := bytestr.StrChunked
		if contentLength == -2 {
			h.SetConnectionClose(true)
			value = bytestr.StrIdentity
		}
		h.h = setArgBytes(h.h, bytestr.StrTransferEncoding, value, ArgsHasValue)
	}
}

func (h *ResponseHeader) ContentLengthBytes() []byte {
	return h.contentLengthBytes
}

func (h *ResponseHeader) InitContentLengthWithValue(contentLength int) {
	h.contentLength = contentLength
}

// VisitAllCookie calls f for each response cookie.
//
// Cookie name is passed in key and the whole Set-Cookie header value
// is passed in value on each f invocation. Value may be parsed
// with Cookie.ParseBytes().
//
// f must not retain references to key and/or value after returning.
func (h *ResponseHeader) VisitAllCookie(f func(key, value []byte)) {
	visitArgs(h.cookies, f)
}

// DelAllCookies removes all the cookies from response headers.
func (h *ResponseHeader) DelAllCookies() {
	h.cookies = h.cookies[:0]
}

// DelCookie removes cookie under the given key from response header.
//
// Note that DelCookie doesn't remove the cookie from the client.
// Use DelClientCookie instead.
func (h *ResponseHeader) DelCookie(key string) {
	h.cookies = delAllArgs(h.cookies, key)
}

// DelCookieBytes removes cookie under the given key from response header.
//
// Note that DelCookieBytes doesn't remove the cookie from the client.
// Use DelClientCookieBytes instead.
func (h *ResponseHeader) DelCookieBytes(key []byte) {
	h.DelCookie(bytesconv.B2s(key))
}

// DelBytes deletes header with the given key.
func (h *ResponseHeader) DelBytes(key []byte) {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	utils.NormalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	h.del(h.bufKV.key)
}

// Header returns response header representation.
//
// The returned value is valid until the next call to ResponseHeader methods.
func (h *ResponseHeader) Header() []byte {
	h.bufKV.value = h.AppendBytes(h.bufKV.value[:0])
	return h.bufKV.value
}

func (h *ResponseHeader) PeekLocation() []byte {
	return h.peek(bytestr.StrLocation)
}

// DelClientCookie instructs the client to remove the given cookie.
// This doesn't work for a cookie with specific domain or path,
// you should delete it manually like:
//
//	c := AcquireCookie()
//	c.SetKey(key)
//	c.SetDomain("example.com")
//	c.SetPath("/path")
//	c.SetExpire(CookieExpireDelete)
//	h.SetCookie(c)
//	ReleaseCookie(c)
//
// Use DelCookie if you want just removing the cookie from response header.
func (h *ResponseHeader) DelClientCookie(key string) {
	h.DelCookie(key)

	c := AcquireCookie()
	c.SetKey(key)
	c.SetExpire(CookieExpireDelete)
	h.SetCookie(c)
	ReleaseCookie(c)
}

// DelClientCookieBytes instructs the client to remove the given cookie.
// This doesn't work for a cookie with specific domain or path,
// you should delete it manually like:
//
//	c := AcquireCookie()
//	c.SetKey(key)
//	c.SetDomain("example.com")
//	c.SetPath("/path")
//	c.SetExpire(CookieExpireDelete)
//	h.SetCookie(c)
//	ReleaseCookie(c)
//
// Use DelCookieBytes if you want just removing the cookie from response header.
func (h *ResponseHeader) DelClientCookieBytes(key []byte) {
	h.DelClientCookie(bytesconv.B2s(key))
}

// Peek returns header value for the given key.
//
// Returned value is valid until the next call to ResponseHeader.
// Do not store references to returned value. Make copies instead.
func (h *ResponseHeader) Peek(key string) []byte {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	return h.peek(k)
}

func (h *ResponseHeader) IsDisableNormalizing() bool {
	return h.disableNormalizing
}

func (h *ResponseHeader) ParseSetCookie(value []byte) {
	var kv *argsKV
	h.cookies, kv = allocArg(h.cookies)
	kv.key = getCookieKey(kv.key, value)
	kv.value = append(kv.value[:0], value...)
}

func (h *ResponseHeader) peek(key []byte) []byte {
	switch string(key) {
	case consts.HeaderContentType:
		return h.ContentType()
	case consts.HeaderContentEncoding:
		return h.ContentEncoding()
	case consts.HeaderServer:
		return h.Server()
	case consts.HeaderConnection:
		if h.ConnectionClose() {
			return bytestr.StrClose
		}
		return peekArgBytes(h.h, key)
	case consts.HeaderContentLength:
		return h.contentLengthBytes
	case consts.HeaderSetCookie:
		return appendResponseCookieBytes(nil, h.cookies)
	case consts.HeaderTrailer:
		return h.Trailer().GetBytes()
	default:
		return peekArgBytes(h.h, key)
	}
}

// PeekAll returns all header value for the given key.
//
// The returned value is valid until the request is released,
// either though ReleaseResponse or your request handler returning.
// Any future calls to the Peek* will modify the returned value.
// Do not store references to returned value. Use ResponseHeader.GetAll(key) instead.
func (h *ResponseHeader) PeekAll(key string) [][]byte {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	return h.peekAll(k)
}

func (h *ResponseHeader) peekAll(key []byte) [][]byte {
	h.mulHeader = h.mulHeader[:0]
	switch string(key) {
	case consts.HeaderContentType:
		if contentType := h.ContentType(); len(contentType) > 0 {
			h.mulHeader = append(h.mulHeader, contentType)
		}
	case consts.HeaderContentEncoding:
		if contentEncoding := h.ContentEncoding(); len(contentEncoding) > 0 {
			h.mulHeader = append(h.mulHeader, contentEncoding)
		}
	case consts.HeaderServer:
		if server := h.Server(); len(server) > 0 {
			h.mulHeader = append(h.mulHeader, server)
		}
	case consts.HeaderConnection:
		if h.ConnectionClose() {
			h.mulHeader = append(h.mulHeader, bytestr.StrClose)
		} else {
			h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
		}
	case consts.HeaderContentLength:
		h.mulHeader = append(h.mulHeader, h.contentLengthBytes)
	case consts.HeaderSetCookie:
		h.mulHeader = append(h.mulHeader, appendResponseCookieBytes(nil, h.cookies))
	default:
		h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
	}
	return h.mulHeader
}

// PeekAll returns all header value for the given key.
//
// The returned value is valid until the request is released,
// either though ReleaseRequest or your request handler returning.
// Any future calls to the Peek* will modify the returned value.
// Do not store references to returned value. Use RequestHeader.GetAll(key) instead.
func (h *RequestHeader) PeekAll(key string) [][]byte {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	return h.peekAll(k)
}

func (h *RequestHeader) peekAll(key []byte) [][]byte {
	h.mulHeader = h.mulHeader[:0]
	switch string(key) {
	case consts.HeaderHost:
		if host := h.Host(); len(host) > 0 {
			h.mulHeader = append(h.mulHeader, host)
		}
	case consts.HeaderContentType:
		if contentType := h.ContentType(); len(contentType) > 0 {
			h.mulHeader = append(h.mulHeader, contentType)
		}
	case consts.HeaderUserAgent:
		if ua := h.UserAgent(); len(ua) > 0 {
			h.mulHeader = append(h.mulHeader, ua)
		}
	case consts.HeaderConnection:
		if h.ConnectionClose() {
			h.mulHeader = append(h.mulHeader, bytestr.StrClose)
		} else {
			h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
		}
	case consts.HeaderContentLength:
		h.mulHeader = append(h.mulHeader, h.contentLengthBytes)
	case consts.HeaderCookie:
		if h.cookiesCollected {
			h.mulHeader = append(h.mulHeader, appendRequestCookieBytes(nil, h.cookies))
		} else {
			h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
		}
	default:
		h.mulHeader = peekAllArgBytesToDst(h.mulHeader, h.h, key)
	}
	return h.mulHeader
}

// SetContentTypeBytes sets Content-Type header value.
func (h *ResponseHeader) SetContentTypeBytes(contentType []byte) {
	h.contentType = append(h.contentType[:0], contentType...)
}

// ContentEncoding returns Content-Encoding header value.
func (h *ResponseHeader) ContentEncoding() []byte {
	return h.contentEncoding
}

// SetContentEncoding sets Content-Encoding header value.
func (h *ResponseHeader) SetContentEncoding(contentEncoding string) {
	h.contentEncoding = append(h.contentEncoding[:0], contentEncoding...)
}

// SetContentEncodingBytes sets Content-Encoding header value.
func (h *ResponseHeader) SetContentEncodingBytes(contentEncoding []byte) {
	h.contentEncoding = append(h.contentEncoding[:0], contentEncoding...)
}

func (h *ResponseHeader) SetContentLengthBytes(contentLength []byte) {
	h.contentLengthBytes = append(h.contentLengthBytes[:0], contentLength...)
}

// SetCanonical sets the given 'key: value' header assuming that
// key is in canonical form.
func (h *ResponseHeader) SetCanonical(key, value []byte) {
	if h.setSpecialHeader(key, value) {
		return
	}

	h.h = setArgBytes(h.h, key, value, ArgsHasValue)
}

// ResetConnectionClose clears 'Connection: close' header if it exists.
func (h *ResponseHeader) ResetConnectionClose() {
	if h.connectionClose {
		h.connectionClose = false
		h.h = delAllArgsBytes(h.h, bytestr.StrConnection)
	}
}

// Server returns Server header value.
func (h *ResponseHeader) Server() []byte {
	return h.server
}

func (h *ResponseHeader) AddArgBytes(key, value []byte, noValue bool) {
	h.h = appendArgBytes(h.h, key, value, noValue)
}

func (h *ResponseHeader) SetArgBytes(key, value []byte, noValue bool) {
	h.h = setArgBytes(h.h, key, value, noValue)
}

// AppendBytes appends response header representation to dst and returns
// the extended dst.
func (h *ResponseHeader) AppendBytes(dst []byte) []byte {
	statusCode := h.StatusCode()
	if statusCode < 0 {
		statusCode = consts.StatusOK
	}
	dst = append(dst, consts.StatusLine(statusCode)...)

	server := h.Server()
	if len(server) != 0 {
		dst = appendHeaderLine(dst, bytestr.StrServer, server)
	}

	if !h.noDefaultDate {
		ServerDateOnce.Do(UpdateServerDate)
		dst = appendHeaderLine(dst, bytestr.StrDate, ServerDate.Load().([]byte))
	}

	// Append Content-Type only for non-zero responses
	// or if it is explicitly set.
	if h.ContentLength() != 0 || len(h.contentType) > 0 {
		contentType := h.ContentType()
		if len(contentType) > 0 {
			dst = appendHeaderLine(dst, bytestr.StrContentType, contentType)
		}
	}
	contentEncoding := h.ContentEncoding()
	if len(contentEncoding) > 0 {
		dst = appendHeaderLine(dst, bytestr.StrContentEncoding, contentEncoding)
	}
	if len(h.contentLengthBytes) > 0 {
		dst = appendHeaderLine(dst, bytestr.StrContentLength, h.contentLengthBytes)
	}

	for i, n := 0, len(h.h); i < n; i++ {
		kv := &h.h[i]
		if h.noDefaultDate || !bytes.Equal(kv.key, bytestr.StrDate) {
			dst = appendHeaderLine(dst, kv.key, kv.value)
		}
	}

	if !h.Trailer().Empty() {
		dst = appendHeaderLine(dst, bytestr.StrTrailer, h.Trailer().GetBytes())
	}

	n := len(h.cookies)
	if n > 0 {
		for i := 0; i < n; i++ {
			kv := &h.cookies[i]
			dst = appendHeaderLine(dst, bytestr.StrSetCookie, kv.value)
		}
	}

	if h.ConnectionClose() {
		dst = appendHeaderLine(dst, bytestr.StrConnection, bytestr.StrClose)
	}

	return append(dst, bytestr.StrCRLF...)
}

// ConnectionClose returns true if 'Connection: close' header is set.
func (h *ResponseHeader) ConnectionClose() bool {
	return h.connectionClose
}

func (h *ResponseHeader) GetCookies() []argsKV {
	return h.cookies
}

// ContentType returns Content-Type header value.
func (h *ResponseHeader) ContentType() []byte {
	contentType := h.contentType
	if !h.noDefaultContentType && len(h.contentType) == 0 {
		contentType = bytestr.DefaultContentType
	}
	return contentType
}

// SetNoDefaultContentType set noDefaultContentType value of ResponseHeader.
func (h *ResponseHeader) SetNoDefaultContentType(b bool) {
	h.noDefaultContentType = b
}

// SetNoDefaultDate set noDefaultDate value of ResponseHeader.
func (h *ResponseHeader) SetNoDefaultDate(b bool) {
	h.noDefaultDate = b
}

// SetServerBytes sets Server header value.
func (h *ResponseHeader) SetServerBytes(server []byte) {
	h.server = append(h.server[:0], server...)
}

func (h *ResponseHeader) MustSkipContentLength() bool {
	// From http/1.1 specs:
	// All 1xx (informational), 204 (no content), and 304 (not modified) responses MUST NOT include a message-body
	statusCode := h.StatusCode()

	// Fast path.
	if statusCode < 100 || statusCode == consts.StatusOK {
		return false
	}

	// Slow path.
	return statusCode == consts.StatusNotModified || statusCode == consts.StatusNoContent || statusCode < 200
}

// StatusCode returns response status code.
func (h *ResponseHeader) StatusCode() int {
	if h.statusCode == 0 {
		return consts.StatusOK
	}
	return h.statusCode
}

// Del deletes header with the given key.
func (h *ResponseHeader) Del(key string) {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.del(k)
}

func (h *ResponseHeader) del(key []byte) {
	switch string(key) {
	case consts.HeaderContentType:
		h.contentType = h.contentType[:0]
	case consts.HeaderContentEncoding:
		h.contentEncoding = h.contentEncoding[:0]
	case consts.HeaderServer:
		h.server = h.server[:0]
	case consts.HeaderSetCookie:
		h.cookies = h.cookies[:0]
	case consts.HeaderContentLength:
		h.contentLength = 0
		h.contentLengthBytes = h.contentLengthBytes[:0]
	case consts.HeaderConnection:
		h.connectionClose = false
	case consts.HeaderTrailer:
		h.Trailer().ResetSkipNormalize()
	}
	h.h = delAllArgsBytes(h.h, key)
}

// SetBytesV sets the given 'key: value' header.
//
// Use AddBytesV for setting multiple header values under the same key.
func (h *ResponseHeader) SetBytesV(key string, value []byte) {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.SetCanonical(k, value)
}

// Len returns the number of headers set,
// i.e. the number of times f is called in VisitAll.
func (h *ResponseHeader) Len() int {
	n := 0
	h.VisitAll(func(k, v []byte) { n++ })
	return n
}

// Len returns the number of headers set,
// i.e. the number of times f is called in VisitAll.
func (h *RequestHeader) Len() int {
	n := 0
	h.VisitAll(func(k, v []byte) { n++ })
	return n
}

// Reset clears request header.
func (h *RequestHeader) Reset() {
	h.disableNormalizing = false
	h.Trailer().disableNormalizing = false
	h.ResetSkipNormalize()
}

// SetByteRange sets 'Range: bytes=startPos-endPos' header.
//
//   - If startPos is negative, then 'bytes=-startPos' value is set.
//   - If endPos is negative, then 'bytes=startPos-' value is set.
func (h *RequestHeader) SetByteRange(startPos, endPos int) {
	b := h.bufKV.value[:0]
	b = append(b, bytestr.StrBytes...)
	b = append(b, '=')
	if startPos >= 0 {
		b = bytesconv.AppendUint(b, startPos)
	} else {
		endPos = -startPos
	}
	b = append(b, '-')
	if endPos >= 0 {
		b = bytesconv.AppendUint(b, endPos)
	}
	h.bufKV.value = b

	h.SetCanonical(bytestr.StrRange, h.bufKV.value)
}

// DelBytes deletes header with the given key.
func (h *RequestHeader) DelBytes(key []byte) {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	utils.NormalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	h.del(h.bufKV.key)
}

// Del deletes header with the given key.
func (h *RequestHeader) Del(key string) {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	h.del(k)
}

func (h *RequestHeader) SetArgBytes(key, value []byte, noValue bool) {
	h.h = setArgBytes(h.h, key, value, noValue)
}

func (h *RequestHeader) del(key []byte) {
	switch string(key) {
	case consts.HeaderHost:
		h.host = h.host[:0]
	case consts.HeaderContentType:
		h.contentType = h.contentType[:0]
	case consts.HeaderUserAgent:
		h.userAgent = h.userAgent[:0]
	case consts.HeaderCookie:
		h.cookies = h.cookies[:0]
	case consts.HeaderContentLength:
		h.contentLength = 0
		h.contentLengthBytes = h.contentLengthBytes[:0]
	case consts.HeaderConnection:
		h.connectionClose = false
	case consts.HeaderTrailer:
		h.Trailer().ResetSkipNormalize()
	}
	h.h = delAllArgsBytes(h.h, key)
}

// CopyTo copies all the headers to dst.
func (h *RequestHeader) CopyTo(dst *RequestHeader) {
	dst.Reset()

	dst.disableNormalizing = h.disableNormalizing
	dst.connectionClose = h.connectionClose
	dst.noDefaultContentType = h.noDefaultContentType

	dst.contentLength = h.contentLength
	dst.contentLengthBytes = append(dst.contentLengthBytes[:0], h.contentLengthBytes...)
	dst.method = append(dst.method[:0], h.method...)
	dst.requestURI = append(dst.requestURI[:0], h.requestURI...)
	dst.host = append(dst.host[:0], h.host...)
	dst.contentType = append(dst.contentType[:0], h.contentType...)
	dst.userAgent = append(dst.userAgent[:0], h.userAgent...)
	h.Trailer().CopyTo(dst.Trailer())
	dst.h = copyArgs(dst.h, h.h)
	dst.cookies = copyArgs(dst.cookies, h.cookies)
	dst.cookiesCollected = h.cookiesCollected
	dst.rawHeaders = append(dst.rawHeaders[:0], h.rawHeaders...)
	dst.protocol = h.protocol
}

// Peek returns header value for the given key.
//
// Returned value is valid until the next call to RequestHeader.
// Do not store references to returned value. Make copies instead.
func (h *RequestHeader) Peek(key string) []byte {
	k := getHeaderKeyBytes(&h.bufKV, key, h.disableNormalizing)
	return h.peek(k)
}

// SetMultipartFormBoundary sets the following Content-Type:
// 'multipart/form-data; boundary=...'
// where ... is substituted by the given boundary.
func (h *RequestHeader) SetMultipartFormBoundary(boundary string) {
	b := h.bufKV.value[:0]
	b = append(b, bytestr.StrMultipartFormData...)
	b = append(b, ';', ' ')
	b = append(b, bytestr.StrBoundary...)
	b = append(b, '=')
	b = append(b, boundary...)
	h.bufKV.value = b

	h.SetContentTypeBytes(h.bufKV.value)
}

func (h *RequestHeader) ContentLengthBytes() []byte {
	return h.contentLengthBytes
}

func (h *RequestHeader) SetContentLengthBytes(contentLength []byte) {
	h.contentLengthBytes = append(h.contentLengthBytes[:0], contentLength...)
}

// SetContentTypeBytes sets Content-Type header value.
func (h *RequestHeader) SetContentTypeBytes(contentType []byte) {
	h.contentType = append(h.contentType[:0], contentType...)
}

// ContentType returns Content-Type header value.
func (h *RequestHeader) ContentType() []byte {
	return h.contentType
}

// SetNoDefaultContentType controls the default Content-Type header behaviour.
//
// When set to false, the Content-Type header is sent with a default value if no Content-Type value is specified.
// When set to true, no Content-Type header is sent if no Content-Type value is specified.
func (h *RequestHeader) SetNoDefaultContentType(b bool) {
	h.noDefaultContentType = b
}

// SetContentLength sets Content-Length header value.
//
// Negative content-length sets 'Transfer-Encoding: chunked' header.
func (h *RequestHeader) SetContentLength(contentLength int) {
	h.contentLength = contentLength
	if contentLength >= 0 {
		h.contentLengthBytes = bytesconv.AppendUint(h.contentLengthBytes[:0], contentLength)
		h.h = delAllArgsBytes(h.h, bytestr.StrTransferEncoding)
	} else {
		h.contentLengthBytes = h.contentLengthBytes[:0]
		h.h = setArgBytes(h.h, bytestr.StrTransferEncoding, bytestr.StrChunked, ArgsHasValue)
	}
}

func (h *RequestHeader) InitContentLengthWithValue(contentLength int) {
	h.contentLength = contentLength
}

// MultipartFormBoundary returns boundary part
// from 'multipart/form-data; boundary=...' Content-Type.
func (h *RequestHeader) MultipartFormBoundary() []byte {
	b := h.ContentType()
	if !bytes.HasPrefix(b, bytestr.StrMultipartFormData) {
		return nil
	}
	b = b[len(bytestr.StrMultipartFormData):]
	if len(b) == 0 || b[0] != ';' {
		return nil
	}

	var n int
	for len(b) > 0 {
		n++
		for len(b) > n && b[n] == ' ' {
			n++
		}
		b = b[n:]
		if !bytes.HasPrefix(b, bytestr.StrBoundary) {
			if n = bytes.IndexByte(b, ';'); n < 0 {
				return nil
			}
			continue
		}

		b = b[len(bytestr.StrBoundary):]
		if len(b) == 0 || b[0] != '=' {
			return nil
		}
		b = b[1:]
		if n = bytes.IndexByte(b, ';'); n >= 0 {
			b = b[:n]
		}
		if len(b) > 1 && b[0] == '"' && b[len(b)-1] == '"' {
			b = b[1 : len(b)-1]
		}
		return b
	}
	return nil
}

// ConnectionClose returns true if 'Connection: close' header is set.
func (h *RequestHeader) ConnectionClose() bool {
	return h.connectionClose
}

// Method returns HTTP request method.
func (h *RequestHeader) Method() []byte {
	if len(h.method) == 0 {
		return bytestr.StrGet
	}
	return h.method
}

// IsGet returns true if request method is GET.
func (h *RequestHeader) IsGet() bool {
	return bytes.Equal(h.Method(), bytestr.StrGet)
}

// IsOptions returns true if request method is Options.
func (h *RequestHeader) IsOptions() bool {
	return bytes.Equal(h.Method(), bytestr.StrOptions)
}

// IsTrace returns true if request method is Trace.
func (h *RequestHeader) IsTrace() bool {
	return bytes.Equal(h.Method(), bytestr.StrTrace)
}

// SetHostBytes sets Host header value.
func (h *RequestHeader) SetHostBytes(host []byte) {
	h.host = append(h.host[:0], host...)
}

// SetRequestURIBytes sets RequestURI for the first HTTP request line.
// RequestURI must be properly encoded.
// Use URI.RequestURI for constructing proper RequestURI if unsure.
func (h *RequestHeader) SetRequestURIBytes(requestURI []byte) {
	h.requestURI = append(h.requestURI[:0], requestURI...)
}

// SetBytesKV sets the given 'key: value' header.
//
// Use AddBytesKV for setting multiple header values under the same key.
func (h *RequestHeader) SetBytesKV(key, value []byte) {
	h.bufKV.key = append(h.bufKV.key[:0], key...)
	utils.NormalizeHeaderKey(h.bufKV.key, h.disableNormalizing)
	h.SetCanonical(h.bufKV.key, value)
}

func (h *RequestHeader) AddArgBytes(key, value []byte, noValue bool) {
	h.h = appendArgBytes(h.h, key, value, noValue)
}

// SetUserAgentBytes sets User-Agent header value.
func (h *RequestHeader) SetUserAgentBytes(userAgent []byte) {
	h.userAgent = append(h.userAgent[:0], userAgent...)
}

// SetCookie sets 'key: value' cookies.
func (h *RequestHeader) SetCookie(key, value string) {
	h.collectCookies()
	h.cookies = setArg(h.cookies, key, value, ArgsHasValue)
}

// SetCookie sets the given response cookie.
// It is save re-using the cookie after the function returns.
func (h *ResponseHeader) SetCookie(cookie *Cookie) {
	h.cookies = setArgBytes(h.cookies, cookie.Key(), cookie.Cookie(), ArgsHasValue)
}

// Cookie returns cookie for the given key.
func (h *RequestHeader) Cookie(key string) []byte {
	h.collectCookies()
	return peekArgStr(h.cookies, key)
}

// Cookies returns all the request cookies.
//
// It's a good idea to call protocol.ReleaseCookie to reduce GC load after the cookie used.
func (h *RequestHeader) Cookies() []*Cookie {
	var cookies []*Cookie
	h.VisitAllCookie(func(key, value []byte) {
		cookie := AcquireCookie()
		cookie.SetKeyBytes(key)
		cookie.SetValueBytes(value)
		cookies = append(cookies, cookie)
	})
	return cookies
}

func (h *RequestHeader) PeekRange() []byte {
	return h.peek(bytestr.StrRange)
}

func (h *RequestHeader) PeekContentEncoding() []byte {
	return h.peek(bytestr.StrContentEncoding)
}

// FullCookie returns complete cookie bytes
func (h *RequestHeader) FullCookie() []byte {
	return h.Peek(consts.HeaderCookie)
}

// DelCookie removes cookie under the given key.
func (h *RequestHeader) DelCookie(key string) {
	h.collectCookies()
	h.cookies = delAllArgs(h.cookies, key)
}

// DelAllCookies removes all the cookies from request headers.
func (h *RequestHeader) DelAllCookies() {
	h.collectCookies()
	h.cookies = h.cookies[:0]
}

// VisitAllCookie calls f for each request cookie.
//
// f must not retain references to key and/or value after returning.
func (h *RequestHeader) VisitAllCookie(f func(key, value []byte)) {
	h.collectCookies()
	visitArgs(h.cookies, f)
}

func (h *RequestHeader) collectCookies() {
	if h.cookiesCollected {
		return
	}

	for i, n := 0, len(h.h); i < n; i++ {
		kv := &h.h[i]
		if bytes.Equal(kv.key, bytestr.StrCookie) {
			h.cookies = parseRequestCookies(h.cookies, kv.value)
			tmp := *kv
			copy(h.h[i:], h.h[i+1:])
			n--
			i--
			h.h[n] = tmp
			h.h = h.h[:n]
		}
	}
	h.cookiesCollected = true
}

func (h *RequestHeader) SetConnectionClose(close bool) {
	h.connectionClose = close
}

// ResetConnectionClose clears 'Connection: close' header if it exists.
func (h *RequestHeader) ResetConnectionClose() {
	if h.connectionClose {
		h.connectionClose = false
		h.h = delAllArgsBytes(h.h, bytestr.StrConnection)
	}
}

// SetMethod sets HTTP request method.
func (h *RequestHeader) SetMethod(method string) {
	h.method = append(h.method[:0], method...)
}

// SetRequestURI sets RequestURI for the first HTTP request line.
// RequestURI must be properly encoded.
// Use URI.RequestURI for constructing proper RequestURI if unsure.
func (h *RequestHeader) SetRequestURI(requestURI string) {
	h.requestURI = append(h.requestURI[:0], requestURI...)
}

// Set sets the given 'key: value' header.
//
// Use Add for setting multiple header values under the same key.
func (h *RequestHeader) Set(key, value string) {
	initHeaderKV(&h.bufKV, key, value, h.disableNormalizing)
	h.SetCanonical(h.bufKV.key, h.bufKV.value)
}

func initHeaderKV(kv *argsKV, key, value string, disableNormalizing bool) {
	kv.key = getHeaderKeyBytes(kv, key, disableNormalizing)
	kv.value = append(kv.value[:0], value...)
}

// SetCanonical sets the given 'key: value' header assuming that
// key is in canonical form.
func (h *RequestHeader) SetCanonical(key, value []byte) {
	if h.setSpecialHeader(key, value) {
		return
	}

	h.h = setArgBytes(h.h, key, value, ArgsHasValue)
}

func (h *RequestHeader) ResetSkipNormalize() {
	h.connectionClose = false
	h.protocol = ""
	h.noDefaultContentType = false

	h.contentLength = 0
	h.contentLengthBytes = h.contentLengthBytes[:0]

	h.method = h.method[:0]
	h.requestURI = h.requestURI[:0]
	h.host = h.host[:0]
	h.contentType = h.contentType[:0]
	h.userAgent = h.userAgent[:0]

	h.h = h.h[:0]
	h.cookies = h.cookies[:0]
	h.cookiesCollected = false

	h.rawHeaders = h.rawHeaders[:0]
	h.mulHeader = h.mulHeader[:0]
	h.Trailer().ResetSkipNormalize()
}

func peekRawHeader(buf, key []byte) []byte {
	n := bytes.Index(buf, key)
	if n < 0 {
		return nil
	}
	if n > 0 && buf[n-1] != '\n' {
		return nil
	}
	n += len(key)
	if n >= len(buf) {
		return nil
	}
	if buf[n] != ':' {
		return nil
	}
	n++
	if buf[n] != ' ' {
		return nil
	}
	n++
	buf = buf[n:]
	n = bytes.IndexByte(buf, '\n')
	if n < 0 {
		return nil
	}
	if n > 0 && buf[n-1] == '\r' {
		n--
	}
	return buf[:n]
}

// Host returns Host header value.
func (h *RequestHeader) Host() []byte {
	return h.host
}

// UserAgent returns User-Agent header value.
func (h *RequestHeader) UserAgent() []byte {
	return h.userAgent
}

// DisableNormalizing disables header names' normalization.
//
// By default all the header names are normalized by uppercasing
// the first letter and all the first letters following dashes,
// while lowercasing all the other letters.
// Examples:
//
//   - CONNECTION -> Connection
//   - conteNT-tYPE -> Content-Type
//   - foo-bar-baz -> Foo-Bar-Baz
//
// Disable header names' normalization only if you know what are you doing.
func (h *RequestHeader) DisableNormalizing() {
	h.disableNormalizing = true
	h.Trailer().DisableNormalizing()
}

func (h *RequestHeader) IsDisableNormalizing() bool {
	return h.disableNormalizing
}

// String returns request header representation.
func (h *RequestHeader) String() string {
	return string(h.Header())
}

// VisitAll calls f for each header.
//
// f must not retain references to key and/or value after returning.
// Copy key and/or value contents before returning if you need retaining them.
//
// To get the headers in order they were received use VisitAllInOrder.
func (h *RequestHeader) VisitAll(f func(key, value []byte)) {
	host := h.Host()
	if len(host) > 0 {
		f(bytestr.StrHost, host)
	}
	if len(h.contentLengthBytes) > 0 {
		f(bytestr.StrContentLength, h.contentLengthBytes)
	}
	contentType := h.ContentType()
	if len(contentType) > 0 {
		f(bytestr.StrContentType, contentType)
	}
	userAgent := h.UserAgent()
	if len(userAgent) > 0 {
		f(bytestr.StrUserAgent, userAgent)
	}
	if !h.Trailer().Empty() {
		f(bytestr.StrTrailer, h.Trailer().GetBytes())
	}

	h.collectCookies()
	if len(h.cookies) > 0 {
		h.bufKV.value = appendRequestCookieBytes(h.bufKV.value[:0], h.cookies)
		f(bytestr.StrCookie, h.bufKV.value)
	}
	visitArgs(h.h, f)
	if h.ConnectionClose() {
		f(bytestr.StrConnection, bytestr.StrClose)
	}
}

// VisitAllCustomHeader calls f for each header in header.h which contains all headers
// except cookie, host, content-length, content-type, user-agent and connection.
//
// f must not retain references to key and/or value after returning.
// Copy key and/or value contents before returning if you need retaining them.
//
// To get the headers in order they were received use VisitAllInOrder.
func (h *RequestHeader) VisitAllCustomHeader(f func(key, value []byte)) {
	visitArgs(h.h, f)
}

func ParseContentLength(b []byte) (int, error) {
	v, n, err := bytesconv.ParseUintBuf(b)
	if err != nil {
		return -1, err
	}
	if n != len(b) {
		return -1, errs.NewPublic("non-numeric chars at the end of Content-Length")
	}
	return v, nil
}

func appendArgBytes(args []argsKV, key, value []byte, noValue bool) []argsKV {
	var kv *argsKV
	args, kv = allocArg(args)
	kv.key = append(kv.key[:0], key...)
	if noValue {
		kv.value = kv.value[:0]
	} else {
		kv.value = append(kv.value[:0], value...)
	}
	kv.noValue = noValue
	return args
}

func appendArg(args []argsKV, key, value string, noValue bool) []argsKV {
	var kv *argsKV
	args, kv = allocArg(args)
	kv.key = append(kv.key[:0], key...)
	if noValue {
		kv.value = kv.value[:0]
	} else {
		kv.value = append(kv.value[:0], value...)
	}
	kv.noValue = noValue
	return args
}

func (h *RequestHeader) peek(key []byte) []byte {
	switch string(key) {
	case consts.HeaderHost:
		return h.Host()
	case consts.HeaderContentType:
		return h.ContentType()
	case consts.HeaderUserAgent:
		return h.UserAgent()
	case consts.HeaderConnection:
		if h.ConnectionClose() {
			return bytestr.StrClose
		}
		return peekArgBytes(h.h, key)
	case consts.HeaderContentLength:
		return h.contentLengthBytes
	case consts.HeaderCookie:
		if h.cookiesCollected {
			return appendRequestCookieBytes(nil, h.cookies)
		}
		return peekArgBytes(h.h, key)
	case consts.HeaderTrailer:
		return h.Trailer().GetBytes()
	default:
		return peekArgBytes(h.h, key)
	}
}

func (h *RequestHeader) Get(key string) string {
	return string(h.Peek(key))
}

func (h *ResponseHeader) Get(key string) string {
	return string(h.Peek(key))
}

// GetAll returns all header value for the given key
// it is concurrent safety and long lifetime.
func (h *RequestHeader) GetAll(key string) []string {
	res := make([]string, 0)
	headers := h.PeekAll(key)
	for _, header := range headers {
		res = append(res, string(header))
	}
	return res
}

// GetAll returns all header value for the given key and is concurrent safety.
// it is concurrent safety and long lifetime.
func (h *ResponseHeader) GetAll(key string) []string {
	res := make([]string, 0)
	headers := h.PeekAll(key)
	for _, header := range headers {
		res = append(res, string(header))
	}
	return res
}

func appendHeaderLine(dst, key, value []byte) []byte {
	for _, k := range key {
		// if header field contains invalid key, just skip it.
		if bytesconv.ValidHeaderFieldNameTable[k] == 0 {
			return dst
		}
	}
	dst = append(dst, key...)
	dst = append(dst, bytestr.StrColonSpace...)
	dst = append(dst, newlineToSpace(value)...)
	return append(dst, bytestr.StrCRLF...)
}

// newlineToSpace will return a copy of the original byte slice.
func newlineToSpace(val []byte) []byte {
	filteredVal := make([]byte, len(val))
	copy(filteredVal, val)
	for i := 0; i < len(filteredVal); i++ {
		filteredVal[i] = bytesconv.NewlineToSpaceTable[filteredVal[i]]
	}
	return filteredVal
}

func UpdateServerDate() {
	refreshServerDate()
	go func() {
		for {
			time.Sleep(time.Second)
			refreshServerDate()
		}
	}()
}

func refreshServerDate() {
	b := bytesconv.AppendHTTPDate(make([]byte, 0, len(http.TimeFormat)), time.Now())
	ServerDate.Store(b)
}

func getHeaderKeyBytes(kv *argsKV, key string, disableNormalizing bool) []byte {
	kv.key = append(kv.key[:0], key...)
	utils.NormalizeHeaderKey(kv.key, disableNormalizing)
	return kv.key
}

// SetMethodBytes sets HTTP request method.
func (h *RequestHeader) SetMethodBytes(method []byte) {
	h.method = append(h.method[:0], method...)
}

// DisableNormalizing disables header names' normalization.
//
// By default all the header names are normalized by uppercasing
// the first letter and all the first letters following dashes,
// while lowercasing all the other letters.
// Examples:
//
//   - CONNECTION -> Connection
//   - conteNT-tYPE -> Content-Type
//   - foo-bar-baz -> Foo-Bar-Baz
//
// Disable header names' normalization only if you know what are you doing.
func (h *ResponseHeader) DisableNormalizing() {
	h.disableNormalizing = true
	h.Trailer().DisableNormalizing()
}

// setSpecialHeader handles special headers and return true when a header is processed.
func (h *ResponseHeader) setSpecialHeader(key, value []byte) bool {
	if len(key) == 0 {
		return false
	}

	switch key[0] | 0x20 {
	case 'c':
		if utils.CaseInsensitiveCompare(bytestr.StrContentType, key) {
			h.SetContentTypeBytes(value)
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrContentLength, key) {
			if contentLength, err := ParseContentLength(value); err == nil {
				h.contentLength = contentLength
				h.contentLengthBytes = append(h.contentLengthBytes[:0], value...)
			}
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrContentEncoding, key) {
			h.SetContentEncodingBytes(value)
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrConnection, key) {
			if bytes.Equal(bytestr.StrClose, value) {
				h.SetConnectionClose(true)
			} else {
				h.ResetConnectionClose()
				h.h = setArgBytes(h.h, key, value, ArgsHasValue)
			}
			return true
		}
	case 's':
		if utils.CaseInsensitiveCompare(bytestr.StrServer, key) {
			h.SetServerBytes(value)
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrSetCookie, key) {
			var kv *argsKV
			h.cookies, kv = allocArg(h.cookies)
			kv.key = getCookieKey(kv.key, value)
			kv.value = append(kv.value[:0], value...)
			return true
		}
	case 't':
		if utils.CaseInsensitiveCompare(bytestr.StrTransferEncoding, key) {
			// Transfer-Encoding is managed automatically.
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrTrailer, key) {
			h.Trailer().SetTrailers(value)
			return true
		}
	case 'd':
		if utils.CaseInsensitiveCompare(bytestr.StrDate, key) {
			// Date is managed automatically.
			return true
		}
	}

	return false
}

// setSpecialHeader handles special headers and return true when a header is processed.
func (h *RequestHeader) setSpecialHeader(key, value []byte) bool {
	if len(key) == 0 {
		return false
	}

	switch key[0] | 0x20 {
	case 'c':
		if utils.CaseInsensitiveCompare(bytestr.StrContentType, key) {
			h.SetContentTypeBytes(value)
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrContentLength, key) {
			if contentLength, err := ParseContentLength(value); err == nil {
				h.contentLength = contentLength
				h.contentLengthBytes = append(h.contentLengthBytes[:0], value...)
			}
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrConnection, key) {
			if bytes.Equal(bytestr.StrClose, value) {
				h.SetConnectionClose(true)
			} else {
				h.ResetConnectionClose()
				h.h = setArgBytes(h.h, key, value, ArgsHasValue)
			}
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrCookie, key) {
			h.collectCookies()
			h.cookies = parseRequestCookies(h.cookies, value)
			return true
		}
	case 't':
		if utils.CaseInsensitiveCompare(bytestr.StrTransferEncoding, key) {
			// Transfer-Encoding is managed automatically.
			return true
		} else if utils.CaseInsensitiveCompare(bytestr.StrTrailer, key) {
			// copy value to avoid panic
			value = append(h.bufKV.value[:0], value...)
			h.Trailer().SetTrailers(value)
			return true
		}
	case 'h':
		if utils.CaseInsensitiveCompare(bytestr.StrHost, key) {
			h.SetHostBytes(value)
			return true
		}
	case 'u':
		if utils.CaseInsensitiveCompare(bytestr.StrUserAgent, key) {
			h.SetUserAgentBytes(value)
			return true
		}
	}

	return false
}

// Trailer returns the Trailer of HTTP Header.
func (h *ResponseHeader) Trailer() *Trailer {
	if h.trailer == nil {
		h.trailer = new(Trailer)
	}
	return h.trailer
}

// Trailer returns the Trailer of HTTP Header.
func (h *RequestHeader) Trailer() *Trailer {
	if h.trailer == nil {
		h.trailer = new(Trailer)
	}
	return h.trailer
}

func (h *ResponseHeader) SetProtocol(p string) {
	h.protocol = p
}

func (h *ResponseHeader) GetProtocol() string {
	return h.protocol
}
