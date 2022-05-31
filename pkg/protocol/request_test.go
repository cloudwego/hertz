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
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/compress"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

type errorReader struct{}

func (er errorReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("dummy!")
}

func TestMultiForm(t *testing.T) {
	var r Request
	// r.Header.Set()
	_, err := r.MultipartForm()
	fmt.Println(err)
}

func TestRequestKnownSizeStreamMultipartFormWithFile(t *testing.T) {
	t.Parallel()

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
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
	mr := strings.NewReader(s)
	r := NewRequest("POST", "/upload", mr)
	r.Header.SetContentLength(521)
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	f, err := r.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer r.RemoveMultipartFormFiles()

	// verify tail
	tail, err := ioutil.ReadAll(mr)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "tailfoobar" {
		t.Fatalf("unexpected tail %q. Expecting %q", tail, "tailfoobar")
	}

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

func TestRequestUnknownSizeStreamMultipartFormWithFile(t *testing.T) {
	t.Parallel()

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
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
	mr := strings.NewReader(s)
	r := NewRequest("POST", "/upload", mr)
	r.Header.SetContentLength(-1)
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	f, err := r.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer r.RemoveMultipartFormFiles()

	// all data must be consumed if the content length is unknown
	tail, err := ioutil.ReadAll(mr)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "" {
		t.Fatalf("unexpected tail %q. Expecting empty string", tail)
	}

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

func TestRequestStreamMultipartFormWithFileGzip(t *testing.T) {
	t.Parallel()

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
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

	ns := compress.AppendGzipBytes(nil, []byte(s))

	mr := bytes.NewBuffer(ns)
	r := NewRequest("POST", "/upload", mr)
	r.Header.Set("Content-Encoding", "gzip")
	r.Header.SetContentLength(len(s))
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

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

func TestRequestBodyWriteToPlain(t *testing.T) {
	t.Parallel()

	var r Request

	expectedS := "foobarbaz"
	r.AppendBodyString(expectedS)

	testBodyWriteTo(t, &r, expectedS, true)
}

func TestRequestBodyWriteToMultipart(t *testing.T) {
	t.Parallel()

	expectedS := "--foobar\r\nContent-Disposition: form-data; name=\"key_0\"\r\n\r\nvalue_0\r\n--foobar--\r\n"

	var r Request
	SetMultipartFormWithBoundary(&r, &multipart.Form{Value: map[string][]string{"key_0": {"value_0"}}}, "foobar")

	testBodyWriteTo(t, &r, expectedS, true)
}

func TestNewRequest(t *testing.T) {
	// get
	req := NewRequest("GET", "http://www.google.com/hi", bytes.NewReader([]byte("hello")))
	assert.NotNil(t, req)
	assert.DeepEqual(t, "GET /hi HTTP/1.1\r\nHost: www.google.com\r\n\r\n", string(req.Header.Header()))
	assert.Nil(t, req.Body())

	// post + bytes reader
	req = NewRequest("POST", "http://www.google.com/hi", bytes.NewReader([]byte("hello")))
	assert.NotNil(t, req)
	assert.DeepEqual(t, "POST /hi HTTP/1.1\r\nHost: www.google.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 5\r\n\r\n", string(req.Header.Header()))
	assert.DeepEqual(t, "hello", string(req.Body()))

	// post + string reader
	req = NewRequest("POST", "http://www.google.com/hi", strings.NewReader("hello world"))
	assert.NotNil(t, req)
	assert.DeepEqual(t, "POST /hi HTTP/1.1\r\nHost: www.google.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 11\r\n\r\n", string(req.Header.Header()))
	assert.DeepEqual(t, "hello world", string(req.Body()))

	// post + bytes buffer
	req = NewRequest("POST", "http://www.google.com/hi", bytes.NewBuffer([]byte("hello hertz!")))
	assert.NotNil(t, req)
	assert.DeepEqual(t, "POST /hi HTTP/1.1\r\nHost: www.google.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 12\r\n\r\n", string(req.Header.Header()))
	assert.DeepEqual(t, "hello hertz!", string(req.Body()))

	// empty method
	req = NewRequest("", "/", bytes.NewBufferString(""))
	assert.DeepEqual(t, "GET", string(req.Method()))
	// unstandard method
	req = NewRequest("DUMMY", "/", bytes.NewBufferString(""))
	assert.DeepEqual(t, "DUMMY", string(req.Method()))

	// empty body
	req = NewRequest("GET", "/", nil)
	assert.NotNil(t, req)
	// wrong body
	req = NewRequest("POST", "/", errorReader{})
	_, err := req.BodyE()
	assert.DeepEqual(t, err.Error(), "dummy!")
	req = NewRequest("POST", "/", errorReader{})
	body := req.Body()
	assert.Nil(t, body)

	// GET RequestURI
	req = NewRequest("GET", "http://www.google.com/hi?a=1&b=2", nil)
	assert.DeepEqual(t, "/hi?a=1&b=2", string(req.RequestURI()))

	// POST RequestURI
	req = NewRequest("POST", "http://www.google.com/hi?a=1&b=2", nil)
	assert.DeepEqual(t, "/hi?a=1&b=2", string(req.RequestURI()))

	// nil-interface body
	assert.Panic(t, func() {
		fake := func() *errorReader {
			return nil
		}
		req = NewRequest("POST", "/", fake())
		req.Body()
	})
}

func TestRequestResetBody(t *testing.T) {
	req := Request{}
	req.BodyBuffer()
	assert.NotNil(t, req.body)
	req.maxKeepBodySize = math.MaxUint32
	req.ResetBody()
	assert.NotNil(t, req.body)
	req.maxKeepBodySize = -1
	req.ResetBody()
	assert.Nil(t, req.body)
}

func TestRequestMayContinue(t *testing.T) {
	t.Parallel()

	var r Request
	if r.MayContinue() {
		t.Fatalf("MayContinue on empty request must return false")
	}

	r.Header.Set("Expect", "123sdfds")
	if r.MayContinue() {
		t.Fatalf("MayContinue on invalid Expect header must return false")
	}

	r.Header.Set("Expect", "100-continue")
	if !r.MayContinue() {
		t.Fatalf("MayContinue on 'Expect: 100-continue' header must return true")
	}
}

func TestRequestSwapBodySerial(t *testing.T) {
	t.Parallel()

	testRequestSwapBody(t)
}

func testRequestSwapBody(t *testing.T) {
	var b []byte
	r := &Request{}
	for i := 0; i < 20; i++ {
		bOrig := r.Body()
		b = r.SwapBody(b)
		if !bytes.Equal(bOrig, b) {
			t.Fatalf("unexpected body returned: %q. Expecting %q", b, bOrig)
		}
		r.AppendBodyString("foobar")
	}

	s := "aaaabbbbcccc"
	b = b[:0]
	for i := 0; i < 10; i++ {
		r.SetBodyStream(bytes.NewBufferString(s), len(s))
		b = r.SwapBody(b)
		if string(b) != s {
			t.Fatalf("unexpected body returned: %q. Expecting %q", b, s)
		}
		b = r.SwapBody(b)
		if len(b) > 0 {
			t.Fatalf("unexpected body with non-zero size returned: %q", b)
		}
	}
}

// Test case for testing BasicAuth
var BasicAuthTests = []struct {
	header, username, password string
	ok                         bool
}{
	{"Basic " + base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "Aladdin", "open sesame", true},

	// Case doesn't matter:
	{"BASIC " + base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "Aladdin", "open sesame", true},
	{"basic " + base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "Aladdin", "open sesame", true},

	{"Basic " + base64.StdEncoding.EncodeToString([]byte("Aladdin:open:sesame")), "Aladdin", "open:sesame", true},
	{"Basic " + base64.StdEncoding.EncodeToString([]byte(":")), "", "", true},
	{"Basic" + base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "", "", false},
	{base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "", "", false},
	{"Basic ", "", "", false},
	{"Basic Aladdin:open sesame", "", "", false},
	{`Digest username="Aladdin"`, "", "", false},
}

// struct for
type getBasicAuthTest struct {
	username, password string
	ok                 bool
}

func TestRequestBasicAuth(t *testing.T) {
	for _, tt := range BasicAuthTests {
		req := NewRequest("GET", "http://www.google.com/hi", bytes.NewReader([]byte("hello")))
		req.SetHeader("Authorization", tt.header)
		username, password, ok := req.BasicAuth()
		if ok != tt.ok || username != tt.username || password != tt.password {
			t.Fatalf("BasicAuth() = %+v, want %+v", getBasicAuthTest{username, password, ok},
				getBasicAuthTest{tt.username, tt.password, tt.ok})
		}
	}
}

// Issue: NewRequest should create a Request that doesn't use input parameters as its struct,
// otherwise it will cause panic when we pass a const string as method to NewRequest and call req.SetMethod()
func TestNewRequestWithConstParam(t *testing.T) {
	const method = "POST"
	const uri = "http://www.google.com/hi"
	req := NewRequest(method, uri, nil)
	req.SetMethod("POST")
	req.SetRequestURI("http://www.google.com/hi")
}

func TestRequestCopyToWithOptions(t *testing.T) {
	req := AcquireRequest()
	k1 := "a"
	v1 := "A"
	k2 := "b"
	v2 := "B"
	req.SetOptions(config.WithTag(k1, v1), config.WithTag(k2, v2), config.WithSD(true))
	reqCopy := AcquireRequest()
	req.CopyTo(reqCopy)
	assert.DeepEqual(t, v1, reqCopy.options.Tag(k1))
	assert.DeepEqual(t, v2, reqCopy.options.Tag(k2))
	assert.DeepEqual(t, true, reqCopy.options.IsSD())
}

func TestRequestGetBodyAfterGetBodyStream(t *testing.T) {
	req := AcquireRequest()
	req.SetBodyString("abc")
	req.BodyStream()
	assert.DeepEqual(t, req.Body(), []byte("abc"))
}

func TestRequestSetOptionsNotOverwrite(t *testing.T) {
	req := AcquireRequest()
	req.SetOptions(config.WithSD(true))
	req.SetOptions(config.WithTag("a", "b"))
	req.SetOptions(config.WithTag("c", "d"))
	assert.DeepEqual(t, true, req.Options().IsSD())
	assert.DeepEqual(t, "b", req.Options().Tag("a"))
	assert.DeepEqual(t, "d", req.Options().Tag("c"))

	req.SetOptions(config.WithTag("a", "c"))
	assert.DeepEqual(t, "c", req.Options().Tag("a"))
}

type bodyWriterTo interface {
	BodyWriteTo(writer io.Writer) error
	Body() []byte
}

func testBodyWriteTo(t *testing.T, bw bodyWriterTo, expectedS string, isRetainedBody bool) {
	var buf bytebufferpool.ByteBuffer
	if err := bw.BodyWriteTo(&buf); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	s := buf.B
	if string(s) != expectedS {
		t.Fatalf("unexpected result %q. Expecting %q", s, expectedS)
	}

	body := bw.Body()
	if isRetainedBody {
		if string(body) != expectedS {
			t.Fatalf("unexpected body %q. Expecting %q", body, expectedS)
		}
	} else {
		if len(body) > 0 {
			t.Fatalf("unexpected non-zero body after BodyWriteTo: %q", body)
		}
	}
}
