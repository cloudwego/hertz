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
	}
}

type httpResponseWriter struct {
	rc     *app.RequestContext
	header http.Header

	err error

	wroteHeader bool
	hijacked    bool
	skipBody    bool
}

var errConnHijacked = errors.New("hertz net/http adaptor: conn hijacked")

func (p *httpResponseWriter) Header() http.Header {
	if p.header != nil {
		return p.header
	}
	p.header = make(map[string][]string)
	return p.header
}

func (p *httpResponseWriter) Write(b []byte) (n int, _ error) {
	if p.hijacked {
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

func (p *httpResponseWriter) WriteHeader(statusCode int) {
	if p.wroteHeader || p.hijacked {
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
		return
	}
	if r.Header.ContentLength() < 0 {
		r.HijackWriter(resp.NewChunkedBodyWriter(r, w))
	} else {
		p.err = resp.WriteHeader(&r.Header, w)
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
