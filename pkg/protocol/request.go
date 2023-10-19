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
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/internal/nocopy"
	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/compress"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	errMissingFile = errors.NewPublic("http: no such file")

	responseBodyPool bytebufferpool.Pool
	requestBodyPool  bytebufferpool.Pool

	requestPool sync.Pool
)

// NoBody is an io.ReadCloser with no bytes. Read always returns EOF
// and Close always returns nil. It can be used in an outgoing client
// request to explicitly signal that a request has zero bytes.
var NoBody = noBody{}

type noBody struct{}

func (noBody) Read([]byte) (int, error) { return 0, io.EOF }
func (noBody) Close() error             { return nil }

type Request struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	Header RequestHeader

	uri      URI
	postArgs Args

	bodyStream      io.Reader
	w               requestBodyWriter
	body            *bytebufferpool.ByteBuffer
	bodyRaw         []byte
	maxKeepBodySize int

	multipartForm         *multipart.Form
	multipartFormBoundary string

	// Group bool members in order to reduce Request object size.
	parsedURI      bool
	parsedPostArgs bool

	isTLS bool

	multipartFiles  []*File
	multipartFields []*MultipartField

	// Request level options, service discovery options etc.
	options *config.RequestOptions
}

type requestBodyWriter struct {
	r *Request
}

// File struct represent file information for multipart request
type File struct {
	Name      string
	ParamName string
	io.Reader
}

// MultipartField struct represent custom data part for multipart request
type MultipartField struct {
	Param       string
	FileName    string
	ContentType string
	io.Reader
}

func (w *requestBodyWriter) Write(p []byte) (int, error) {
	w.r.AppendBody(p)
	return len(p), nil
}

func (req *Request) Options() *config.RequestOptions {
	if req.options == nil {
		req.options = config.NewRequestOptions(nil)
	}
	return req.options
}

// AppendBody appends p to request body.
//
// It is safe re-using p after the function returns.
func (req *Request) AppendBody(p []byte) {
	req.RemoveMultipartFormFiles()
	req.CloseBodyStream()     //nolint:errcheck
	req.BodyBuffer().Write(p) //nolint:errcheck
}

func (req *Request) BodyBuffer() *bytebufferpool.ByteBuffer {
	if req.body == nil {
		req.body = requestBodyPool.Get()
	}
	req.bodyRaw = nil
	return req.body
}

// MayContinue returns true if the request contains
// 'Expect: 100-continue' header.
//
// The caller must do one of the following actions if MayContinue returns true:
//
//   - Either send StatusExpectationFailed response if request headers don't
//     satisfy the caller.
//   - Or send StatusContinue response before reading request body
//     with ContinueReadBody.
//   - Or close the connection.
func (req *Request) MayContinue() bool {
	return bytes.Equal(req.Header.peek(bytestr.StrExpect), bytestr.Str100Continue)
}

// Scheme returns the scheme of the request.
// uri will be parsed in ServeHTTP(before user's process), so that there is no need for uri nil-check.
func (req *Request) Scheme() []byte {
	return req.uri.Scheme()
}

// For keepalive connection reuse.
// It is roughly the same as ResetSkipHeader, except that the connection related fields are removed:
// - req.isTLS
func (req *Request) resetSkipHeaderAndConn() {
	req.ResetBody()
	req.uri.Reset()
	req.parsedURI = false
	req.parsedPostArgs = false
	req.postArgs.Reset()
}

func (req *Request) ResetSkipHeader() {
	req.resetSkipHeaderAndConn()
	req.isTLS = false
}

func SwapRequestBody(a, b *Request) {
	a.body, b.body = b.body, a.body
	a.bodyRaw, b.bodyRaw = b.bodyRaw, a.bodyRaw
	a.bodyStream, b.bodyStream = b.bodyStream, a.bodyStream
	a.multipartFields, b.multipartFields = b.multipartFields, a.multipartFields
	a.multipartFiles, b.multipartFiles = b.multipartFiles, a.multipartFiles
}

// Reset clears request contents.
func (req *Request) Reset() {
	req.Header.Reset()
	req.ResetSkipHeader()
	req.CloseBodyStream()

	req.options = nil
}

func (req *Request) IsURIParsed() bool {
	return req.parsedURI
}

func (req *Request) PostArgString() []byte {
	return req.postArgs.QueryString()
}

// MultipartForm returns request's multipart form.
//
// Returns errors.ErrNoMultipartForm if request's Content-Type
// isn't 'multipart/form-data'.
//
// RemoveMultipartFormFiles must be called after returned multipart form
// is processed.
func (req *Request) MultipartForm() (*multipart.Form, error) {
	if req.multipartForm != nil {
		return req.multipartForm, nil
	}
	req.multipartFormBoundary = string(req.Header.MultipartFormBoundary())
	if len(req.multipartFormBoundary) == 0 {
		return nil, errors.ErrNoMultipartForm
	}

	ce := req.Header.peek(bytestr.StrContentEncoding)
	var err error
	var f *multipart.Form

	if !req.IsBodyStream() {
		body := req.BodyBytes()
		if bytes.Equal(ce, bytestr.StrGzip) {
			// Do not care about memory usage here.
			var err error
			if body, err = compress.AppendGunzipBytes(nil, body); err != nil {
				return nil, fmt.Errorf("cannot gunzip request body: %s", err)
			}
		} else if len(ce) > 0 {
			return nil, fmt.Errorf("unsupported Content-Encoding: %q", ce)
		}
		f, err = ReadMultipartForm(bytes.NewReader(body), req.multipartFormBoundary, len(body), len(body))
	} else {
		bodyStream := req.bodyStream
		if req.Header.contentLength > 0 {
			bodyStream = io.LimitReader(bodyStream, int64(req.Header.contentLength))
		}
		if bytes.Equal(ce, bytestr.StrGzip) {
			// Do not care about memory usage here.
			if bodyStream, err = gzip.NewReader(bodyStream); err != nil {
				return nil, fmt.Errorf("cannot gunzip request body: %w", err)
			}
		} else if len(ce) > 0 {
			return nil, fmt.Errorf("unsupported Content-Encoding: %q", ce)
		}

		mr := multipart.NewReader(bodyStream, req.multipartFormBoundary)

		f, err = mr.ReadForm(8 * 1024)
	}

	if err != nil {
		return nil, err
	}
	req.multipartForm = f
	return f, nil
}

// AppendBodyString appends s to request body.
func (req *Request) AppendBodyString(s string) {
	req.RemoveMultipartFormFiles()
	req.CloseBodyStream()           //nolint:errcheck
	req.BodyBuffer().WriteString(s) //nolint:errcheck
}

// SetRequestURI sets RequestURI.
func (req *Request) SetRequestURI(requestURI string) {
	req.Header.SetRequestURI(requestURI)
	req.parsedURI = false
}

func (req *Request) SetMaxKeepBodySize(n int) {
	req.maxKeepBodySize = n
}

// RequestURI returns the RequestURI for the given request.
func (req *Request) RequestURI() []byte {
	return req.Header.RequestURI()
}

// FormFile returns the first file for the provided form key.
func (req *Request) FormFile(name string) (*multipart.FileHeader, error) {
	mf, err := req.MultipartForm()
	if err != nil {
		return nil, err
	}
	if mf.File == nil {
		return nil, err
	}
	fhh := mf.File[name]
	if fhh == nil {
		return nil, errMissingFile
	}
	return fhh[0], nil
}

// SetHost sets host for the request.
func (req *Request) SetHost(host string) {
	req.URI().SetHost(host)
}

// Host returns the host for the given request.
func (req *Request) Host() []byte {
	return req.URI().Host()
}

// SetIsTLS is used by TLS server to mark whether the request is a TLS request.
// Client shouldn't use this method but should depend on the uri.scheme instead.
func (req *Request) SetIsTLS(isTLS bool) {
	req.isTLS = isTLS
}

// SwapBody swaps request body with the given body and returns
// the previous request body.
//
// It is forbidden to use the body passed to SwapBody after
// the function returns.
func (req *Request) SwapBody(body []byte) []byte {
	bb := req.BodyBuffer()
	zw := network.NewWriter(bb)

	if req.IsBodyStream() {
		bb.Reset()
		_, err := utils.CopyZeroAlloc(zw, req.bodyStream)
		req.CloseBodyStream() //nolint:errcheck
		if err != nil {
			bb.Reset()
			bb.SetString(err.Error())
		}
	}

	req.bodyRaw = nil

	oldBody := bb.B
	bb.B = body
	return oldBody
}

// CopyTo copies req contents to dst except of body stream.
func (req *Request) CopyTo(dst *Request) {
	req.CopyToSkipBody(dst)
	if req.bodyRaw != nil {
		dst.bodyRaw = append(dst.bodyRaw[:0], req.bodyRaw...)
		if dst.body != nil {
			dst.body.Reset()
		}
	} else if req.body != nil {
		dst.BodyBuffer().Set(req.body.B)
	} else if dst.body != nil {
		dst.body.Reset()
	}
}

func (req *Request) CopyToSkipBody(dst *Request) {
	dst.Reset()
	req.Header.CopyTo(&dst.Header)

	req.uri.CopyTo(&dst.uri)
	dst.parsedURI = req.parsedURI

	req.postArgs.CopyTo(&dst.postArgs)
	dst.parsedPostArgs = req.parsedPostArgs
	dst.isTLS = req.isTLS

	if req.options != nil {
		dst.options = &config.RequestOptions{}
		req.options.CopyTo(dst.options)
	}

	// do not copy multipartForm - it will be automatically
	// re-created on the first call to MultipartForm.
}

func (req *Request) BodyBytes() []byte {
	if req.bodyRaw != nil {
		return req.bodyRaw
	}
	if req.body == nil {
		return nil
	}
	return req.body.B
}

// ResetBody resets request body.
func (req *Request) ResetBody() {
	req.bodyRaw = nil
	req.RemoveMultipartFormFiles()
	req.CloseBodyStream() //nolint:errcheck
	if req.body != nil {
		if req.body.Len() <= req.maxKeepBodySize {
			req.body.Reset()
			return
		}
		requestBodyPool.Put(req.body)
		req.body = nil
	}
}

// SetBodyRaw sets request body, but without copying it.
//
// From this point onward the body argument must not be changed.
func (req *Request) SetBodyRaw(body []byte) {
	req.ResetBody()
	req.bodyRaw = body
}

// SetMultipartFormBoundary will set the multipart form boundary for the request.
func (req *Request) SetMultipartFormBoundary(b string) {
	req.multipartFormBoundary = b
}

func (req *Request) MultipartFormBoundary() string {
	return req.multipartFormBoundary
}

// SetBody sets request body.
//
// It is safe re-using body argument after the function returns.
func (req *Request) SetBody(body []byte) {
	req.RemoveMultipartFormFiles()
	req.CloseBodyStream() //nolint:errcheck
	req.BodyBuffer().Set(body)
}

// SetBodyString sets request body.
func (req *Request) SetBodyString(body string) {
	req.RemoveMultipartFormFiles()
	req.CloseBodyStream() //nolint:errcheck
	req.BodyBuffer().SetString(body)
}

// SetQueryString sets query string.
func (req *Request) SetQueryString(queryString string) {
	req.URI().SetQueryString(queryString)
}

// SetFormData sets x-www-form-urlencoded params
func (req *Request) SetFormData(data map[string]string) {
	for k, v := range data {
		req.postArgs.Add(k, v)
	}
	req.parsedPostArgs = true
	req.Header.SetContentTypeBytes(bytestr.StrPostArgsContentType)
}

// SetFormDataFromValues sets x-www-form-urlencoded params from url values.
func (req *Request) SetFormDataFromValues(data url.Values) {
	for k, v := range data {
		for _, kv := range v {
			req.postArgs.Add(k, kv)
		}
	}
	req.parsedPostArgs = true
	req.Header.SetContentTypeBytes(bytestr.StrPostArgsContentType)
}

// SetFile sets single file field name and its path for multipart upload.
func (req *Request) SetFile(param, filePath string) {
	req.multipartFiles = append(req.multipartFiles, &File{
		Name:      filePath,
		ParamName: param,
	})
}

// SetFiles sets multiple file field name and its path for multipart upload.
func (req *Request) SetFiles(files map[string]string) {
	for f, fp := range files {
		req.multipartFiles = append(req.multipartFiles, &File{
			Name:      fp,
			ParamName: f,
		})
	}
}

// SetFileReader sets single file using io.Reader for multipart upload.
func (req *Request) SetFileReader(param, fileName string, reader io.Reader) {
	req.multipartFiles = append(req.multipartFiles, &File{
		Name:      fileName,
		ParamName: param,
		Reader:    reader,
	})
}

// SetMultipartFormData method allows simple form data to be attached to the request as `multipart:form-data`
func (req *Request) SetMultipartFormData(data map[string]string) {
	for k, v := range data {
		req.SetMultipartField(k, "", "", strings.NewReader(v))
	}
}

func (req *Request) MultipartFiles() []*File {
	return req.multipartFiles
}

// SetMultipartField sets custom data using io.Reader for multipart upload.
func (req *Request) SetMultipartField(param, fileName, contentType string, reader io.Reader) {
	req.multipartFields = append(req.multipartFields, &MultipartField{
		Param:       param,
		FileName:    fileName,
		ContentType: contentType,
		Reader:      reader,
	})
}

// SetMultipartFields sets multiple data fields using io.Reader for multipart upload.
func (req *Request) SetMultipartFields(fields ...*MultipartField) {
	req.multipartFields = append(req.multipartFields, fields...)
}

func (req *Request) MultipartFields() []*MultipartField {
	return req.multipartFields
}

// SetBasicAuth sets the basic authentication header in the current HTTP request.
func (req *Request) SetBasicAuth(username, password string) {
	encodeStr := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.SetHeader(consts.HeaderAuthorization, "Basic "+encodeStr)
}

// BasicAuth can return the username and password in the request's Authorization
// header, if the request uses the HTTP Basic Authorization.
func (req *Request) BasicAuth() (username, password string, ok bool) {
	// Using Peek to reduce the cost for type transfer.
	auth := req.Header.Peek(consts.HeaderAuthorization)
	if auth == nil {
		return
	}

	return parseBasicAuth(auth)
}

var prefix = []byte{'B', 'a', 's', 'i', 'c', ' '}

// parseBasicAuth can parse an HTTP Basic Authorization string encrypted by base64.
// Example: "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==" returns ("Aladdin", "open sesame", true).
func parseBasicAuth(auth []byte) (username, password string, ok bool) {
	if len(auth) < len(prefix) || !bytes.EqualFold(auth[:len(prefix)], prefix) {
		return
	}

	decodeLen := base64.StdEncoding.DecodedLen(len(auth[len(prefix):]))
	// base64.StdEncoding.Decode(dst,rsc []byte) will return less than DecodedLen(len(src)))
	decodeData := make([]byte, decodeLen)
	num, err := base64.StdEncoding.Decode(decodeData, auth[len(prefix):])
	if err != nil {
		return
	}

	cs := bytesconv.B2s(decodeData[:num])
	s := strings.IndexByte(cs, ':')

	if s < 0 {
		return
	}

	return cs[:s], cs[s+1:], true
}

// SetAuthToken sets the auth token header(Default Scheme: Bearer) in the current HTTP request. Header example:
//
//	Authorization: Bearer <auth-token-value-comes-here>
func (req *Request) SetAuthToken(token string) {
	req.SetHeader(consts.HeaderAuthorization, "Bearer "+token)
}

// SetAuthSchemeToken sets the auth token scheme type in the HTTP request. For Example:
//
//	Authorization: <auth-scheme-value-set-here> <auth-token-value>
func (req *Request) SetAuthSchemeToken(scheme, token string) {
	req.SetHeader(consts.HeaderAuthorization, scheme+" "+token)
}

// SetHeader sets a single header field and its value in the current request.
func (req *Request) SetHeader(header, value string) {
	req.Header.Set(header, value)
}

// SetHeaders sets multiple header field and its value in the current request.
func (req *Request) SetHeaders(headers map[string]string) {
	for h, v := range headers {
		req.Header.Set(h, v)
	}
}

// SetCookie appends a single cookie in the current request instance.
func (req *Request) SetCookie(key, value string) {
	req.Header.SetCookie(key, value)
}

// SetCookies sets an array of cookies in the current request instance.
func (req *Request) SetCookies(hc map[string]string) {
	for k, v := range hc {
		req.Header.SetCookie(k, v)
	}
}

// SetMethod sets http method for this request.
func (req *Request) SetMethod(method string) {
	req.Header.SetMethod(method)
}

func (req *Request) OnlyMultipartForm() bool {
	return req.multipartForm != nil && (req.body == nil || len(req.body.B) == 0)
}

func (req *Request) HasMultipartForm() bool {
	return req.multipartForm != nil
}

// IsBodyStream returns true if body is set via SetBodyStream*
func (req *Request) IsBodyStream() bool {
	return req.bodyStream != nil && req.bodyStream != NoBody
}

func (req *Request) BodyStream() io.Reader {
	if req.bodyStream == nil {
		req.bodyStream = NoBody
	}
	return req.bodyStream
}

// SetBodyStream sets request body stream and, optionally body size.
//
// If bodySize is >= 0, then the bodyStream must provide exactly bodySize bytes
// before returning io.EOF.
//
// If bodySize < 0, then bodyStream is read until io.EOF.
//
// bodyStream.Close() is called after finishing reading all body data
// if it implements io.Closer.
//
// Note that GET and HEAD requests cannot have body.
//
// See also SetBodyStreamWriter.
func (req *Request) SetBodyStream(bodyStream io.Reader, bodySize int) {
	req.ResetBody()
	req.bodyStream = bodyStream
	req.Header.SetContentLength(bodySize)
}

func (req *Request) ConstructBodyStream(body *bytebufferpool.ByteBuffer, bodyStream io.Reader) {
	req.body = body
	req.bodyStream = bodyStream
}

// BodyWriter returns writer for populating request body.
func (req *Request) BodyWriter() io.Writer {
	req.w.r = req
	return &req.w
}

// PostArgs returns POST arguments.
func (req *Request) PostArgs() *Args {
	req.parsePostArgs()
	return &req.postArgs
}

func (req *Request) parsePostArgs() {
	if req.parsedPostArgs {
		return
	}
	req.parsedPostArgs = true

	if !bytes.HasPrefix(req.Header.ContentType(), bytestr.StrPostArgsContentType) {
		return
	}
	req.postArgs.ParseBytes(req.Body())
}

// BodyE returns request body.
func (req *Request) BodyE() ([]byte, error) {
	if req.bodyRaw != nil {
		return req.bodyRaw, nil
	}
	if req.IsBodyStream() {
		bodyBuf := req.BodyBuffer()
		bodyBuf.Reset()
		zw := network.NewWriter(bodyBuf)
		_, err := utils.CopyZeroAlloc(zw, req.bodyStream)
		req.CloseBodyStream() //nolint:errcheck
		if err != nil {
			return nil, err
		}
		return req.BodyBytes(), nil
	}
	if req.OnlyMultipartForm() {
		body, err := MarshalMultipartForm(req.multipartForm, req.multipartFormBoundary)
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	return req.BodyBytes(), nil
}

// Body returns request body.
// if get body failed, returns nil.
func (req *Request) Body() []byte {
	body, _ := req.BodyE()
	return body
}

// BodyWriteTo writes request body to w.
func (req *Request) BodyWriteTo(w io.Writer) error {
	if req.IsBodyStream() {
		zw := network.NewWriter(w)
		_, err := utils.CopyZeroAlloc(zw, req.bodyStream)
		req.CloseBodyStream() //nolint:errcheck
		return err
	}
	if req.OnlyMultipartForm() {
		return WriteMultipartForm(w, req.multipartForm, req.multipartFormBoundary)
	}
	_, err := w.Write(req.BodyBytes())
	return err
}

func (req *Request) CloseBodyStream() error {
	if req.bodyStream == nil {
		return nil
	}

	var err error
	if bsc, ok := req.bodyStream.(io.Closer); ok {
		err = bsc.Close()
	}
	req.bodyStream = nil
	return err
}

// URI returns request URI
func (req *Request) URI() *URI {
	req.ParseURI()
	return &req.uri
}

func (req *Request) ParseURI() {
	if req.parsedURI {
		return
	}
	req.parsedURI = true

	req.uri.parse(req.Header.Host(), req.Header.RequestURI(), req.isTLS)
}

// RemoveMultipartFormFiles removes multipart/form-data temporary files
// associated with the request.
func (req *Request) RemoveMultipartFormFiles() {
	if req.multipartForm != nil {
		// Do not check for error, since these files may be deleted or moved
		// to new places by user code.
		req.multipartForm.RemoveAll() //nolint:errcheck
		req.multipartForm = nil
	}
	req.multipartFormBoundary = ""
	req.multipartFiles = nil
	req.multipartFields = nil
}

func AddMultipartFormField(w *multipart.Writer, mf *MultipartField) error {
	partWriter, err := w.CreatePart(CreateMultipartHeader(mf.Param, mf.FileName, mf.ContentType))
	if err != nil {
		return err
	}

	_, err = io.Copy(partWriter, mf.Reader)
	return err
}

// Method returns request method
func (req *Request) Method() []byte {
	return req.Header.Method()
}

// Path returns request path
func (req *Request) Path() []byte {
	return req.URI().Path()
}

// QueryString returns request query
func (req *Request) QueryString() []byte {
	return req.URI().QueryString()
}

// SetOptions is used to set request options.
// These options can be used to do something in middlewares such as service discovery.
func (req *Request) SetOptions(opts ...config.RequestOption) {
	req.Options().Apply(opts)
}

// ConnectionClose returns true if 'Connection: close' header is set.
func (req *Request) ConnectionClose() bool {
	return req.Header.ConnectionClose()
}

// SetConnectionClose sets 'Connection: close' header.
func (req *Request) SetConnectionClose() {
	req.Header.SetConnectionClose(true)
}

func (req *Request) ResetWithoutConn() {
	req.Header.Reset()
	req.resetSkipHeaderAndConn()
	req.CloseBodyStream()

	req.options = nil
}

// AcquireRequest returns an empty Request instance from request pool.
//
// The returned Request instance may be passed to ReleaseRequest when it is
// no longer needed. This allows Request recycling, reduces GC pressure
// and usually improves performance.
func AcquireRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

// ReleaseRequest returns req acquired via AcquireRequest to request pool.
//
// It is forbidden accessing req and/or its members after returning
// it to request pool.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

// NewRequest makes a new Request given a method, URL, and
// optional body.
//
// # Method's default value is GET
//
// Url must contain fully qualified uri, i.e. with scheme and host,
// and http is assumed if scheme is omitted.
//
// Protocol version is always HTTP/1.1
//
// NewRequest just uses for unit-testing. Use AcquireRequest() in other cases.
func NewRequest(method, url string, body io.Reader) *Request {
	if method == "" {
		method = consts.MethodGet
	}

	req := new(Request)
	req.SetRequestURI(url)
	req.SetIsTLS(bytes.HasPrefix(bytesconv.S2b(url), bytestr.StrHTTPS))
	req.ParseURI()
	req.SetMethod(method)
	req.Header.SetHost(string(req.URI().Host()))
	req.Header.SetRequestURIBytes(req.URI().RequestURI())

	if !req.Header.IgnoreBody() {
		req.SetBodyStream(body, -1)
		switch v := req.BodyStream().(type) {
		case *bytes.Buffer:
			req.Header.SetContentLength(v.Len())
		case *bytes.Reader:
			req.Header.SetContentLength(v.Len())
		case *strings.Reader:
			req.Header.SetContentLength(v.Len())
		default:
		}
	}

	return req
}
