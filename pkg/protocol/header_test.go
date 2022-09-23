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

func Test_peekRawHeader(t *testing.T) {
	s := "Expect: 100-continue\r\nUser-Agent: foo\r\nHost: 127.0.0.1\r\nConnection: Keep-Alive\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	assert.DeepEqual(t, []byte("127.0.0.1"), peekRawHeader([]byte(s), []byte("Host")))
}

func TestResponseHeader_SetContentLength(t *testing.T) {
	rh := new(ResponseHeader)
	rh.SetContentLength(-2)
	assert.True(t, strings.Contains(string(rh.Header()), "Transfer-Encoding: identity"))
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

func TestRequestHeaderGet(t *testing.T) {
	h := RequestHeader{}
	rightVal := "yyy"
	h.Set("xxx", rightVal)
	val := h.Get("xxx")
	if val != rightVal {
		t.Fatalf("Unexpected %v. Expected %v", val, rightVal)
	}
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
		t.Fatalf("unexpected cookie obtianed: %v", &c)
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
