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
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRequestHeaderSetRawHeaders(t *testing.T) {
	h := RequestHeader{}
	h.SetRawHeaders([]byte("foo"))
	assert.DeepEqual(t, h.rawHeaders, []byte("foo"))
}

func TestResponseHeaderSetHeaderLength(t *testing.T) {
	h := ResponseHeader{}
	h.SetHeaderLength(15)
	assert.DeepEqual(t, h.headerLength, 15)
	assert.DeepEqual(t, h.GetHeaderLength(), 15)
}

func TestSetNoHTTP11(t *testing.T) {
	rh := ResponseHeader{}
	rh.SetNoHTTP11(true)
	assert.True(t, rh.noHTTP11)

	rh.SetNoHTTP11(false)
	assert.False(t, rh.noHTTP11)
	assert.True(t, rh.IsHTTP11())

	h := RequestHeader{}
	h.SetNoHTTP11(true)
	assert.True(t, h.noHTTP11)

	h.SetNoHTTP11(false)
	assert.False(t, h.noHTTP11)
	assert.True(t, h.IsHTTP11())
}

func TestResponseHeaderSetContentType(t *testing.T) {
	h := ResponseHeader{}
	h.SetContentType("foo")
	assert.DeepEqual(t, h.contentType, []byte("foo"))
}

func TestSetContentLengthBytes(t *testing.T) {
	h := RequestHeader{}
	h.SetContentLengthBytes([]byte("foo"))
	assert.DeepEqual(t, h.contentLengthBytes, []byte("foo"))

	rh := ResponseHeader{}
	rh.SetContentLengthBytes([]byte("foo"))
	assert.DeepEqual(t, rh.contentLengthBytes, []byte("foo"))
}

func Test_peekRawHeader(t *testing.T) {
	s := "Expect: 100-continue\r\nUser-Agent: foo\r\nHost: 127.0.0.1\r\nConnection: Keep-Alive\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	assert.DeepEqual(t, []byte("127.0.0.1"), peekRawHeader([]byte(s), []byte("Host")))
}

func TestResponseHeader_SetContentLength(t *testing.T) {
	rh := new(ResponseHeader)
	rh.SetContentLength(-1)
	assert.True(t, strings.Contains(string(rh.Header()), "Transfer-Encoding: chunked"))
	rh.SetContentLength(-2)
	assert.True(t, strings.Contains(string(rh.Header()), "Transfer-Encoding: identity"))
}

func TestResponseHeader_SetContentRange(t *testing.T) {
	rh := new(ResponseHeader)
	rh.SetContentRange(1, 5, 10)
	assert.DeepEqual(t, rh.bufKV.value, []byte("bytes 1-5/10"))
}

func TestSetCanonical(t *testing.T) {
	h := ResponseHeader{}
	h.SetCanonical([]byte(consts.HeaderContentType), []byte("foo"))
	h.SetCanonical([]byte(consts.HeaderServer), []byte("foo1"))
	h.SetCanonical([]byte(consts.HeaderSetCookie), []byte("foo2"))
	h.SetCanonical([]byte(consts.HeaderContentLength), []byte("3"))
	h.SetCanonical([]byte(consts.HeaderConnection), []byte("foo4"))
	h.SetCanonical([]byte(consts.HeaderTransferEncoding), []byte("foo5"))
	h.SetCanonical([]byte("bar"), []byte("foo6"))

	assert.DeepEqual(t, []byte("foo"), h.ContentType())
	assert.DeepEqual(t, []byte("foo1"), h.Server())
	assert.DeepEqual(t, true, strings.Contains(string(h.Header()), "foo2"))
	assert.DeepEqual(t, 3, h.ContentLength())
	assert.DeepEqual(t, false, h.ConnectionClose())
	assert.DeepEqual(t, false, strings.Contains(string(h.ContentType()), "foo5"))
	assert.DeepEqual(t, true, strings.Contains(string(h.Header()), "bar: foo6"))
}

func TestHasAcceptEncodingBytes(t *testing.T) {
	h := RequestHeader{}
	h.Set(consts.HeaderAcceptEncoding, "gzip")
	assert.True(t, h.HasAcceptEncodingBytes([]byte("gzip")))
}

func TestRequestHeaderGet(t *testing.T) {
	h := RequestHeader{}
	rightVal := "yyy"
	h.Set("xxx", rightVal)
	val := h.Get("xxx")
	if val != rightVal {
		t.Fatalf("Unexpected %v. Expected %v", val, rightVal)
	}
}

func TestResponseHeaderGet(t *testing.T) {
	h := ResponseHeader{}
	rightVal := "yyy"
	h.Set("xxx", rightVal)
	val := h.Get("xxx")
	assert.DeepEqual(t, val, rightVal)
}

func TestRequestHeaderVisitAll(t *testing.T) {
	h := RequestHeader{}
	h.Set("xxx", "yyy")
	h.Set("xxx2", "yyy2")
	h.VisitAll(
		func(k, v []byte) {
			key := string(k)
			value := string(v)
			if key != "Xxx" && key != "Xxx2" {
				t.Fatalf("Unexpected %v. Expected %v", key, "xxx or yyy")
			}
			if key == "Xxx" && value != "yyy" {
				t.Fatalf("Unexpected %v. Expected %v", value, "yyy")
			}
			if key == "Xxx2" && value != "yyy2" {
				t.Fatalf("Unexpected %v. Expected %v", value, "yyy2")
			}
		})
}

func TestRequestHeaderDel(t *testing.T) {
	t.Parallel()

	var h RequestHeader
	h.Set("Foo-Bar", "baz")
	h.Set("aaa", "bbb")
	h.Set(consts.HeaderConnection, "keep-alive")
	h.Set(consts.HeaderContentType, "aaa")
	h.Set(consts.HeaderServer, "aaabbb")
	h.Set(consts.HeaderContentLength, "1123")
	h.SetHost("foobar")
	h.SetCookie("foo", "bar")

	h.del([]byte("Foo-Bar"))
	h.del([]byte("Connection"))
	h.DelBytes([]byte("Content-Type"))
	h.del([]byte(consts.HeaderServer))
	h.del([]byte("Content-Length"))
	h.del([]byte("Set-Cookie"))
	h.del([]byte("Host"))
	h.DelCookie("foo")

	hv := h.Peek("aaa")
	if string(hv) != "bbb" {
		t.Fatalf("unexpected header value: %q. Expecting %q", hv, "bbb")
	}
	hv = h.Peek("Foo-Bar")
	if len(hv) > 0 {
		t.Fatalf("non-zero header value: %q", hv)
	}
	hv = h.Peek(consts.HeaderConnection)
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
	hv = h.Peek(consts.HeaderContentType)
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
	hv = h.Peek(consts.HeaderServer)
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
	hv = h.Peek(consts.HeaderContentLength)
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
	hv = h.FullCookie()
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
	hv = h.Peek(consts.HeaderCookie)
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
	if h.ContentLength() != 0 {
		t.Fatalf("unexpected content-length: %d. Expecting 0", h.ContentLength())
	}
}

func TestResponseHeaderDel(t *testing.T) {
	t.Parallel()

	var h ResponseHeader
	h.Set("Foo-Bar", "baz")
	h.Set("aaa", "bbb")
	h.Set(consts.HeaderConnection, "keep-alive")
	h.Set(consts.HeaderContentType, "aaa")
	h.Set(consts.HeaderServer, "aaabbb")
	h.Set(consts.HeaderContentLength, "1123")

	var c Cookie
	c.SetKey("foo")
	c.SetValue("bar")
	h.SetCookie(&c)

	h.Del("foo-bar")
	h.Del("connection")
	h.DelBytes([]byte("content-type"))
	h.Del(consts.HeaderServer)
	h.Del("content-length")
	h.Del("set-cookie")

	hv := h.Peek("aaa")
	if string(hv) != "bbb" {
		t.Fatalf("unexpected header value: %q. Expecting %q", hv, "bbb")
	}
	hv = h.Peek("Foo-Bar")
	if len(hv) > 0 {
		t.Fatalf("non-zero header value: %q", hv)
	}
	hv = h.Peek(consts.HeaderConnection)
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
	hv = h.Peek(consts.HeaderContentType)
	if string(hv) != string(bytestr.DefaultContentType) {
		t.Fatalf("unexpected content-type: %q. Expecting %q", hv, bytestr.DefaultContentType)
	}
	hv = h.Peek(consts.HeaderServer)
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
	hv = h.Peek(consts.HeaderContentLength)
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}

	if h.Cookie(&c) {
		t.Fatalf("unexpected cookie obtained: %v", &c)
	}

	if h.ContentLength() != 0 {
		t.Fatalf("unexpected content-length: %d. Expecting 0", h.ContentLength())
	}
}

func TestResponseHeaderDelClientCookie(t *testing.T) {
	t.Parallel()

	cookieName := "foobar"

	var h ResponseHeader
	c := AcquireCookie()
	c.SetKey(cookieName)
	c.SetValue("aasdfsdaf")
	h.SetCookie(c)

	h.DelClientCookieBytes([]byte(cookieName))
	if !h.Cookie(c) {
		t.Fatalf("expecting cookie %q", c.Key())
	}
	if !c.Expire().Equal(CookieExpireDelete) {
		t.Fatalf("unexpected cookie expiration time: %s. Expecting %s", c.Expire(), CookieExpireDelete)
	}
	if len(c.Value()) > 0 {
		t.Fatalf("unexpected cookie value: %q. Expecting empty value", c.Value())
	}
	ReleaseCookie(c)
}

func TestResponseHeaderResetConnectionClose(t *testing.T) {
	h := ResponseHeader{}
	h.Set(consts.HeaderConnection, "close")
	hv := h.Peek(consts.HeaderConnection)
	assert.DeepEqual(t, hv, []byte("close"))
	h.SetConnectionClose(true)
	h.ResetConnectionClose()
	assert.False(t, h.connectionClose)
	hv = h.Peek(consts.HeaderConnection)
	if len(hv) > 0 {
		t.Fatalf("ResetConnectionClose do not work,Connection: %q", hv)
	}
}

func TestRequestHeaderResetConnectionClose(t *testing.T) {
	h := RequestHeader{}
	h.Set(consts.HeaderConnection, "close")
	hv := h.Peek(consts.HeaderConnection)
	assert.DeepEqual(t, hv, []byte("close"))
	h.connectionClose = true
	h.ResetConnectionClose()
	assert.False(t, h.connectionClose)
	hv = h.Peek(consts.HeaderConnection)
	if len(hv) > 0 {
		t.Fatalf("ResetConnectionClose do not work,Connection: %q", hv)
	}
}

func TestCheckWriteHeaderCode(t *testing.T) {
	buffer := bytes.NewBuffer(make([]byte, 0, 1024))
	hlog.SetOutput(buffer)
	checkWriteHeaderCode(99)
	assert.True(t, strings.Contains(buffer.String(), "[Warn] HERTZ: Invalid StatusCode code"))
	buffer.Reset()
	checkWriteHeaderCode(600)
	assert.True(t, strings.Contains(buffer.String(), "[Warn] HERTZ: Invalid StatusCode code"))
	buffer.Reset()
	checkWriteHeaderCode(100)
	assert.False(t, strings.Contains(buffer.String(), "[Warn] HERTZ: Invalid StatusCode code"))
	buffer.Reset()
	checkWriteHeaderCode(599)
	assert.False(t, strings.Contains(buffer.String(), "[Warn] HERTZ: Invalid StatusCode code"))
}

func TestResponseHeaderAdd(t *testing.T) {
	t.Parallel()

	m := make(map[string]struct{})
	var h ResponseHeader
	h.Add("aaa", "bbb")
	h.Add("content-type", "xxx")
	m["bbb"] = struct{}{}
	m["xxx"] = struct{}{}
	for i := 0; i < 10; i++ {
		v := fmt.Sprintf("%d", i)
		h.Add("Foo-Bar", v)
		m[v] = struct{}{}
	}
	if h.Len() != 12 {
		t.Fatalf("unexpected header len %d. Expecting 12", h.Len())
	}

	h.VisitAll(func(k, v []byte) {
		switch string(k) {
		case "Aaa", "Foo-Bar", "Content-Type":
			if _, ok := m[string(v)]; !ok {
				t.Fatalf("unexpected value found %q. key %q", v, k)
			}
			delete(m, string(v))
		default:
			t.Fatalf("unexpected key found: %q", k)
		}
	})
	if len(m) > 0 {
		t.Fatalf("%d headers are missed", len(m))
	}
}

func TestRequestHeaderAdd(t *testing.T) {
	t.Parallel()

	m := make(map[string]struct{})
	var h RequestHeader
	h.Add("aaa", "bbb")
	h.Add("user-agent", "xxx")
	m["bbb"] = struct{}{}
	m["xxx"] = struct{}{}
	for i := 0; i < 10; i++ {
		v := fmt.Sprintf("%d", i)
		h.Add("Foo-Bar", v)
		m[v] = struct{}{}
	}
	if h.Len() != 12 {
		t.Fatalf("unexpected header len %d. Expecting 12", h.Len())
	}

	h.VisitAll(func(k, v []byte) {
		switch string(k) {
		case "Aaa", "Foo-Bar", "User-Agent":
			if _, ok := m[string(v)]; !ok {
				t.Fatalf("unexpected value found %q. key %q", v, k)
			}
			delete(m, string(v))
		default:
			t.Fatalf("unexpected key found: %q", k)
		}
	})
	if len(m) > 0 {
		t.Fatalf("%d headers are missed", len(m))
	}
}

func TestResponseHeaderAddContentType(t *testing.T) {
	t.Parallel()

	var h ResponseHeader
	h.Add("Content-Type", "test")

	got := string(h.Peek("Content-Type"))
	expected := "test"
	if got != expected {
		t.Errorf("expected %q got %q", expected, got)
	}

	if n := strings.Count(string(h.Header()), "Content-Type: "); n != 1 {
		t.Errorf("Content-Type occurred %d times", n)
	}
}

func TestRequestHeaderAddContentType(t *testing.T) {
	t.Parallel()

	var h RequestHeader
	h.Add("Content-Type", "test")

	got := string(h.Peek("Content-Type"))
	expected := "test"
	if got != expected {
		t.Errorf("expected %q got %q", expected, got)
	}

	if n := strings.Count(h.String(), "Content-Type: "); n != 1 {
		t.Errorf("Content-Type occurred %d times", n)
	}
}

func TestSetMultipartFormBoundary(t *testing.T) {
	h := RequestHeader{}
	h.SetMultipartFormBoundary("foo")
	assert.DeepEqual(t, h.contentType, []byte("multipart/form-data; boundary=foo"))
}

func TestRequestHeaderSetByteRange(t *testing.T) {
	var h RequestHeader
	h.SetByteRange(1, 5)
	hv := h.Peek(consts.HeaderRange)
	assert.DeepEqual(t, hv, []byte("bytes=1-5"))
}

func TestRequestHeaderSetMethodBytes(t *testing.T) {
	var h RequestHeader
	h.SetMethodBytes([]byte("foo"))
	assert.DeepEqual(t, h.Method(), []byte("foo"))
}

func TestRequestHeaderSetBytesKV(t *testing.T) {
	var h RequestHeader
	h.SetBytesKV([]byte("foo"), []byte("foo1"))
	hv := h.Peek("foo")
	assert.DeepEqual(t, hv, []byte("foo1"))
}

func TestResponseHeaderSetBytesV(t *testing.T) {
	var h ResponseHeader
	h.SetBytesV("foo", []byte("foo1"))
	hv := h.Peek("foo")
	assert.DeepEqual(t, hv, []byte("foo1"))
}

func TestRequestHeaderInitBufValue(t *testing.T) {
	var h RequestHeader
	slice := make([]byte, 0, 10)
	h.InitBufValue(10)
	assert.DeepEqual(t, cap(h.bufKV.value), cap(slice))
	assert.DeepEqual(t, h.GetBufValue(), slice)
}

func TestRequestHeaderDelAllCookies(t *testing.T) {
	var h RequestHeader
	h.SetCanonical([]byte(consts.HeaderSetCookie), []byte("foo2"))
	h.DelAllCookies()
	hv := h.FullCookie()
	if len(hv) > 0 {
		t.Fatalf("non-zero value: %q", hv)
	}
}
