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

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

func HertzHandler(h http.Handler) app.HandlerFunc {
	return func(ctx context.Context, rc *app.RequestContext) {
		// http.Request
		r := &rc.Request
		req, err := http.NewRequestWithContext(ctx, methodstr(r.Method()), r.URI().String(), nil)
		if err != nil {
			rc.SetStatusCode(500)
			rc.SetBodyString(err.Error())
			return
		}
		req.RemoteAddr = rc.RemoteAddr().String()

		if r.IsBodyStream() {
			req.Body = reader2closer(r.BodyStream())
			req.ContentLength = int64(r.Header.ContentLength())

		} else { // hot path
			b := r.Body()
			req.Body = newBytesRWCloser(b)
			req.ContentLength = int64(len(b))
		}

		r.Header.VisitAll(func(k, v []byte) {
			req.Header[string(k)] = append(req.Header[string(k)], string(v))
		})

		// http.ResponseWriter
		// coz it's server response
		// no need to copy anything from hertz Response
		w := &httpResponseWriter{rc: rc}
		h.ServeHTTP(w, req)
	}
}

type httpResponseWriter struct {
	rc     *app.RequestContext
	header http.Header

	wroteHeader bool
	hijacked    bool
}

var errConnHijacked = errors.New("hertz net/http adaptor: conn hijacked")

func (p *httpResponseWriter) Header() http.Header {
	if p.header != nil {
		return p.header
	}
	p.header = make(map[string][]string)
	return p.header
}

func (p *httpResponseWriter) Write(b []byte) (int, error) {
	if p.hijacked {
		return 0, errConnHijacked
	}
	if !p.wroteHeader {
		p.WriteHeader(consts.StatusOK)
	}
	return p.rc.Response.GetHijackWriter().Write(b)
}

func (p *httpResponseWriter) WriteHeader(statusCode int) {
	if p.wroteHeader {
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
	r.Header.SetStatusCode(statusCode)

	w := p.rc.GetWriter()
	if r.Header.ContentLength() == -1 {
		r.HijackWriter(resp.NewChunkedBodyWriter(r, w))
	} else {
		_ = resp.WriteHeader(&r.Header, w)
		// use Writer directly instead of keep buffering data in resp.BodyBuffer()
		// you never know how much data would be written to response
		r.HijackWriter(writer2writerExt(w))
	}
}

var _ http.Flusher = (*httpResponseWriter)(nil)

// Flush implements the [http.Flusher]
func (p *httpResponseWriter) Flush() {
	_ = p.rc.GetWriter().Flush()
}

var _ http.Hijacker = (*httpResponseWriter)(nil)

// Hijack implements the [net/http.Hijacker]
func (p *httpResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if p.hijacked {
		return nil, nil, errConnHijacked
	}
	p.hijacked = true

	conn := p.rc.GetConn()

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

func (noopHijackWriter) Write(b []byte) (int, error) {
	return 0, errConnHijacked
}
func (noopHijackWriter) Flush() error    { return errConnHijacked }
func (noopHijackWriter) Finalize() error { return nil }
