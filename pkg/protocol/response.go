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
	"io"
	"net"
	"sync"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/nocopy"
	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/compress"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
)

var (
	responsePool sync.Pool
	// NoResponseBody is an io.ReadCloser with no bytes. Read always returns EOF
	// and Close always returns nil. It can be used in an ingoing client
	// response to explicitly signal that a response has zero bytes.
	NoResponseBody = noBody{}
)

// Response represents HTTP response.
//
// It is forbidden copying Response instances. Create new instances
// and use CopyTo instead.
//
// Response instance MUST NOT be used from concurrently running goroutines.
type Response struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	// Response header
	//
	// Copying Header by value is forbidden. Use pointer to Header instead.
	Header ResponseHeader

	// Flush headers as soon as possible without waiting for first body bytes.
	// Relevant for bodyStream only.
	ImmediateHeaderFlush bool

	bodyStream      io.Reader
	w               responseBodyWriter
	body            *bytebufferpool.ByteBuffer
	bodyRaw         []byte
	maxKeepBodySize int

	// Response.Read() skips reading body if set to true.
	// Use it for reading HEAD responses.
	//
	// Response.Write() skips writing body if set to true.
	// Use it for writing HEAD responses.
	SkipBody bool

	// Remote TCPAddr from concurrently net.Conn
	raddr net.Addr
	// Local TCPAddr from concurrently net.Conn
	laddr net.Addr

	// If set a hijackWriter, hertz will skip the default header/body writer process.
	hijackWriter network.ExtWriter
}

func (resp *Response) GetHijackWriter() network.ExtWriter {
	return resp.hijackWriter
}

func (resp *Response) HijackWriter(writer network.ExtWriter) {
	resp.hijackWriter = writer
}

type responseBodyWriter struct {
	r *Response
}

func (w *responseBodyWriter) Write(p []byte) (int, error) {
	w.r.AppendBody(p)
	return len(p), nil
}

func (resp *Response) MustSkipBody() bool {
	return resp.SkipBody || resp.Header.MustSkipContentLength()
}

// BodyGunzip returns un-gzipped body data.
//
// This method may be used if the response header contains
// 'Content-Encoding: gzip' for reading un-gzipped body.
// Use Body for reading gzipped response body.
func (resp *Response) BodyGunzip() ([]byte, error) {
	return gunzipData(resp.Body())
}

// SetConnectionClose sets 'Connection: close' header.
func (resp *Response) SetConnectionClose() {
	resp.Header.SetConnectionClose(true)
}

// SetBodyString sets response body.
func (resp *Response) SetBodyString(body string) {
	resp.CloseBodyStream()            //nolint:errcheck
	resp.BodyBuffer().SetString(body) //nolint:errcheck
}

func (resp *Response) ConstructBodyStream(body *bytebufferpool.ByteBuffer, bodyStream io.Reader) {
	resp.body = body
	resp.bodyStream = bodyStream
}

// BodyWriter returns writer for populating response body.
//
// If used inside RequestHandler, the returned writer must not be used
// after returning from RequestHandler. Use RequestContext.Write
// or SetBodyStreamWriter in this case.
func (resp *Response) BodyWriter() io.Writer {
	resp.w.r = resp
	return &resp.w
}

// SetStatusCode sets response status code.
func (resp *Response) SetStatusCode(statusCode int) {
	resp.Header.SetStatusCode(statusCode)
}

func (resp *Response) SetMaxKeepBodySize(n int) {
	resp.maxKeepBodySize = n
}

func (resp *Response) BodyBytes() []byte {
	if resp.bodyRaw != nil {
		return resp.bodyRaw
	}
	if resp.body == nil {
		return nil
	}
	return resp.body.B
}

func (resp *Response) HasBodyBytes() bool {
	return len(resp.BodyBytes()) != 0
}

func (resp *Response) CopyToSkipBody(dst *Response) {
	dst.Reset()
	resp.Header.CopyTo(&dst.Header)
	dst.SkipBody = resp.SkipBody
	dst.raddr = resp.raddr
	dst.laddr = resp.laddr
}

// IsBodyStream returns true if body is set via SetBodyStream*
func (resp *Response) IsBodyStream() bool {
	return resp.bodyStream != nil
}

// SetBodyStream sets response body stream and, optionally body size.
//
// If bodySize is >= 0, then the bodyStream must provide exactly bodySize bytes
// before returning io.EOF.
//
// If bodySize < 0, then bodyStream is read until io.EOF.
//
// bodyStream.Close() is called after finishing reading all body data
// if it implements io.Closer.
//
// See also SetBodyStreamWriter.
func (resp *Response) SetBodyStream(bodyStream io.Reader, bodySize int) {
	resp.ResetBody()
	resp.bodyStream = bodyStream
	resp.Header.SetContentLength(bodySize)
}

// SetBodyStreamNoReset is almost the same as SetBodyStream,
// but it doesn't reset the bodyStream before.
func (resp *Response) SetBodyStreamNoReset(bodyStream io.Reader, bodySize int) {
	resp.bodyStream = bodyStream
	resp.Header.SetContentLength(bodySize)
}

// BodyE returns response body.
func (resp *Response) BodyE() ([]byte, error) {
	if resp.bodyStream != nil {
		bodyBuf := resp.BodyBuffer()
		bodyBuf.Reset()
		zw := network.NewWriter(bodyBuf)
		_, err := utils.CopyZeroAlloc(zw, resp.bodyStream)
		resp.CloseBodyStream() //nolint:errcheck
		if err != nil {
			return nil, err
		}
	}
	return resp.BodyBytes(), nil
}

// Body returns response body.
// if get body failed, returns nil.
func (resp *Response) Body() []byte {
	body, _ := resp.BodyE()
	return body
}

// BodyWriteTo writes response body to w.
func (resp *Response) BodyWriteTo(w io.Writer) error {
	zw := network.NewWriter(w)
	if resp.bodyStream != nil {
		_, err := utils.CopyZeroAlloc(zw, resp.bodyStream)
		resp.CloseBodyStream() //nolint:errcheck
		return err
	}

	body := resp.BodyBytes()
	zw.WriteBinary(body) //nolint:errcheck
	return zw.Flush()
}

// CopyTo copies resp contents to dst except of body stream.
func (resp *Response) CopyTo(dst *Response) {
	resp.CopyToSkipBody(dst)
	if resp.bodyRaw != nil {
		dst.bodyRaw = append(dst.bodyRaw[:0], resp.bodyRaw...)
		if dst.body != nil {
			dst.body.Reset()
		}
	} else if resp.body != nil {
		dst.BodyBuffer().Set(resp.body.B)
	} else if dst.body != nil {
		dst.body.Reset()
	}
}

func SwapResponseBody(a, b *Response) {
	a.body, b.body = b.body, a.body
	a.bodyRaw, b.bodyRaw = b.bodyRaw, a.bodyRaw
	a.bodyStream, b.bodyStream = b.bodyStream, a.bodyStream
}

// Reset clears response contents.
func (resp *Response) Reset() {
	resp.Header.Reset()
	resp.resetSkipHeader()
	resp.SkipBody = false
	resp.raddr = nil
	resp.laddr = nil
	resp.ImmediateHeaderFlush = false
	resp.hijackWriter = nil
}

func (resp *Response) resetSkipHeader() {
	resp.ResetBody()
}

// ResetBody resets response body.
func (resp *Response) ResetBody() {
	resp.bodyRaw = nil
	resp.CloseBodyStream() //nolint:errcheck
	if resp.body != nil {
		if resp.body.Len() <= resp.maxKeepBodySize {
			resp.body.Reset()
			return
		}
		responseBodyPool.Put(resp.body)
		resp.body = nil
	}
}

// SetBodyRaw sets response body, but without copying it.
//
// From this point onward the body argument must not be changed.
func (resp *Response) SetBodyRaw(body []byte) {
	resp.ResetBody()
	resp.bodyRaw = body
}

// StatusCode returns response status code.
func (resp *Response) StatusCode() int {
	return resp.Header.StatusCode()
}

// SetBody sets response body.
//
// It is safe re-using body argument after the function returns.
func (resp *Response) SetBody(body []byte) {
	resp.CloseBodyStream() //nolint:errcheck
	if resp.GetHijackWriter() == nil {
		resp.BodyBuffer().Set(body) //nolint:errcheck
		return
	}

	// If the hijack writer support .SetBody() api, then use it.
	if setter, ok := resp.GetHijackWriter().(interface {
		SetBody(b []byte)
	}); ok {
		setter.SetBody(body)
		return
	}

	// Otherwise, call .Write() api instead.
	resp.GetHijackWriter().Write(body) //nolint:errcheck
}

func (resp *Response) BodyStream() io.Reader {
	if resp.bodyStream == nil {
		resp.bodyStream = NoResponseBody
	}
	return resp.bodyStream
}

// AppendBody appends p to response body.
//
// It is safe re-using p after the function returns.
func (resp *Response) AppendBody(p []byte) {
	resp.CloseBodyStream() //nolint:errcheck
	if resp.hijackWriter != nil {
		resp.hijackWriter.Write(p) //nolint:errcheck
		return
	}
	resp.BodyBuffer().Write(p) //nolint:errcheck
}

// AppendBodyString appends s to response body.
func (resp *Response) AppendBodyString(s string) {
	resp.CloseBodyStream() //nolint:errcheck
	if resp.hijackWriter != nil {
		resp.hijackWriter.Write(bytesconv.S2b(s)) //nolint:errcheck
		return
	}
	resp.BodyBuffer().WriteString(s) //nolint:errcheck
}

// ConnectionClose returns true if 'Connection: close' header is set.
func (resp *Response) ConnectionClose() bool {
	return resp.Header.ConnectionClose()
}

func (resp *Response) CloseBodyStream() error {
	if resp.bodyStream == nil {
		return nil
	}
	var err error
	if bsc, ok := resp.bodyStream.(io.Closer); ok {
		err = bsc.Close()
	}
	resp.bodyStream = nil
	return err
}

func (resp *Response) BodyBuffer() *bytebufferpool.ByteBuffer {
	if resp.body == nil {
		resp.body = responseBodyPool.Get()
	}
	resp.bodyRaw = nil
	return resp.body
}

func gunzipData(p []byte) ([]byte, error) {
	var bb bytebufferpool.ByteBuffer
	_, err := compress.WriteGunzip(&bb, p)
	if err != nil {
		return nil, err
	}
	return bb.B, nil
}

// RemoteAddr returns the remote network address. The Addr returned is shared
// by all invocations of RemoteAddr, so do not modify it.
func (resp *Response) RemoteAddr() net.Addr {
	return resp.raddr
}

// LocalAddr returns the local network address. The Addr returned is shared
// by all invocations of LocalAddr, so do not modify it.
func (resp *Response) LocalAddr() net.Addr {
	return resp.laddr
}

func (resp *Response) ParseNetAddr(conn network.Conn) {
	resp.raddr = conn.RemoteAddr()
	resp.laddr = conn.LocalAddr()
}

// AcquireResponse returns an empty Response instance from response pool.
//
// The returned Response instance may be passed to ReleaseResponse when it is
// no longer needed. This allows Response recycling, reduces GC pressure
// and usually improves performance.
func AcquireResponse() *Response {
	v := responsePool.Get()
	if v == nil {
		return &Response{}
	}
	return v.(*Response)
}

// ReleaseResponse return resp acquired via AcquireResponse to response pool.
//
// It is forbidden accessing resp and/or its members after returning
// it to response pool.
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}
