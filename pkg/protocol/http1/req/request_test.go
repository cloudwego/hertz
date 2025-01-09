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
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/url"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/compress"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/ext"
	"github.com/cloudwego/netpoll"
)

func TestRequestContinueReadBody(t *testing.T) {
	t.Parallel()
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)

	var r protocol.Request
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := ContinueReadBody(&r, zr, 0, true); err != nil {
		t.Fatalf("error when reading request body: %s", err)
	}
	body := r.Body()
	if string(body) != "abcde" {
		t.Fatalf("unexpected body %q. Expecting %q", body, "abcde")
	}

	tail, err := zr.Peek(zr.Len())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "f4343" {
		t.Fatalf("unexpected tail %q. Expecting %q", tail, "f4343")
	}
}

func TestRequestReadNoBody(t *testing.T) {
	t.Parallel()

	var r protocol.Request

	s := "GET / HTTP/1.1\r\n\r\n"

	zr := mock.NewZeroCopyReader(s)
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	r.SetHost("foobar")
	headerStr := r.Header.String()
	if strings.Contains(headerStr, "Content-Length: ") {
		t.Fatalf("unexpected Content-Length")
	}
}

func TestRequestRead(t *testing.T) {
	t.Parallel()

	var r protocol.Request

	s := "POST / HTTP/1.1\r\n\r\n"

	zr := mock.NewZeroCopyReader(s)
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	r.SetHost("foobar")
	headerStr := r.Header.String()
	if !strings.Contains(headerStr, "Content-Length: ") {
		t.Fatalf("should contain Content-Length")
	}
	cLen := r.Header.Peek(consts.HeaderContentLength)
	if string(cLen) != "0" {
		t.Fatalf("unexpected Content-Length: %s, Expecting 0", string(cLen))
	}
}

func TestRequestReadNoBodyStreaming(t *testing.T) {
	t.Parallel()

	var r protocol.Request
	r.Header.SetContentLength(-2)
	r.Header.SetMethod("GET")

	s := ""

	zr := mock.NewZeroCopyReader(s)
	if err := ContinueReadBodyStream(&r, zr, 2048, true); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	r.SetHost("foobar")
	headerStr := r.Header.String()
	if strings.Contains(headerStr, "Content-Length: ") {
		t.Fatalf("unexpected Content-Length")
	}
}

func TestRequestReadStreaming(t *testing.T) {
	t.Parallel()

	var r protocol.Request
	r.Header.SetContentLength(-2)
	r.Header.SetMethod("POST")

	s := ""

	zr := mock.NewZeroCopyReader(s)
	if err := ContinueReadBodyStream(&r, zr, 2048, true); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	r.SetHost("foobar")
	headerStr := r.Header.String()
	if !strings.Contains(headerStr, "Content-Length: ") {
		t.Fatalf("should contain Content-Length")
	}
	cLen := r.Header.Peek(consts.HeaderContentLength)
	if string(cLen) != "0" {
		t.Fatalf("unexpected Content-Length: %s, Expecting 0", string(cLen))
	}
}

func TestMethodAndPathAndQueryString(t *testing.T) {
	s := "PUT /foo/bar?query=1 HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)

	var r protocol.Request
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(r.RequestURI()) != "/foo/bar?query=1" {
		t.Fatalf("unexpected request uri %s. Expecting %s", r.RequestURI(), "/foo/bar?query=1")
	}
	if string(r.Method()) != "PUT" {
		t.Fatalf("unexpected method %s. Expecting %s", r.Header.Method(), "PUT")
	}

	if string(r.Path()) != "/foo/bar" {
		t.Fatalf("unexpected uri path %s. Expecting %s", r.URI().Path(), "/foo/bar")
	}
	if string(r.QueryString()) != "query=1" {
		t.Fatalf("unexpected query string %s. Expecting %s", r.URI().QueryString(), "query=1")
	}
}

func TestRequestSuccess(t *testing.T) {
	t.Parallel()

	// empty method, user-agent and body
	testRequestSuccess(t, "", "/foo/bar", "google.com", "", "", consts.MethodGet)

	// non-empty user-agent
	testRequestSuccess(t, consts.MethodGet, "/foo/bar", "google.com", "MSIE", "", consts.MethodGet)

	// non-empty method
	testRequestSuccess(t, consts.MethodHead, "/aaa", "fobar", "", "", consts.MethodHead)

	// POST method with body
	testRequestSuccess(t, consts.MethodPost, "/bbb", "aaa.com", "Chrome aaa", "post body", consts.MethodPost)

	// PUT method with body
	testRequestSuccess(t, consts.MethodPut, "/aa/bb", "a.com", "ome aaa", "put body", consts.MethodPut)

	// only host is set
	testRequestSuccess(t, "", "", "gooble.com", "", "", consts.MethodGet)

	// get with body
	testRequestSuccess(t, consts.MethodGet, "/foo/bar", "aaa.com", "", "foobar", consts.MethodGet)
}

func TestRequestMultipartFormBoundary(t *testing.T) {
	t.Parallel()

	testRequestMultipartFormBoundary(t, "POST / HTTP/1.1\r\nContent-Type: multipart/form-data; boundary=foobar\r\n\r\n", "foobar")

	// incorrect content-type
	testRequestMultipartFormBoundary(t, "POST / HTTP/1.1\r\nContent-Type: foo/bar\r\n\r\n", "")

	// empty boundary
	testRequestMultipartFormBoundary(t, "POST / HTTP/1.1\r\nContent-Type: multipart/form-data; boundary=\r\n\r\n", "")

	// missing boundary
	testRequestMultipartFormBoundary(t, "POST / HTTP/1.1\r\nContent-Type: multipart/form-data\r\n\r\n", "")

	// boundary after other content-type params
	testRequestMultipartFormBoundary(t, "POST / HTTP/1.1\r\nContent-Type: multipart/form-data;   foo=bar;   boundary=--aaabb  \r\n\r\n", "--aaabb")

	// quoted boundary
	testRequestMultipartFormBoundary(t, "POST / HTTP/1.1\r\nContent-Type: multipart/form-data; boundary=\"foobar\"\r\n\r\n", "foobar")

	var h protocol.RequestHeader
	h.SetMultipartFormBoundary("foobarbaz")
	b := h.MultipartFormBoundary()
	if string(b) != "foobarbaz" {
		t.Fatalf("unexpected boundary %q. Expecting %q", b, "foobarbaz")
	}
}

func testRequestSuccess(t *testing.T, method, requestURI, host, userAgent, body, expectedMethod string) {
	var req protocol.Request

	req.Header.SetMethod(method)
	req.Header.SetRequestURI(requestURI)
	req.Header.Set(consts.HeaderHost, host)
	req.Header.Set(consts.HeaderUserAgent, userAgent)
	req.SetBody([]byte(body))

	contentType := "foobar"
	if method == consts.MethodPost {
		req.Header.Set(consts.HeaderContentType, contentType)
	}

	w := &bytes.Buffer{}
	zw := netpoll.NewWriter(w)
	err := Write(&req, zw)
	if err != nil {
		t.Fatalf("Unexpected error when calling Write(): %s", err)
	}

	if err = zw.Flush(); err != nil {
		t.Fatalf("Unexpected error when flushing bufio.Writer: %s", err)
	}

	var req1 protocol.Request
	br := bufio.NewReader(w)
	zr := netpoll.NewReader(br)
	if err = Read(&req1, zr); err != nil {
		t.Fatalf("Unexpected error when calling Read(): %s", err)
	}
	if string(req1.Header.Method()) != expectedMethod {
		t.Fatalf("Unexpected method: %q. Expected %q", req1.Header.Method(), expectedMethod)
	}
	if len(requestURI) == 0 {
		requestURI = "/"
	}
	if string(req1.Header.RequestURI()) != requestURI {
		t.Fatalf("Unexpected RequestURI: %q. Expected %q", req1.Header.RequestURI(), requestURI)
	}
	if string(req1.Header.Peek(consts.HeaderHost)) != host {
		t.Fatalf("Unexpected host: %q. Expected %q", req1.Header.Peek(consts.HeaderHost), host)
	}
	if string(req1.Header.Peek(consts.HeaderUserAgent)) != userAgent {
		t.Fatalf("Unexpected user-agent: %q. Expected %q", req1.Header.Peek(consts.HeaderUserAgent), userAgent)
	}
	if !bytes.Equal(req1.Body(), []byte(body)) {
		t.Fatalf("Unexpected body: %q. Expected %q", req1.Body(), body)
	}

	if method == consts.MethodPost && string(req1.Header.Peek(consts.HeaderContentType)) != contentType {
		t.Fatalf("Unexpected content-type: %q. Expected %q", req1.Header.Peek(consts.HeaderContentType), contentType)
	}
}

func TestRequestWriteError(t *testing.T) {
	t.Parallel()

	// no host
	testRequestWriteError(t, "", "/foo/bar", "", "", "")
}

func TestRequestPostArgsSuccess(t *testing.T) {
	t.Parallel()

	var req protocol.Request

	testRequestPostArgsSuccess(t, &req, "POST / HTTP/1.1\r\nHost: aaa.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 0\r\n\r\n", 0, "foo=", "=")

	testRequestPostArgsSuccess(t, &req, "POST / HTTP/1.1\r\nHost: aaa.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 18\r\n\r\nfoo&b%20r=b+z=&qwe", 3, "foo=", "b r=b z=", "qwe=")
}

func testRequestPostArgsSuccess(t *testing.T, req *protocol.Request, s string, expectedArgsLen int, expectedArgs ...string) {
	r := bytes.NewBufferString(s)
	zr := netpoll.NewReader(r)
	err := Read(req, zr)
	if err != nil {
		t.Fatalf("Unexpected error when reading %q: %s", s, err)
	}

	args := req.PostArgs()
	if args.Len() != expectedArgsLen {
		t.Fatalf("Unexpected args len %d. Expected %d for %q", args.Len(), expectedArgsLen, s)
	}
	for _, x := range expectedArgs {
		tmp := strings.SplitN(x, "=", 2)
		k := tmp[0]
		v := tmp[1]
		vv := string(args.Peek(k))
		if vv != v {
			t.Fatalf("Unexpected value for key %q: %q. Expected %q for %q", k, vv, v, s)
		}
	}
}

func TestRequestPostArgsBodyStream(t *testing.T) {
	var req protocol.Request
	s := "POST / HTTP/1.1\r\nHost: aaa.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 8196\r\n\r\n"
	contentB := make([]byte, 8192)
	for i := 0; i < len(contentB); i++ {
		contentB[i] = 'a'
	}
	content := string(contentB)
	requestString := s + url.Values{"key": []string{content}}.Encode()

	r := bytes.NewBufferString(requestString)
	zr := netpoll.NewReader(r)
	if err := ReadHeader(&req.Header, zr); err != nil {
		t.Fatalf("Unexpected error when reading header %q: %s", s, err)
	}

	err := ReadBodyStream(&req, zr, 1024*4, false, false)
	if err != nil {
		t.Fatalf("Unexpected error when reading bodystream %q: %s", s, err)
	}
	if string(req.PostArgs().Peek("key")) != content {
		assert.DeepEqual(t, content, string(req.PostArgs().Peek("key")))
	}
}

func testRequestWriteError(t *testing.T, method, requestURI, host, userAgent, body string) {
	var req protocol.Request

	req.Header.SetMethod(method)
	req.Header.SetRequestURI(requestURI)
	req.Header.Set(consts.HeaderHost, host)
	req.Header.Set(consts.HeaderUserAgent, userAgent)
	req.SetBody([]byte(body))

	w := &bytebufferpool.ByteBuffer{}
	zw := netpoll.NewWriter(w)
	err := Write(&req, zw)
	if err == nil {
		t.Fatalf("Expecting error when writing request=%#v", &req)
	}
}

func TestChunkedUnexpectedEOF(t *testing.T) {
	reader := &mock.EOFReader{}

	_, err := ext.ReadBody(reader, -1, 0, nil)
	if err != io.ErrUnexpectedEOF {
		assert.DeepEqual(t, io.ErrUnexpectedEOF, err)
	}

	var pool bytebufferpool.Pool
	var req1 protocol.Request
	bs := ext.AcquireBodyStream(pool.Get(), reader, req1.Header.Trailer(), -1)
	byteSlice := make([]byte, 4096)
	_, err = bs.Read(byteSlice)
	if err != io.ErrUnexpectedEOF {
		assert.DeepEqual(t, io.ErrUnexpectedEOF, err)
	}
}

func TestReadBodyChunked(t *testing.T) {
	t.Parallel()

	// zero-size body
	testReadBodyChunked(t, 0)

	// small-size body
	testReadBodyChunked(t, 5)

	// medium-size body
	testReadBodyChunked(t, 43488)

	// big body
	testReadBodyChunked(t, 3*1024*1024)

	// smaller body after big one
	testReadBodyChunked(t, 12343)
}

func TestReadBodyFixedSize(t *testing.T) {
	t.Parallel()

	// zero-size body
	testReadBodyFixedSize(t, 0)

	// small-size body
	testReadBodyFixedSize(t, 3)

	// medium-size body
	testReadBodyFixedSize(t, 1024)

	// large-size body
	testReadBodyFixedSize(t, 1024*1024)

	// smaller body after big one
	testReadBodyFixedSize(t, 34345)
}

func TestRequestWriteRequestURINoHost(t *testing.T) {
	t.Parallel()

	var req protocol.Request
	req.Header.SetRequestURI("http://user:pass@google.com/foo/bar?baz=aaa")
	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	if err := Write(&req, zw); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var req1 protocol.Request
	br := bufio.NewReader(&w)
	zr := netpoll.NewReader(br)
	if err := Read(&req1, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(req1.Header.Host()) != "google.com" {
		t.Fatalf("unexpected host: %q. Expecting %q", req1.Header.Host(), "google.com")
	}
	if string(req.Header.RequestURI()) != "/foo/bar?baz=aaa" {
		t.Fatalf("unexpected requestURI: %q. Expecting %q", req.Header.RequestURI(), "/foo/bar?baz=aaa")
	}
	// authorization
	authorization := req.Header.Get(string(bytestr.StrAuthorization))
	author, err := base64.StdEncoding.DecodeString(authorization[len(bytestr.StrBasicSpace):])
	if err != nil {
		t.Fatalf("expecting error")
	}

	if string(author) != "user:pass" {
		t.Fatalf("unexpected Authorization: %q. Expecting %q", authorization, "user:pass")
	}

	// verify that Write returns error on non-absolute RequestURI
	req.Reset()
	req.Header.SetRequestURI("/foo/bar")
	w.Reset()
	if err := Write(&req, zw); err == nil {
		t.Fatalf("expecting error")
	}
}

func TestRequestWriteMultipartFile(t *testing.T) {
	t.Parallel()

	var req protocol.Request
	req.Header.SetHost("foobar.com")
	req.Header.SetMethod(consts.MethodPost)
	req.SetFileReader("filea", "filea.txt", bytes.NewReader([]byte("This is filea.")))
	req.SetMultipartField("fileb", "fileb.txt", "text/plain", bytes.NewReader([]byte("This is fileb.")))

	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	if err := Write(&req, zw); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var req1 protocol.Request
	zr := mock.NewZeroCopyReader(w.String())
	if err := Read(&req1, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	filea, err := req1.FormFile("filea")
	assert.Nil(t, err)
	assert.DeepEqual(t, "filea.txt", filea.Filename)
	fileb, err := req1.FormFile("fileb")
	assert.Nil(t, err)
	assert.DeepEqual(t, "fileb.txt", fileb.Filename)
}

func TestSetRequestBodyStreamChunked(t *testing.T) {
	t.Parallel()

	testSetRequestBodyStreamChunked(t, "", map[string]string{"Foo": "bar"})

	body := "foobar baz aaa bbb ccc"
	testSetRequestBodyStreamChunked(t, body, nil)

	body = string(mock.CreateFixedBody(10001))
	testSetRequestBodyStreamChunked(t, body, map[string]string{"Foo": "test", "Bar": "test"})
}

func TestSetRequestBodyStreamFixedSize(t *testing.T) {
	t.Parallel()

	testSetRequestBodyStream(t, "a")
	testSetRequestBodyStream(t, string(mock.CreateFixedBody(4097)))
	testSetRequestBodyStream(t, string(mock.CreateFixedBody(100500)))
}

func TestRequestHostFromRequestURI(t *testing.T) {
	t.Parallel()

	hExpected := "foobar.com"
	var req protocol.Request
	req.SetRequestURI("http://proxy-host:123/foobar?baz")
	req.SetHost(hExpected)
	h := bytesconv.B2s(req.Host())
	if h != hExpected {
		t.Fatalf("unexpected host set: %q. Expecting %q", h, hExpected)
	}
}

func TestRequestHostFromHeader(t *testing.T) {
	t.Parallel()

	hExpected := "foobar.com"
	var req protocol.Request
	req.Header.SetHost(hExpected)
	h := bytesconv.B2s(req.Host())
	if h != hExpected {
		t.Fatalf("unexpected host set: %q. Expecting %q", h, hExpected)
	}
}

func TestRequestContentTypeWithCharset(t *testing.T) {
	t.Parallel()

	expectedContentType := consts.MIMEApplicationHTMLFormUTF8
	expectedBody := "0123=56789"
	s := fmt.Sprintf("POST / HTTP/1.1\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n%s",
		expectedContentType, len(expectedBody), expectedBody)

	zr := mock.NewZeroCopyReader(s)
	var r protocol.Request
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	body := r.Body()
	if string(body) != expectedBody {
		t.Fatalf("unexpected body %q. Expecting %q", body, expectedBody)
	}
	ct := r.Header.ContentType()
	if string(ct) != expectedContentType {
		t.Fatalf("unexpected content-type %q. Expecting %q", ct, expectedContentType)
	}
	args := r.PostArgs()
	if args.Len() != 1 {
		t.Fatalf("unexpected number of POST args: %d. Expecting 1", args.Len())
	}
	av := args.Peek("0123")
	if string(av) != "56789" {
		t.Fatalf("unexpected POST arg value: %q. Expecting %q", av, "56789")
	}
}

func TestRequestBodyStreamMultipleBodyCalls(t *testing.T) {
	t.Parallel()

	var r protocol.Request

	s := "foobar baz abc"
	if r.IsBodyStream() {
		t.Fatalf("IsBodyStream must return false")
	}
	r.SetBodyStream(bytes.NewBufferString(s), len(s))
	if !r.IsBodyStream() {
		t.Fatalf("IsBodyStream must return true")
	}
	for i := 0; i < 10; i++ {
		body := r.Body()
		if string(body) != s {
			t.Fatalf("unexpected body %q. Expecting %q. iteration %d", body, s, i)
		}
	}
}

func TestRequestNoContentLength(t *testing.T) {
	t.Parallel()

	var r protocol.Request

	r.Header.SetMethod(consts.MethodHead)
	r.Header.SetHost("foobar")

	s := GetHTTP1Request(&r).String()
	if strings.Contains(s, "Content-Length: ") {
		t.Fatalf("unexpected content-length in HEAD request %q", s)
	}

	r.Header.SetMethod(consts.MethodPost)
	fmt.Fprintf(r.BodyWriter(), "foobar body")
	s = GetHTTP1Request(&r).String()
	if !strings.Contains(s, "Content-Length: ") {
		t.Fatalf("missing content-length header in non-GET request %q", s)
	}
}

func TestRequestReadGzippedBody(t *testing.T) {
	t.Parallel()

	var r protocol.Request

	bodyOriginal := "foo bar baz compress me better!"
	body := compress.AppendGzipBytes(nil, []byte(bodyOriginal))
	s := fmt.Sprintf("POST /foobar HTTP/1.1\r\nContent-Type: foo/bar\r\nContent-Encoding: gzip\r\nContent-Length: %d\r\n\r\n%s",
		len(body), body)
	zr := mock.NewZeroCopyReader(s)
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if string(r.Header.Peek(consts.HeaderContentEncoding)) != "gzip" {
		t.Fatalf("unexpected content-encoding: %q. Expecting %q", r.Header.Peek(consts.HeaderContentEncoding), "gzip")
	}
	if r.Header.ContentLength() != len(body) {
		t.Fatalf("unexpected content-length: %d. Expecting %d", r.Header.ContentLength(), len(body))
	}
	if string(r.Body()) != string(body) {
		t.Fatalf("unexpected body: %q. Expecting %q", r.Body(), body)
	}

	bodyGunzipped, err := compress.AppendGunzipBytes(nil, r.Body())
	if err != nil {
		t.Fatalf("unexpected error when uncompressing data: %s", err)
	}
	if string(bodyGunzipped) != bodyOriginal {
		t.Fatalf("unexpected uncompressed body %q. Expecting %q", bodyGunzipped, bodyOriginal)
	}
}

func TestRequestReadPostNoBody(t *testing.T) {
	t.Parallel()

	var r protocol.Request

	s := "POST /foo/bar HTTP/1.1\r\nContent-Type: aaa/bbb\r\n\r\naaaa"
	zr := mock.NewZeroCopyReader(s)
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if string(r.Header.RequestURI()) != "/foo/bar" {
		t.Fatalf("unexpected request uri %q. Expecting %q", r.Header.RequestURI(), "/foo/bar")
	}
	if string(r.Header.ContentType()) != "aaa/bbb" {
		t.Fatalf("unexpected content-type %q. Expecting %q", r.Header.ContentType(), "aaa/bbb")
	}
	if len(r.Body()) != 0 {
		t.Fatalf("unexpected body found %q. Expecting empty body", r.Body())
	}
	if r.Header.ContentLength() != 0 {
		t.Fatalf("unexpected content-length: %d. Expecting 0", r.Header.ContentLength())
	}

	tail, err := ioutil.ReadAll(zr)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "aaaa" {
		t.Fatalf("unexpected tail %q. Expecting %q", tail, "aaaa")
	}
}

func TestRequestContinueReadBodyDisablePrereadMultipartForm(t *testing.T) {
	t.Parallel()

	var w bytes.Buffer
	mw := multipart.NewWriter(&w)
	for i := 0; i < 10; i++ {
		k := fmt.Sprintf("key_%d", i)
		v := fmt.Sprintf("value_%d", i)
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}
	boundary := mw.Boundary()
	if err := mw.Close(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	formData := w.Bytes()

	s := fmt.Sprintf("POST / HTTP/1.1\r\nHost: aaa\r\nContent-Type: multipart/form-data; boundary=%s\r\nContent-Length: %d\r\n\r\n%s",
		boundary, len(formData), formData)
	zr := mock.NewZeroCopyReader(s)

	var r protocol.Request

	if err := ReadHeader(&r.Header, zr); err != nil {
		t.Fatalf("unexpected error reading headers: %s", err)
	}

	if err := ReadLimitBody(&r, zr, 10000, false, false); err != nil {
		t.Fatalf("unexpected error reading body: %s", err)
	}

	if r.HasMultipartForm() {
		t.Fatalf("The multipartForm of the Request must be nil")
	}

	if string(formData) != string(r.Body()) {
		t.Fatalf("The body given must equal the body in the Request")
	}
}

func TestRequestReadLimitBody(t *testing.T) {
	t.Parallel()

	testRequestReadLimitBodyReadOnly(t, "POST /foo HTTP/1.1\r\nHost: aaa.com\r\nContent-Length: 9\r\nContent-Type: aaa\r\n\r\n123456789")
	// request with content-length
	testRequestReadLimitBodySuccess(t, "POST /foo HTTP/1.1\r\nHost: aaa.com\r\nContent-Length: 9\r\nContent-Type: aaa\r\n\r\n123456789", 9)
	testRequestReadLimitBodySuccess(t, "POST /foo HTTP/1.1\r\nHost: aaa.com\r\nContent-Length: 9\r\nContent-Type: aaa\r\n\r\n123456789", 92)
	testRequestReadLimitBodyError(t, "POST /foo HTTP/1.1\r\nHost: aaa.com\r\nContent-Length: 9\r\nContent-Type: aaa\r\n\r\n123456789", 5)

	// chunked request
	testRequestReadLimitBodySuccess(t, "POST /a HTTP/1.1\r\nHost: a.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 9)
	testRequestReadLimitBodySuccess(t, "POST /a HTTP/1.1\r\nHost: a.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 999)
	testRequestReadLimitBodyError(t, "POST /a HTTP/1.1\r\nHost: a.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 8)
}

func testRequestReadLimitBodyReadOnly(t *testing.T, s string) {
	var req protocol.Request
	zr := mock.NewZeroCopyReader(s)

	ReadHeader(&req.Header, zr)
	if err := ReadLimitBody(&req, zr, 10, true, false); err == nil {
		t.Fatalf("expected error: %s", errGetOnly.Error())
	}
}

func TestRequestString(t *testing.T) {
	t.Parallel()

	var r protocol.Request
	r.SetRequestURI("http://foobar.com/aaa")
	s := GetHTTP1Request(&r).String()
	expectedS := "GET /aaa HTTP/1.1\r\nHost: foobar.com\r\n\r\n"
	if s != expectedS {
		t.Fatalf("unexpected request: %q. Expecting %q", s, expectedS)
	}
}

func TestRequestReadChunked(t *testing.T) {
	t.Parallel()

	var req protocol.Request

	s := "POST /foo HTTP/1.1\r\nHost: google.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n3\r\nabc\r\n5\r\n12345\r\n0\r\n\r\nTrail: test\r\n\r\n"
	zr := netpoll.NewReader(bytes.NewBufferString(s))
	err := Read(&req, zr)
	if err != nil {
		t.Fatalf("Unexpected error when reading chunked request: %s", err)
	}
	expectedBody := "abc12345"
	if string(req.Body()) != expectedBody {
		t.Fatalf("Unexpected body %q. Expected %q", req.Body(), expectedBody)
	}
	verifyRequestHeader(t, &req.Header, 8, "/foo", "google.com", "", "aa/bb")
	verifyTrailer(t, zr, map[string]string{"Trail": "test"})
}

func verifyTrailer(t *testing.T, r network.Reader, exceptedTrailers map[string]string) {
	trailer := protocol.Trailer{}
	keys := make([]string, 0, len(exceptedTrailers))
	for k := range exceptedTrailers {
		keys = append(keys, k)
	}
	trailer.SetTrailers([]byte(strings.Join(keys, ", ")))
	err := ext.ReadTrailer(&trailer, r)
	if err == io.EOF && exceptedTrailers == nil {
		return
	}
	if err != nil {
		t.Fatalf("Cannot read trailer: %v", err)
	}

	for k, v := range exceptedTrailers {
		got := trailer.Peek(k)
		if !bytes.Equal(got, []byte(v)) {
			t.Fatalf("Unexpected trailer %q. Expected %q. Got %q", k, v, got)
		}
	}
}

func TestRequestChunkedWhitespace(t *testing.T) {
	t.Parallel()

	var req protocol.Request

	s := "POST /foo HTTP/1.1\r\nHost: google.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n3  \r\nabc\r\n0\r\n\r\n"
	zr := mock.NewZeroCopyReader(s)

	err := Read(&req, zr)
	if err != nil {
		t.Fatalf("Unexpected error when reading chunked request: %s", err)
	}
	expectedBody := "abc"
	if string(req.Body()) != expectedBody {
		t.Fatalf("Unexpected body %q. Expected %q", req.Body(), expectedBody)
	}
}

func testRequestReadLimitBodyError(t *testing.T, s string, maxBodySize int) {
	var req protocol.Request
	zr := mock.NewZeroCopyReader(s)

	err := ReadHeaderAndLimitBody(&req, zr, maxBodySize)
	if err == nil {
		t.Fatalf("expecting error. s=%q, maxBodySize=%d", s, maxBodySize)
	}
	if !errors.Is(err, errs.ErrBodyTooLarge) {
		t.Fatalf("unexpected error: %s. Expecting %s. s=%q, maxBodySize=%d", err, errBodyTooLarge, s, maxBodySize)
	}
}

func testRequestReadLimitBodySuccess(t *testing.T, s string, maxBodySize int) {
	var req protocol.Request
	zr := mock.NewZeroCopyReader(s)
	if err := ReadHeaderAndLimitBody(&req, zr, maxBodySize); err != nil {
		t.Fatalf("unexpected error: %s. s=%q, maxBodySize=%d", err, s, maxBodySize)
	}
}

func testSetRequestBodyStream(t *testing.T, body string) {
	var req protocol.Request
	req.Header.SetHost("foobar.com")
	req.Header.SetMethod(consts.MethodPost)

	bodySize := len(body)
	if req.IsBodyStream() {
		t.Fatalf("IsBodyStream must return false")
	}
	req.SetBodyStream(bytes.NewBufferString(body), bodySize)
	if !req.IsBodyStream() {
		t.Fatalf("IsBodyStream must return true")
	}

	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	if err := Write(&req, zw); err != nil {
		t.Fatalf("unexpected error when writing request: %s. body=%q", err, body)
	}

	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error when flushing request: %s. body=%q", err, body)
	}

	var req1 protocol.Request
	br := bufio.NewReader(&w)
	zr := netpoll.NewReader(br)
	if err := Read(&req1, zr); err != nil {
		t.Fatalf("unexpected error when reading request: %s. body=%q", err, body)
	}
	if string(req1.Body()) != body {
		fmt.Println(string(req1.Body()))
		fmt.Println(body)
		t.Fatalf("unexpected body %q. Expecting %q", req1.Body(), body)
	}
}

func testSetRequestBodyStreamChunked(t *testing.T, body string, trailer map[string]string) {
	var req protocol.Request
	req.Header.SetHost("foobar.com")
	req.Header.SetMethod(consts.MethodPost)

	if req.IsBodyStream() {
		t.Fatalf("IsBodyStream must return false")
	}
	req.SetBodyStream(bytes.NewBufferString(body), -1)
	if !req.IsBodyStream() {
		t.Fatalf("IsBodyStream must return true")
	}

	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	for k, v := range trailer {
		err := req.Header.Trailer().Add(k, v)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if err := Write(&req, zw); err != nil {
		t.Fatalf("unexpected error when writing request: %v, body=%q", err, body)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error when flushing request: %v, body=%q", err, body)
	}

	var req1 protocol.Request
	br := bufio.NewReader(&w)
	zr := netpoll.NewReader(br)
	if err := Read(&req1, zr); err != nil {
		t.Fatalf("unexpected error when reading request: %v. body=%q", err, body)
	}
	if string(req1.Body()) != body {
		t.Fatalf("unexpected body %q. Expecting %q", req1.Body(), body)
	}
	for k, v := range trailer {
		r := req.Header.Trailer().Peek(k)
		if string(r) != v {
			t.Fatalf("unexpected trailer %q. Expecting %q. Got %q", k, v, r)
		}
	}
}

func TestRequestMultipartForm(t *testing.T) {
	t.Parallel()

	var w bytes.Buffer
	mw := multipart.NewWriter(&w)
	for i := 0; i < 10; i++ {
		k := fmt.Sprintf("key_%d", i)
		v := fmt.Sprintf("value_%d", i)
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}
	boundary := mw.Boundary()
	if err := mw.Close(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	formData := w.Bytes()
	for i := 0; i < 5; i++ {
		formData = testRequestMultipartForm(t, boundary, formData, 10)
		testRequestMultipartFormNotPreParse(t, boundary, formData, 10)
	}

	// verify request unmarshalling / marshalling
	s := "POST / HTTP/1.1\r\nHost: aaa\r\nContent-Type: multipart/form-data; boundary=foobar\r\nContent-Length: 213\r\n\r\n--foobar\r\nContent-Disposition: form-data; name=\"key_0\"\r\n\r\nvalue_0\r\n--foobar\r\nContent-Disposition: form-data; name=\"key_1\"\r\n\r\nvalue_1\r\n--foobar\r\nContent-Disposition: form-data; name=\"key_2\"\r\n\r\nvalue_2\r\n--foobar--\r\n"
	var r protocol.Request
	zr := mock.NewZeroCopyReader(s)
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	s = GetHTTP1Request(&r).String()
	zr = mock.NewZeroCopyReader(s)
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	testRequestMultipartForm(t, "foobar", r.Body(), 3)
}

func testRequestMultipartForm(t *testing.T, boundary string, formData []byte, partsCount int) []byte {
	s := fmt.Sprintf("POST / HTTP/1.1\r\nHost: aaa\r\nContent-Type: multipart/form-data; boundary=%s\r\nContent-Length: %d\r\n\r\n%s",
		boundary, len(formData), formData)
	var req protocol.Request

	zr := mock.NewZeroCopyReader(s)
	if err := Read(&req, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	f, err := req.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer req.RemoveMultipartFormFiles()

	if len(f.File) > 0 {
		t.Fatalf("unexpected files found in the multipart form: %d", len(f.File))
	}

	if len(f.Value) != partsCount {
		t.Fatalf("unexpected number of values found: %d. Expecting %d", len(f.Value), partsCount)
	}

	for k, vv := range f.Value {
		if len(vv) != 1 {
			t.Fatalf("unexpected number of values found for key=%q: %d. Expecting 1", k, len(vv))
		}
		if !strings.HasPrefix(k, "key_") {
			t.Fatalf("unexpected key prefix=%q. Expecting %q", k, "key_")
		}
		v := vv[0]
		if !strings.HasPrefix(v, "value_") {
			t.Fatalf("unexpected value prefix=%q. expecting %q", v, "value_")
		}
		if k[len("key_"):] != v[len("value_"):] {
			t.Fatalf("key and value suffixes don't match: %q vs %q", k, v)
		}
	}

	return req.Body()
}

func testRequestMultipartFormNotPreParse(t *testing.T, boundary string, formData []byte, partsCount int) []byte {
	s := fmt.Sprintf("POST / HTTP/1.1\r\nHost: aaa\r\nContent-Type: multipart/form-data; boundary=%s\r\nContent-Length: %d\r\n\r\n%s",
		boundary, len(formData), formData)
	var req protocol.Request

	zr := mock.NewZeroCopyReader(s)
	if err := Read(&req, zr, false); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	f, err := req.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer req.RemoveMultipartFormFiles()

	if len(f.File) > 0 {
		t.Fatalf("unexpected files found in the multipart form: %d", len(f.File))
	}

	if len(f.Value) != partsCount {
		t.Fatalf("unexpected number of values found: %d. Expecting %d", len(f.Value), partsCount)
	}

	for k, vv := range f.Value {
		if len(vv) != 1 {
			t.Fatalf("unexpected number of values found for key=%q: %d. Expecting 1", k, len(vv))
		}
		if !strings.HasPrefix(k, "key_") {
			t.Fatalf("unexpected key prefix=%q. Expecting %q", k, "key_")
		}
		v := vv[0]
		if !strings.HasPrefix(v, "value_") {
			t.Fatalf("unexpected value prefix=%q. expecting %q", v, "value_")
		}
		if k[len("key_"):] != v[len("value_"):] {
			t.Fatalf("key and value suffixes don't match: %q vs %q", k, v)
		}
	}

	return req.Body()
}

func testReadBodyChunked(t *testing.T, bodySize int) {
	body := mock.CreateFixedBody(bodySize)
	expectedTrailer := map[string]string{"Foo": "chunked shit"}
	chunkedBody := mock.CreateChunkedBody(body, expectedTrailer, true)

	zr := mock.NewZeroCopyReader(string(chunkedBody))

	// p,_ := mr.Next(3687)
	b, err := ext.ReadBody(zr, -1, 0, nil)
	if err != nil {
		t.Fatalf("Unexpected error for bodySize=%d: %s. body=%q, chunkedBody=%q", bodySize, err, body, chunkedBody)
	}
	if !bytes.Equal(b, body) {
		t.Fatalf("Unexpected response read for bodySize=%d: %q. Expected %q. chunkedBody=%q", bodySize, b, body, chunkedBody)
	}
	verifyTrailer(t, zr, expectedTrailer)
}

func testReadBodyFixedSize(t *testing.T, bodySize int) {
	body := mock.CreateFixedBody(bodySize)

	zr := mock.NewZeroCopyReader(string(body))
	b, err := ext.ReadBody(zr, bodySize, 0, nil)
	if err != nil {
		t.Fatalf("Unexpected error in ReadResponseBody(%d): %s", bodySize, err)
	}
	if !bytes.Equal(b, body) {
		t.Fatalf("Unexpected response read for bodySize=%d: %q. Expected %q", bodySize, b, body)
	}
	verifyTrailer(t, zr, nil)
}

func TestRequestFormFile(t *testing.T) {
	t.Parallel()

	s := `POST /upload HTTP/1.1
Host: localhost:10000
Content-Length: 521
Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg

------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f1"

value1
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="fileaaa"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`

	mr := mock.NewZeroCopyReader(s)

	var r protocol.Request
	if err := Read(&r, mr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	tail, err := ioutil.ReadAll(mr)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "tailfoobar" {
		t.Fatalf("unexpected tail %q. Expecting %q", tail, "tailfoobar")
	}
	fh, err := r.FormFile("fileaaa")
	if err != nil {
		t.Fatalf("TestRequestFormFile error: %#v", err.Error())
	}
	if fh == nil {
		t.Fatalf("fh unexpected nil")
	}
}

func TestRequest_ContinueReadBodyStream(t *testing.T) {
	// small body
	genBody := "abcdef4343"
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\n"
	testContinueReadBodyStream(t, s, genBody, 10, 5, 0, 5)
	testContinueReadBodyStream(t, s, genBody, 1, 5, 0, 0)

	// big body (> 8193)
	s1 := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 9216\r\nContent-Type: foo/bar\r\n\r\n"
	genBody = strings.Repeat("1", 9*1024)
	testContinueReadBodyStream(t, s1, genBody, 10*1024, 5*1024, 4*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 10*1024, 1*1024, 8*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 10*1024, 9*1024, 0*1024, 0)

	// normal stream
	testContinueReadBodyStream(t, s1, genBody, 1*1024, 5*1024, 4*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 1*1024, 1*1024, 8*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 1*1024, 9*1024, 0*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 5, 5*1024, 4*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 5, 1*1024, 8*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 5, 9*1024, 0, 0)

	// critical point
	testContinueReadBodyStream(t, s1, genBody, 8*1024+1, 5*1024, 4*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 8*1024+1, 1*1024, 8*1024, 0)
	testContinueReadBodyStream(t, s1, genBody, 8*1024+1, 9*1024, 0*1024, 0)

	// chunked body
	s2 := "POST /foo HTTP/1.1\r\nHost: google.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n3\r\nabc\r\n5\r\n12345\r\n0\r\n\r\ntrail"
	testContinueReadBodyStream(t, s2, "", 10*1024, 3, 5, 5)
	s3 := "POST /foo HTTP/1.1\r\nHost: google.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n3\r\nabc\r\n5\r\n12345\r\n0\r\n\r\n"
	testContinueReadBodyStream(t, s3, "", 10*1024, 3, 5, 0)
}

func TestRequest_Chunked(t *testing.T) {
	t.Parallel()
	s4 := "POST /foo HTTP/1.1\r\nHost: google.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n5\r\n12345\r\n0\r\n\r\n"
	testReadChunked(t, s4, "", 3, 2)

	s5 := "POST /foo HTTP/1.1\r\nHost: google.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n5\r\n12345\r\n3\r\n1230\r\n\r\n"
	testReadChunked(t, s5, "", 3, 5)
}

func TestRequest_ReadIncompleteStream(t *testing.T) {
	t.Parallel()
	// small body
	genBody := "abcdef4343"
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 100\r\nContent-Type: foo/bar\r\n\r\n"
	testReadIncompleteStream(t, s, genBody)

	// big body (> 8193)
	s1 := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 10000\r\nContent-Type: foo/bar\r\n\r\n"
	genBody = strings.Repeat("1", 9*1024)
	testReadIncompleteStream(t, s1, genBody)
}

func testReadIncompleteStream(t *testing.T, header, body string) {
	mr := mock.NewZeroCopyReader(header + body)
	var r protocol.Request
	if err := ReadHeader(&r.Header, mr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := ContinueReadBodyStream(&r, mr, 1, true); err != nil {
		t.Fatalf("error when reading request body stream: %s", err)
	}
	readBody, err := ioutil.ReadAll(r.BodyStream())
	if !bytes.Equal(readBody, []byte(body)) || len(readBody) != len(body) {
		t.Fatalf("readBody is not equal to the rawBody: %b(len: %d)", readBody, len(readBody))
	}
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("error should be io.ErrUnexpectedEOF, but got: %s", err)
	}
}

func testReadChunked(t *testing.T, header, body string, firstRead, leftBytes int) {
	mr := mock.NewZeroCopyReader(header + body)

	var r protocol.Request
	if err := ReadHeader(&r.Header, mr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := ContinueReadBodyStream(&r, mr, 2048, true); err != nil {
		t.Fatalf("error when reading request body stream: %s", err)
	}
	if r.Header.ContentLength() >= 0 {
		t.Fatalf("expect a chunked body")
	}
	streamRead := make([]byte, firstRead)
	fr, err := r.BodyStream().Read(streamRead)
	if err != nil {
		t.Fatalf("read stream error=%v", err)
	}
	if fr != firstRead {
		t.Fatalf("should read %d from stream body, but got %d", streamRead, fr)
	}
	leftB, _ := ioutil.ReadAll(r.BodyStream())
	if len(leftB) != leftBytes {
		t.Fatalf("should left %d bytes from stream body, but left %d", leftBytes, len(leftB))
	}
}

func testContinueReadBodyStream(t *testing.T, header, body string, maxBodySize, firstRead, leftBytes, bytesLeftInReader int) {
	mr := mock.NewZeroCopyReader(header + body)
	var r protocol.Request
	if err := ReadHeader(&r.Header, mr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := ContinueReadBodyStream(&r, mr, maxBodySize, true); err != nil {
		t.Fatalf("error when reading request body stream: %s", err)
	}
	fRead := firstRead
	streamRead := make([]byte, fRead)
	sR, _ := r.BodyStream().Read(streamRead)

	if sR != firstRead {
		t.Fatalf("should read %d from stream body, but got %d", firstRead, sR)
	}

	leftB, _ := ioutil.ReadAll(r.BodyStream())
	if len(leftB) != leftBytes {
		t.Fatalf("should left %d bytes from stream body, but left %d", leftBytes, len(leftB))
	}
	if r.Header.ContentLength() > 0 {
		gotBody := append(streamRead, leftB...)
		if !bytes.Equal([]byte(body[:r.Header.ContentLength()]), gotBody) {
			t.Fatalf("body read from stream is not equal to the origin. Got: %s", gotBody)
		}
	}

	left, _ := mr.Peek(mr.Len())

	if len(left) != bytesLeftInReader {
		fmt.Printf("##########header:%s,body:%s,%d:max,first:%d,left:%d,leftin:%d\n", header, body, maxBodySize, firstRead, leftBytes, bytesLeftInReader)
		fmt.Printf("##########left: %s\n", left)
		t.Fatalf("should left %d bytes in original reader. got %q", bytesLeftInReader, len(left))
	}
}

func verifyRequestHeader(t *testing.T, h *protocol.RequestHeader, expectedContentLength int,
	expectedRequestURI, expectedHost, expectedReferer, expectedContentType string,
) {
	if h.ContentLength() != expectedContentLength {
		t.Fatalf("Unexpected Content-Length %d. Expected %d", h.ContentLength(), expectedContentLength)
	}
	if string(h.RequestURI()) != expectedRequestURI {
		t.Fatalf("Unexpected RequestURI %q. Expected %q", h.RequestURI(), expectedRequestURI)
	}
	if string(h.Peek(consts.HeaderHost)) != expectedHost {
		t.Fatalf("Unexpected host %q. Expected %q", h.Peek(consts.HeaderHost), expectedHost)
	}
	if string(h.Peek(consts.HeaderReferer)) != expectedReferer {
		t.Fatalf("Unexpected referer %q. Expected %q", h.Peek(consts.HeaderReferer), expectedReferer)
	}
	if string(h.Peek(consts.HeaderContentType)) != expectedContentType {
		t.Fatalf("Unexpected content-type %q. Expected %q", h.Peek(consts.HeaderContentType), expectedContentType)
	}
}

func TestRequestReadMultipartFormWithFile(t *testing.T) {
	t.Parallel()

	s := `POST /upload HTTP/1.1
Host: localhost:10000
Content-Length: 521
Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg

------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f1"

value1
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="fileaaa"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`

	mr := mock.NewZeroCopyReader(s)

	var r protocol.Request
	if err := Read(&r, mr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	tail, err := ioutil.ReadAll(mr)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "tailfoobar" {
		t.Fatalf("unexpected tail %q. Expecting %q", tail, "tailfoobar")
	}

	f, err := r.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer r.RemoveMultipartFormFiles()

	// verify values
	if len(f.Value) != 1 {
		t.Fatalf("unexpected number of values in multipart form: %d. Expecting 1", len(f.Value))
	}
	for k, vv := range f.Value {
		if k != "f1" {
			t.Fatalf("unexpected value name %q. Expecting %q", k, "f1")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v != "value1" {
			t.Fatalf("unexpected value %q. Expecting %q", v, "value1")
		}
	}

	// verify files
	if len(f.File) != 1 {
		t.Fatalf("unexpected number of file values in multipart form: %d. Expecting 1", len(f.File))
	}
	for k, vv := range f.File {
		if k != "fileaaa" {
			t.Fatalf("unexpected file value name %q. Expecting %q", k, "fileaaa")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of file values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v.Filename != "TODO" {
			t.Fatalf("unexpected filename %q. Expecting %q", v.Filename, "TODO")
		}
		ct := v.Header.Get("Content-Type")
		if ct != "application/octet-stream" {
			t.Fatalf("unexpected content-type %q. Expecting %q", ct, "application/octet-stream")
		}
	}
}

func testRequestMultipartFormBoundary(t *testing.T, s, boundary string) {
	var h protocol.RequestHeader
	zr := mock.NewZeroCopyReader(s)

	if err := ReadHeader(&h, zr); err != nil {
		t.Fatalf("unexpected error: %s. s=%q, boundary=%q", err, s, boundary)
	}

	b := h.MultipartFormBoundary()
	if string(b) != boundary {
		t.Fatalf("unexpected boundary %q. Expecting %q. s=%q", b, boundary, s)
	}
}

func TestStreamNotEnoughData(t *testing.T) {
	req := protocol.AcquireRequest()
	req.Header.SetContentLength(1 << 16)
	conn := mock.NewStreamConn()
	const maxBodySize = 4 * 1024 * 1024
	err := ContinueReadBodyStream(req, conn, maxBodySize)
	assert.Nil(t, err)
	err = ext.ReleaseBodyStream(req.BodyStream())
	assert.Nil(t, err)
	assert.DeepEqual(t, 0, len(conn.Data))
	assert.DeepEqual(t, true, conn.HasReleased)
}

func TestRequestBodyStreamWithTrailer(t *testing.T) {
	t.Parallel()

	testRequestBodyStreamWithTrailer(t, []byte("test"), false)
	testRequestBodyStreamWithTrailer(t, mock.CreateFixedBody(4097), false)
	testRequestBodyStreamWithTrailer(t, mock.CreateFixedBody(105000), false)
}

func testRequestBodyStreamWithTrailer(t *testing.T, body []byte, disableNormalizing bool) {
	expectedTrailer := map[string]string{
		"foo": "testfoo",
		"bar": "testbar",
	}

	var req1 protocol.Request
	if disableNormalizing {
		req1.Header.DisableNormalizing()
	}
	req1.SetHost("google.com")
	req1.SetBodyStream(bytes.NewBuffer(body), -1)
	for k, v := range expectedTrailer {
		err := req1.Header.Trailer().Set(k, v)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	w := &bytes.Buffer{}
	zw := netpoll.NewWriter(w)
	if err := Write(&req1, zw); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var req2 protocol.Request
	if disableNormalizing {
		req2.Header.DisableNormalizing()
	}

	br := netpoll.NewReader(w)
	if err := Read(&req2, br); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	reqBody := req2.Body()
	if !bytes.Equal(reqBody, body) {
		t.Fatalf("unexpected body: %q. Excepting %q", reqBody, body)
	}

	for k, v := range expectedTrailer {
		kBytes := []byte(k)
		utils.NormalizeHeaderKey(kBytes, disableNormalizing)
		r := req2.Header.Trailer().Peek(k)
		if string(r) != v {
			t.Fatalf("unexpected trailer header %q: %q. Expecting %s", kBytes, r, v)
		}
	}
}
