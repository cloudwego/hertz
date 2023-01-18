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

package req

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/ext"
)

var (
	errRequestHostRequired = errs.NewPublic("missing required Host header in request")
	errGetOnly             = errs.NewPublic("non-GET request received")
	errBodyTooLarge        = errs.New(errs.ErrBodyTooLarge, errs.ErrorTypePublic, "http1/req")
)

type h1Request struct {
	*protocol.Request
}

// String returns request representation.
//
// Returns error message instead of request representation on error.
//
// Use Write instead of String for performance-critical code.
func (h1Req *h1Request) String() string {
	w := bytebufferpool.Get()
	zw := network.NewWriter(w)
	if err := Write(h1Req.Request, zw); err != nil {
		return err.Error()
	}
	if err := zw.Flush(); err != nil {
		return err.Error()
	}
	s := string(w.B)
	bytebufferpool.Put(w)
	return s
}

func GetHTTP1Request(req *protocol.Request) fmt.Stringer {
	return &h1Request{req}
}

// ReadHeaderAndLimitBody reads request from the given r, limiting the body size.
//
// If maxBodySize > 0 and the body size exceeds maxBodySize,
// then errBodyTooLarge is returned.
//
// RemoveMultipartFormFiles or Reset must be called after
// reading multipart/form-data request in order to delete temporarily
// uploaded files.
//
// If MayContinue returns true, the caller must:
//
//   - Either send StatusExpectationFailed response if request headers don't
//     satisfy the caller.
//   - Or send StatusContinue response before reading request body
//     with ContinueReadBody.
//   - Or close the connection.
//
// io.EOF is returned if r is closed before reading the first header byte.
func ReadHeaderAndLimitBody(req *protocol.Request, r network.Reader, maxBodySize int, preParse ...bool) error {
	var parse bool
	if len(preParse) == 0 {
		parse = true
	} else {
		parse = preParse[0]
	}
	req.ResetSkipHeader()

	if err := ReadHeader(&req.Header, r); err != nil {
		return err
	}

	return ReadLimitBody(req, r, maxBodySize, false, parse)
}

// Read reads request (including body) from the given r.
//
// RemoveMultipartFormFiles or Reset must be called after
// reading multipart/form-data request in order to delete temporarily
// uploaded files.
//
// If MayContinue returns true, the caller must:
//
//   - Either send StatusExpectationFailed response if request headers don't
//     satisfy the caller.
//   - Or send StatusContinue response before reading request body
//     with ContinueReadBody.
//   - Or close the connection.
//
// io.EOF is returned if r is closed before reading the first header byte.
func Read(req *protocol.Request, r network.Reader, preParse ...bool) error {
	return ReadHeaderAndLimitBody(req, r, 0, preParse...)
}

// Write writes request to w.
//
// Write doesn't flush request to w for performance reasons.
//
// See also WriteTo.
func Write(req *protocol.Request, w network.Writer) error {
	return write(req, w, false)
}

// ProxyWrite is like Write but writes the request in the form
// expected by an HTTP proxy. In particular, ProxyWrite writes the
// initial Request-URI line of the request with an absolute URI, per
// section 5.3 of RFC 7230, including the scheme and host.
func ProxyWrite(req *protocol.Request, w network.Writer) error {
	return write(req, w, true)
}

// write writes request to w.
// It supports proxy situation.
func write(req *protocol.Request, w network.Writer, usingProxy bool) error {
	if len(req.Header.Host()) == 0 || req.IsURIParsed() {
		uri := req.URI()
		host := uri.Host()
		if len(host) == 0 {
			return errRequestHostRequired
		}

		if len(req.Header.Host()) == 0 {
			req.Header.SetHostBytes(host)
		}

		ruri := uri.RequestURI()
		if bytes.Equal(req.Method(), bytestr.StrConnect) {
			ruri = uri.Host()
		} else if usingProxy {
			ruri = uri.FullURI()
		}

		req.Header.SetRequestURIBytes(ruri)

		if len(uri.Username()) > 0 {
			// RequestHeader.SetBytesKV only uses RequestHeader.bufKV.key
			// So we are free to use RequestHeader.bufKV.value as a scratch pad for
			// the base64 encoding.
			nl := len(uri.Username()) + len(uri.Password()) + 1
			nb := nl + len(bytestr.StrBasicSpace)
			tl := nb + base64.StdEncoding.EncodedLen(nl)

			req.Header.InitBufValue(tl)
			buf := req.Header.GetBufValue()[:0]
			buf = append(buf, uri.Username()...)
			buf = append(buf, bytestr.StrColon...)
			buf = append(buf, uri.Password()...)
			buf = append(buf, bytestr.StrBasicSpace...)
			base64.StdEncoding.Encode(buf[nb:tl], buf[:nl])
			req.Header.SetBytesKV(bytestr.StrAuthorization, buf[nl:tl])
		}
	}

	if req.IsBodyStream() {
		return writeBodyStream(req, w)
	}

	body := req.BodyBytes()
	err := handleMultipart(req)
	if err != nil {
		return fmt.Errorf("error when handle multipart: %s", err)
	}
	if req.OnlyMultipartForm() {
		m, _ := req.MultipartForm() // req.multipartForm != nil
		body, err = protocol.MarshalMultipartForm(m, req.MultipartFormBoundary())
		if err != nil {
			return fmt.Errorf("error when marshaling multipart form: %s", err)
		}
		req.Header.SetMultipartFormBoundary(req.MultipartFormBoundary())
	}

	hasBody := false
	if len(body) == 0 {
		body = req.PostArgString()
	}
	if len(body) != 0 || !req.Header.IgnoreBody() {
		hasBody = true
		req.Header.SetContentLength(len(body))
	}

	header := req.Header.Header()
	if _, err := w.WriteBinary(header); err != nil {
		return err
	}

	// Write body
	if hasBody {
		w.WriteBinary(body) //nolint:errcheck
	} else if len(body) > 0 {
		return fmt.Errorf("non-zero body for non-POST request. body=%q", body)
	}
	return nil
}

// ContinueReadBodyStream reads request body in stream if request header contains
// 'Expect: 100-continue'.
//
// The caller must send StatusContinue response before calling this method.
//
// If maxBodySize > 0 and the body size exceeds maxBodySize,
// then errBodyTooLarge is returned.
func ContinueReadBodyStream(req *protocol.Request, zr network.Reader, maxBodySize int, preParseMultipartForm ...bool) error {
	var err error
	contentLength := req.Header.ContentLength()
	if contentLength > 0 {
		if len(preParseMultipartForm) == 0 || preParseMultipartForm[0] {
			// Pre-read multipart form data of known length.
			// This way we limit memory usage for large file uploads, since their contents
			// is streamed into temporary files if file size exceeds defaultMaxInMemoryFileSize.
			req.SetMultipartFormBoundary(string(req.Header.MultipartFormBoundary()))
			if len(req.MultipartFormBoundary()) > 0 && len(req.Header.PeekContentEncoding()) == 0 {
				err := protocol.ParseMultipartForm(zr.(io.Reader), req, contentLength, consts.DefaultMaxInMemoryFileSize)
				if err != nil {
					req.Reset()
				}
				return err
			}
		}
	}

	if contentLength == -2 {
		// identity body has no sense for http requests, since
		// the end of body is determined by connection close.
		// So just ignore request body for requests without
		// 'Content-Length' and 'Transfer-Encoding' headers.

		// refer to https://tools.ietf.org/html/rfc7230#section-3.3.2
		if !req.Header.IgnoreBody() {
			req.Header.SetContentLength(0)
		}
		return nil
	}

	bodyBuf := req.BodyBuffer()
	bodyBuf.Reset()
	bodyBuf.B, err = ext.ReadBodyWithStreaming(zr, contentLength, maxBodySize, bodyBuf.B)
	if err != nil {
		if errors.Is(err, errs.ErrBodyTooLarge) {
			req.Header.SetContentLength(contentLength)
			req.ConstructBodyStream(bodyBuf, ext.AcquireBodyStream(bodyBuf, zr, req.Header.Trailer(), contentLength))

			return nil
		}
		if errors.Is(err, errs.ErrChunkedStream) {
			req.ConstructBodyStream(bodyBuf, ext.AcquireBodyStream(bodyBuf, zr, req.Header.Trailer(), contentLength))
			return nil
		}
		req.Reset()
		return err
	}

	req.ConstructBodyStream(bodyBuf, ext.AcquireBodyStream(bodyBuf, zr, req.Header.Trailer(), contentLength))
	return nil
}

func ContinueReadBody(req *protocol.Request, r network.Reader, maxBodySize int, preParseMultipartForm ...bool) error {
	var err error
	contentLength := req.Header.ContentLength()
	if contentLength > 0 {
		if maxBodySize > 0 && contentLength > maxBodySize {
			return errBodyTooLarge
		}

		if len(preParseMultipartForm) == 0 || preParseMultipartForm[0] {
			// Pre-read multipart form data of known length.
			// This way we limit memory usage for large file uploads, since their contents
			// is streamed into temporary files if file size exceeds defaultMaxInMemoryFileSize.
			req.SetMultipartFormBoundary(string(req.Header.MultipartFormBoundary()))
			if len(req.MultipartFormBoundary()) > 0 && len(req.Header.PeekContentEncoding()) == 0 {
				err := protocol.ParseMultipartForm(r.(io.Reader), req, contentLength, consts.DefaultMaxInMemoryFileSize)
				if err != nil {
					req.Reset()
				}
				return err
			}
		}

		// This optimization is just suitable for ping-pong case and the ext.ReadBody is
		// a common function, so we just handle this situation before ext.ReadBody
		buf, err := r.Peek(contentLength)
		if err != nil {
			return err
		}
		r.Skip(contentLength) // nolint: errcheck
		req.SetBodyRaw(buf)
		return nil
	}

	if contentLength == -2 {
		// identity body has no sense for http requests, since
		// the end of body is determined by connection close.
		// So just ignore request body for requests without
		// 'Content-Length' and 'Transfer-Encoding' headers.

		// refer to https://tools.ietf.org/html/rfc7230#section-3.3.2
		if !req.Header.IgnoreBody() {
			req.Header.SetContentLength(0)
		}
		return nil
	}

	bodyBuf := req.BodyBuffer()
	bodyBuf.Reset()
	bodyBuf.B, err = ext.ReadBody(r, contentLength, maxBodySize, bodyBuf.B)
	if err != nil {
		req.Reset()
		return err
	}

	if req.Header.ContentLength() == -1 {
		err = ext.ReadTrailer(req.Header.Trailer(), r)
		if err != nil && err != io.EOF {
			return err
		}
	}

	req.Header.SetContentLength(len(bodyBuf.B))
	return nil
}

func ReadBodyStream(req *protocol.Request, zr network.Reader, maxBodySize int, getOnly, preParseMultipartForm bool) error {
	if getOnly && !req.Header.IsGet() {
		return errGetOnly
	}

	if req.MayContinue() {
		// 'Expect: 100-continue' header found. Let the caller deciding
		// whether to read request body or
		// to return StatusExpectationFailed.
		return nil
	}

	return ContinueReadBodyStream(req, zr, maxBodySize, preParseMultipartForm)
}

func ReadLimitBody(req *protocol.Request, r network.Reader, maxBodySize int, getOnly, preParseMultipartForm bool) error {
	// Do not reset the request here - the caller must reset it before
	// calling this method.
	if getOnly && !req.Header.IsGet() {
		return errGetOnly
	}

	if req.MayContinue() {
		// 'Expect: 100-continue' header found. Let the caller deciding
		// whether to read request body or
		// to return StatusExpectationFailed.
		return nil
	}

	return ContinueReadBody(req, r, maxBodySize, preParseMultipartForm)
}

func writeBodyStream(req *protocol.Request, w network.Writer) error {
	var err error

	contentLength := req.Header.ContentLength()
	if contentLength < 0 {
		lrSize := ext.LimitedReaderSize(req.BodyStream())
		if lrSize >= 0 {
			contentLength = int(lrSize)
			if int64(contentLength) != lrSize {
				contentLength = -1
			}
			if contentLength >= 0 {
				req.Header.SetContentLength(contentLength)
			}
		}
	}
	if contentLength >= 0 {
		if err = WriteHeader(&req.Header, w); err == nil {
			err = ext.WriteBodyFixedSize(w, req.BodyStream(), int64(contentLength))
		}
	} else {
		req.Header.SetContentLength(-1)
		err = WriteHeader(&req.Header, w)
		if err == nil {
			err = ext.WriteBodyChunked(w, req.BodyStream())
		}
		if err == nil {
			err = ext.WriteTrailer(req.Header.Trailer(), w)
		}
	}
	err1 := req.CloseBodyStream()
	if err == nil {
		err = err1
	}
	return err
}

func handleMultipart(req *protocol.Request) error {
	if len(req.MultipartFiles()) == 0 && len(req.MultipartFields()) == 0 {
		return nil
	}
	var err error
	bodyBuffer := &bytes.Buffer{}
	w := multipart.NewWriter(bodyBuffer)
	if len(req.MultipartFiles()) > 0 {
		for _, f := range req.MultipartFiles() {
			if f.Reader != nil {
				err = protocol.WriteMultipartFormFile(w, f.ParamName, f.Name, f.Reader)
			} else {
				err = protocol.AddFile(w, f.ParamName, f.Name)
			}
			if err != nil {
				return err
			}
		}
	}

	if len(req.MultipartFields()) > 0 {
		for _, mf := range req.MultipartFields() {
			if err = protocol.AddMultipartFormField(w, mf); err != nil {
				return err
			}
		}
	}

	req.Header.Set(consts.HeaderContentType, w.FormDataContentType())
	if err = w.Close(); err != nil {
		return err
	}

	r := multipart.NewReader(bodyBuffer, w.Boundary())
	f, err := r.ReadForm(int64(bodyBuffer.Len()))
	if err != nil {
		return err
	}
	protocol.SetMultipartFormWithBoundary(req, f, w.Boundary())

	return nil
}
