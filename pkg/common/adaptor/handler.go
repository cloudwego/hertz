/*
 * Copyright 2025 CloudWeGo Authors
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
 */

package adaptor

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"runtime"
	"sync"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

// HertzHandler converts a http.Handler to an app.HandlerFunc.
func HertzHandler(h http.Handler) app.HandlerFunc {
	return func(ctx context.Context, rc *app.RequestContext) {
		// creating http.Request
		r := &rc.Request
		req, err := http.NewRequestWithContext(ctx, methodstr(r.Method()), r.URI().String(), nil)
		if err != nil {
			rc.SetStatusCode(consts.StatusInternalServerError)
			rc.SetBodyString(err.Error())
			return
		}

		// request Body & ContentLength
		stream := r.IsBodyStream()
		if stream && r.HasMultipartForm() && len(r.BodyBytes()) > 0 {
			// in this case, r.MultipartForm() called before this handler then we can not rely on stream body.
			// coz both r.Body() and r.BodyStream() will return nothing due to EOF of stream body.
			// we fix it by calling r.CloseBodyStream(), then r.Body() correctly returns bytes.
			// FIXME: This should be taken care of by `protocol.Request`
			r.CloseBodyStream()
			stream = false
		}
		if stream {
			// use BodyStream if possible to avoid OOM issue.
			req.Body = reader2closer(r.BodyStream())
			req.ContentLength = int64(r.Header.ContentLength())
		} else {
			// fast path for default cases with StreamRequestBody=false
			b := r.Body()
			req.Body = newBytesRWCloser(b)
			req.ContentLength = int64(len(b))
		}

		// request Header
		r.Header.VisitAll(func(k, v []byte) {
			req.Header[string(k)] = append(req.Header[string(k)], string(v))
		})

		// request other properties
		if s := r.Header.GetProtocol(); s != "" {
			req.Proto = s
			req.ProtoMajor, req.ProtoMinor, _ = parseHTTPVersion(s)
		}
		req.Close = r.ConnectionClose()
		req.RemoteAddr = rc.RemoteAddr().String()
		req.RequestURI = string(r.RequestURI())
		if tlsconn, ok := rc.GetConn().(network.ConnTLSer); ok {
			state := tlsconn.ConnectionState()
			req.TLS = &state
		}

		// creating http.ResponseWriter
		//
		// coz it's server response
		// no need to copy anything from hertz Response
		w := &httpResponseWriter{rc: rc}

		h.ServeHTTP(w, req)
		if w.hijacked != nil {
			// wait for hijacked conn to close before returning,
			// otherwise either hertz will close the conn
			// or netpoll may reuse the conn for next request.
			<-w.hijacked
		}
	}
}

type httpResponseWriter struct {
	rc     *app.RequestContext
	header http.Header

	err error

	wroteHeader bool
	skipBody    bool

	hijacked chan struct{} // != nil if hijacked
}

var errConnHijacked = errors.New("hertz net/http adaptor: conn hijacked")

func (p *httpResponseWriter) Header() http.Header {
	if p.header != nil {
		return p.header
	}
	p.header = make(map[string][]string)
	return p.header
}

// Write implements http.ResponseWriter.Write
func (p *httpResponseWriter) Write(b []byte) (n int, err error) {
	if p.hijacked != nil {
		return 0, errConnHijacked
	}
	if !p.wroteHeader {
		p.WriteHeader(consts.StatusOK)
	}
	if p.err != nil {
		return 0, p.err
	}
	if p.skipBody {
		return len(b), nil
	}
	n, p.err = p.rc.Response.GetHijackWriter().Write(b)
	return n, p.err
}

// WriteHeader implements http.ResponseWriter.WriteHeader
func (p *httpResponseWriter) WriteHeader(statusCode int) {
	if p.wroteHeader || p.hijacked != nil {
		return
	}
	p.wroteHeader = true
	r := &p.rc.Response
	// reset and check if user updates Content-Length
	// if we have no Content-Length, can only use chunked Transfer-Encoding
	r.Header.InitContentLengthWithValue(-1)
	for k, vv := range p.header {
		for _, v := range vv {
			r.Header.Add(k, v)
		}
	}
	w := p.rc.GetWriter()
	r.Header.SetStatusCode(statusCode)
	p.skipBody = r.Header.MustSkipContentLength() ||
		string(p.rc.Request.Method()) == consts.MethodHead
	if p.skipBody {
		// set Content-Length: 0
		r.Header.SetCanonical(bytestr.StrContentLength, []byte("0"))
		// skip all further writes,
		// must be set for hertz request loop or it would write header and body after handler returns
		r.HijackWriter(noopWriter{})
		p.err = resp.WriteHeader(&r.Header, w)
	} else if r.Header.ContentLength() < 0 {
		// For chunked encoding, write headers immediately
		cw := resp.NewChunkedBodyWriter(r, w)
		r.HijackWriter(cw)
		type chunkedBodyWriter interface {
			WriteHeader() error
		}
		p.err = cw.(chunkedBodyWriter).WriteHeader()
	} else {
		// use Writer directly instead of keep buffering data in resp.BodyBuffer()
		// you never know how much data would be written to response
		r.HijackWriter(writer2writerExt(w))
		p.err = resp.WriteHeader(&r.Header, w)
	}
}

var _ http.Flusher = (*httpResponseWriter)(nil)

// Flush implements http.Flusher and captures any flush errors
func (p *httpResponseWriter) Flush() {
	if p.err == nil {
		p.err = p.rc.GetWriter().Flush()
	}
}

var _ http.Hijacker = (*httpResponseWriter)(nil)

// Hijack implements http.Hijacker
func (p *httpResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if p.hijacked != nil {
		return nil, nil, errConnHijacked
	}
	if p.err != nil {
		return nil, nil, p.err
	}
	// If headers were already written, flush the buffer to avoid losing
	// any pending data before hijacking the connection
	if p.wroteHeader {
		if p.err = p.rc.GetWriter().Flush(); p.err != nil {
			return nil, nil, p.err
		}
	}
	conn := newHijackedConn(p.rc.GetConn())
	p.hijacked = conn.closeCh

	// reset timeout if any
	_ = conn.SetReadTimeout(0)
	_ = conn.SetWriteTimeout(0)

	// make sure after handler returns:
	// * hertz won't reuse the conn
	// * hertz won't write any extra bytes to underlying conn
	p.rc.Response.Header.SetConnectionClose(true)
	p.rc.Response.HijackWriter(noopHijackWriter{})

	return conn, bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)), nil
}

type noopHijackWriter struct{}

var _ network.ExtWriter = noopHijackWriter{}

func (noopHijackWriter) Write(b []byte) (int, error) {
	return 0, errConnHijacked
}
func (noopHijackWriter) Flush() error    { return errConnHijacked }
func (noopHijackWriter) Finalize() error { return nil }

type noopWriter struct{}

var _ network.ExtWriter = noopWriter{}

func (noopWriter) Write(b []byte) (int, error) { return len(b), nil }
func (noopWriter) Flush() error                { return nil }
func (noopWriter) Finalize() error             { return nil }

type hijackedConn struct {
	network.Conn

	closeOnce sync.Once
	closeCh   chan struct{}
}

func newHijackedConn(conn network.Conn) *hijackedConn {
	c := &hijackedConn{Conn: conn, closeCh: make(chan struct{})}
	runtime.SetFinalizer(c, hijackedConnFinalizer)
	return c
}

func hijackedConnFinalizer(c *hijackedConn) {
	_ = c.Close()
}

func (c *hijackedConn) Close() error {
	runtime.SetFinalizer(c, nil)
	err := c.Conn.Close()
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
	return err
}
