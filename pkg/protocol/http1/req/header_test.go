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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/netpoll"
)

func TestRequestHeader_Read(t *testing.T) {
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nUser-Agent: foo\r\nHost: 127.0.0.1\r\nConnection: Keep-Alive\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)
	rh := protocol.RequestHeader{}
	ReadHeader(&rh, zr)

	// firstline
	assert.DeepEqual(t, []byte(consts.MethodPut), rh.Method())
	assert.DeepEqual(t, []byte("/foo/bar"), rh.RequestURI())
	assert.True(t, rh.IsHTTP11())

	// headers
	assert.DeepEqual(t, 5, rh.ContentLength())
	assert.DeepEqual(t, []byte("foo/bar"), rh.ContentType())
	count := 0
	rh.VisitAll(func(key, value []byte) {
		count += 1
	})
	assert.DeepEqual(t, 6, count)
	assert.DeepEqual(t, []byte("foo"), rh.UserAgent())
	assert.DeepEqual(t, []byte("127.0.0.1"), rh.Host())
	assert.DeepEqual(t, []byte("100-continue"), rh.Peek("Expect"))
}

func TestRequestHeaderMultiLineValue(t *testing.T) {
	s := "HTTP/1.1 200 OK\r\n" +
		"EmptyValue1:\r\n" +
		"Content-Type: foo/bar;\r\n\tnewline;\r\n another/newline\r\n" +
		"Foo: Bar\r\n" +
		"Multi-Line: one;\r\n two\r\n" +
		"Values: v1;\r\n v2; v3;\r\n v4;\tv5\r\n" +
		"\r\n"

	header := new(protocol.RequestHeader)
	if _, err := parse(header, []byte(s)); err != nil {
		t.Fatalf("parse headers with multi-line values failed, %s", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(strings.NewReader(s)), nil)
	if err != nil {
		t.Fatalf("parse response using net/http failed, %s", err)
	}

	for name, vals := range response.Header {
		got := string(header.Peek(name))
		want := vals[0]

		if got != want {
			t.Errorf("unexpected %s got: %q want: %q", name, got, want)
		}
	}
}

func TestRequestHeader_Peek(t *testing.T) {
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nUser-Agent: foo\r\nHost: 127.0.0.1\r\nConnection: Keep-Alive\r\nContent-Length: 5\r\nTransfer-Encoding: foo\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)
	rh := protocol.RequestHeader{}
	ReadHeader(&rh, zr)
	assert.DeepEqual(t, []byte("100-continue"), rh.Peek("Expect"))
	assert.DeepEqual(t, []byte("127.0.0.1"), rh.Peek("Host"))
	assert.DeepEqual(t, []byte("foo"), rh.Peek("User-Agent"))
	assert.DeepEqual(t, []byte("Keep-Alive"), rh.Peek("Connection"))
	assert.DeepEqual(t, []byte(""), rh.Peek("Content-Length"))
	assert.DeepEqual(t, []byte("foo/bar"), rh.Peek("Content-Type"))
}

func TestRequestHeaderSetGet(t *testing.T) {
	t.Parallel()

	h := &protocol.RequestHeader{}
	h.SetRequestURI("/aa/bbb")
	h.SetMethod(consts.MethodPost)
	h.Set("foo", "bar")
	h.Set("host", "12345")
	h.Set("content-type", "aaa/bbb")
	h.Set("content-length", "1234")
	h.Set("user-agent", "aaabbb")
	h.Set("referer", "axcv")
	h.Set("baz", "xxxxx")
	h.Set("transfer-encoding", "chunked")
	h.Set("connection", "close")

	expectRequestHeaderGet(t, h, "Foo", "bar")
	expectRequestHeaderGet(t, h, consts.HeaderHost, "12345")
	expectRequestHeaderGet(t, h, consts.HeaderContentType, "aaa/bbb")
	expectRequestHeaderGet(t, h, consts.HeaderContentLength, "1234")
	expectRequestHeaderGet(t, h, "USER-AGent", "aaabbb")
	expectRequestHeaderGet(t, h, consts.HeaderReferer, "axcv")
	expectRequestHeaderGet(t, h, "baz", "xxxxx")
	expectRequestHeaderGet(t, h, consts.HeaderTransferEncoding, "")
	expectRequestHeaderGet(t, h, "connecTION", "close")
	if !h.ConnectionClose() {
		t.Fatalf("unset connection: close")
	}

	if h.ContentLength() != 1234 {
		t.Fatalf("Unexpected content-length %d. Expected %d", h.ContentLength(), 1234)
	}

	w := &bytes.Buffer{}
	bw := bufio.NewWriter(w)
	zw := netpoll.NewWriter(bw)
	err := WriteHeader(h, zw)
	if err != nil {
		t.Fatalf("Unexpected error when writing request header: %s", err)
	}
	if err := bw.Flush(); err != nil {
		t.Fatalf("Unexpected error when flushing request header: %s", err)
	}
	zw.Flush()
	bw.Flush()

	var h1 protocol.RequestHeader
	br := bufio.NewReader(w)
	zr := mock.ZeroCopyReader{Reader: br}
	if err = ReadHeader(&h1, zr); err != nil {
		t.Fatalf("Unexpected error when reading request header: %s", err)
	}

	if h1.ContentLength() != h.ContentLength() {
		t.Fatalf("Unexpected Content-Length %d. Expected %d", h1.ContentLength(), h.ContentLength())
	}

	expectRequestHeaderGet(t, &h1, "Foo", "bar")
	expectRequestHeaderGet(t, &h1, "HOST", "12345")
	expectRequestHeaderGet(t, &h1, consts.HeaderContentType, "aaa/bbb")
	expectRequestHeaderGet(t, &h1, consts.HeaderContentLength, "1234")
	expectRequestHeaderGet(t, &h1, "USER-AGent", "aaabbb")
	expectRequestHeaderGet(t, &h1, consts.HeaderReferer, "axcv")
	expectRequestHeaderGet(t, &h1, "baz", "xxxxx")
	expectRequestHeaderGet(t, &h1, consts.HeaderTransferEncoding, "")
	expectRequestHeaderGet(t, &h1, consts.HeaderConnection, "close")
	if !h1.ConnectionClose() {
		t.Fatalf("unset connection: close")
	}
}

func TestRequestHeaderCookie(t *testing.T) {
	t.Parallel()

	var h protocol.RequestHeader
	h.SetRequestURI("/foobar")
	h.Set(consts.HeaderHost, "foobar.com")

	h.SetCookie("foo", "bar")
	h.SetCookie("привет", "мир")

	if string(h.Cookie("foo")) != "bar" {
		t.Fatalf("Unexpected cookie value %q. Expected %q", h.Cookie("foo"), "bar")
	}
	if string(h.Cookie("привет")) != "мир" {
		t.Fatalf("Unexpected cookie value %q. Expected %q", h.Cookie("привет"), "мир")
	}

	w := &bytes.Buffer{}
	zw := netpoll.NewWriter(w)
	if err := WriteHeader(&h, zw); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	var h1 protocol.RequestHeader
	br := bufio.NewReader(w)
	zr := mock.ZeroCopyReader{Reader: br}
	if err := ReadHeader(&h1, zr); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !bytes.Equal(h1.Cookie("foo"), h.Cookie("foo")) {
		t.Fatalf("Unexpected cookie value %q. Expected %q", h1.Cookie("foo"), h.Cookie("foo"))
	}
	h1.DelCookie("foo")
	if len(h1.Cookie("foo")) > 0 {
		t.Fatalf("Unexpected cookie found: %q", h1.Cookie("foo"))
	}
	if !bytes.Equal(h1.Cookie("привет"), h.Cookie("привет")) {
		t.Fatalf("Unexpected cookie value %q. Expected %q", h1.Cookie("привет"), h.Cookie("привет"))
	}
	h1.DelCookie("привет")
	if len(h1.Cookie("привет")) > 0 {
		t.Fatalf("Unexpected cookie found: %q", h1.Cookie("привет"))
	}

	h.DelAllCookies()
	if len(h.Cookie("foo")) > 0 {
		t.Fatalf("Unexpected cookie found: %q", h.Cookie("foo"))
	}
	if len(h.Cookie("привет")) > 0 {
		t.Fatalf("Unexpected cookie found: %q", h.Cookie("привет"))
	}
}

func TestRequestRawHeaders(t *testing.T) {
	t.Parallel()

	kvs := "hOsT: foobar\r\n" +
		"value:  b\r\n" +
		"\r\n"
	t.Run("normalized", func(t *testing.T) {
		s := "GET / HTTP/1.1\r\n" + kvs
		exp := kvs
		var h protocol.RequestHeader
		zr := mock.NewZeroCopyReader(s)
		if err := ReadHeader(&h, zr); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(h.Host()) != "foobar" {
			t.Fatalf("unexpected host: %q. Expecting %q", h.Host(), "foobar")
		}
		v2 := h.Peek("Value")
		if !bytes.Equal(v2, []byte{'b'}) {
			t.Fatalf("expecting non empty value. Got %q", v2)
		}
		if raw := h.RawHeaders(); string(raw) != exp {
			t.Fatalf("expected header %q, got %q", exp, raw)
		}
	})
	for _, n := range []int{0, 1, 4, 8} {
		t.Run(fmt.Sprintf("post-%dk", n), func(t *testing.T) {
			l := 1024 * n
			body := make([]byte, l)
			for i := range body {
				body[i] = 'a'
			}
			cl := fmt.Sprintf("Content-Length: %d\r\n", l)
			s := "POST / HTTP/1.1\r\n" + cl + kvs + string(body)
			exp := cl + kvs
			var h protocol.RequestHeader
			zr := mock.NewZeroCopyReader(s)
			if err := ReadHeader(&h, zr); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if string(h.Host()) != "foobar" {
				t.Fatalf("unexpected host: %q. Expecting %q", h.Host(), "foobar")
			}
			v2 := h.Peek("Value")
			if !bytes.Equal(v2, []byte{'b'}) {
				t.Fatalf("expecting non empty value. Got %q", v2)
			}
			if raw := h.RawHeaders(); string(raw) != exp {
				t.Fatalf("expected header %q, got %q", exp, raw)
			}
		})
	}
	t.Run("http10", func(t *testing.T) {
		s := "GET / HTTP/1.0\r\n" + kvs
		exp := kvs
		var h protocol.RequestHeader
		zr := mock.NewZeroCopyReader(s)
		if err := ReadHeader(&h, zr); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(h.Host()) != "foobar" {
			t.Fatalf("unexpected host: %q. Expecting %q", h.Host(), "foobar")
		}
		v2 := h.Peek("Value")
		if !bytes.Equal(v2, []byte{'b'}) {
			t.Fatalf("expecting non empty value. Got %q", v2)
		}
		if raw := h.RawHeaders(); string(raw) != exp {
			t.Fatalf("expected header %q, got %q", exp, raw)
		}
	})
	t.Run("no-kvs", func(t *testing.T) {
		s := "GET / HTTP/1.1\r\n\r\n"
		exp := ""
		var h protocol.RequestHeader
		h.DisableNormalizing()
		zr := mock.NewZeroCopyReader(s)
		if err := ReadHeader(&h, zr); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(h.Host()) != "" {
			t.Fatalf("unexpected host: %q. Expecting %q", h.Host(), "")
		}
		v1 := h.Peek("NoKey")
		if len(v1) > 0 {
			t.Fatalf("expecting empty value. Got %q", v1)
		}
		if raw := h.RawHeaders(); string(raw) != exp {
			t.Fatalf("expected header %q, got %q", exp, raw)
		}
	})
}

func TestRequestHeaderEmptyValueFromHeader(t *testing.T) {
	t.Parallel()

	var h1 protocol.RequestHeader
	h1.SetRequestURI("/foo/bar")
	h1.SetHost("foobar")
	h1.Set("EmptyValue1", "")
	h1.Set("EmptyValue2", " ")
	s := h1.String()

	var h protocol.RequestHeader
	zr := mock.NewZeroCopyReader(s)
	if err := ReadHeader(&h, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(h.Host()) != string(h1.Host()) {
		t.Fatalf("unexpected host: %q. Expecting %q", h.Host(), h1.Host())
	}
	v1 := h.Peek("EmptyValue1")
	if len(v1) > 0 {
		t.Fatalf("expecting empty value. Got %q", v1)
	}
	v2 := h.Peek("EmptyValue2")
	if len(v2) > 0 {
		t.Fatalf("expecting empty value. Got %q", v2)
	}
}

func TestRequestHeaderEmptyValueFromString(t *testing.T) {
	t.Parallel()

	s := "GET / HTTP/1.1\r\n" +
		"EmptyValue1:\r\n" +
		"Host: foobar\r\n" +
		"EmptyValue2: \r\n" +
		"\r\n"
	var h protocol.RequestHeader
	zr := mock.NewZeroCopyReader(s)
	if err := ReadHeader(&h, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(h.Host()) != "foobar" {
		t.Fatalf("unexpected host: %q. Expecting %q", h.Host(), "foobar")
	}
	v1 := h.Peek("EmptyValue1")
	if len(v1) > 0 {
		t.Fatalf("expecting empty value. Got %q", v1)
	}
	v2 := h.Peek("EmptyValue2")
	if len(v2) > 0 {
		t.Fatalf("expecting empty value. Got %q", v2)
	}
}

func expectRequestHeaderGet(t *testing.T, h *protocol.RequestHeader, key, expectedValue string) {
	if string(h.Peek(key)) != expectedValue {
		t.Fatalf("Unexpected value for key %q: %q. Expected %q", key, h.Peek(key), expectedValue)
	}
}

func TestRequestHeader_PeekIfExists(t *testing.T) {
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nexists: \r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	rh := protocol.RequestHeader{}
	err := ReadHeader(&rh, mock.NewZeroCopyReader(s))
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, []byte{}, rh.Peek("exists"))
	assert.DeepEqual(t, []byte(nil), rh.Peek("non-exists"))
}

func TestRequestHeaderError(t *testing.T) {
	er := mock.EOFReader{}
	rh := protocol.RequestHeader{}
	err := ReadHeader(&rh, &er)
	assert.True(t, errors.Is(err, errs.ErrNothingRead))
}

func TestReadHeader(t *testing.T) {
	s := "P"
	zr := mock.NewZeroCopyReader(s)
	rh := protocol.RequestHeader{}
	err := ReadHeader(&rh, zr)
	assert.NotNil(t, err)
}

func TestParseHeaders(t *testing.T) {
	rh := protocol.RequestHeader{}
	_, err := parseHeaders(&rh, []byte{' '})
	assert.NotNil(t, err)
}

func TestTryRead(t *testing.T) {
	rh := protocol.RequestHeader{}
	s := "P"
	zr := mock.NewZeroCopyReader(s)
	err := tryRead(&rh, zr, 0)
	assert.NotNil(t, err)
}

func TestParseFirstLine(t *testing.T) {
	tests := []struct {
		input    []byte
		method   string
		uri      string
		protocol string
		err      error
	}{
		// Test case 1: n < 0
		{
			input:    []byte("GET /path/to/resource HTTP/1.0\r\n"),
			method:   "GET",
			uri:      "/path/to/resource",
			protocol: "HTTP/1.0",
			err:      nil,
		},
		// Test case 2: n == 0
		{
			input:    []byte(" /path/to/resource HTTP/1.1\r\n"),
			method:   "",
			uri:      "",
			protocol: "",
			err:      fmt.Errorf("requestURI cannot be empty in"),
		},
		// Test case 3: !bytes.Equal(b[n+1:], bytestr.StrHTTP11)
		{
			input:    []byte("POST /path/to/resource HTTP/1.2\r\n"),
			method:   "POST",
			uri:      "/path/to/resource",
			protocol: "HTTP/1.0",
			err:      nil,
		},
	}

	for _, tc := range tests {
		header := &protocol.RequestHeader{}
		_, err := parseFirstLine(header, tc.input)
		assert.NotNil(t, err)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int
		wantErr  bool
	}{
		// normal test
		{
			name:     "normal",
			input:    []byte("GET /path/to/resource HTTP/1.1\r\nHost: example.com\r\n\r\n"),
			expected: len([]byte("GET /path/to/resource HTTP/1.1\r\nHost: example.com\r\n\r\n")),
			wantErr:  false,
		},
		// parseFirstLine error
		{
			name:     "parseFirstLine error",
			input:    []byte("INVALID_LINE\r\nHost: example.com\r\n\r\n"),
			expected: 0,
			wantErr:  true,
		},
		// ext.ReadRawHeaders error
		{
			name:     "ext.ReadRawHeaders error",
			input:    []byte("GET /path/to/resource HTTP/1.1\r\nINVALID_HEADER\r\n\r\n"),
			expected: 0,
			wantErr:  true,
		},
		// parseHeaders error
		{
			name:     "parseHeaders error",
			input:    []byte("GET /path/to/resource HTTP/1.1\r\nHost: example.com\r\nINVALID_HEADER\r\n"),
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header := &protocol.RequestHeader{}
			bytesRead, err := parse(header, tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("Expected error: %v, but got: %v", tc.wantErr, err)
			}
			if bytesRead != tc.expected {
				t.Errorf("Expected bytes read: %d, but got: %d", tc.expected, bytesRead)
			}
		})
	}
}
