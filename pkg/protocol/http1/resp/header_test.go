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

package resp

import (
	"bytes"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/netpoll"
)

func TestResponseHeaderCookie(t *testing.T) {
	t.Parallel()

	var h protocol.ResponseHeader
	var c protocol.Cookie

	c.SetKey("foobar")
	c.SetValue("aaa")
	h.SetCookie(&c)

	c.SetKey("йцук")
	c.SetDomain("foobar.com")
	h.SetCookie(&c)

	c.Reset()
	c.SetKey("foobar")
	if !h.Cookie(&c) {
		t.Fatalf("Cannot find cookie %q", c.Key())
	}

	var expectedC1 protocol.Cookie
	expectedC1.SetKey("foobar")
	expectedC1.SetValue("aaa")
	if !equalCookie(&expectedC1, &c) {
		t.Fatalf("unexpected cookie\n%#v\nExpected\n%#v\n", &c, &expectedC1)
	}

	c.SetKey("йцук")
	if !h.Cookie(&c) {
		t.Fatalf("cannot find cookie %q", c.Key())
	}

	var expectedC2 protocol.Cookie
	expectedC2.SetKey("йцук")
	expectedC2.SetValue("aaa")
	expectedC2.SetDomain("foobar.com")
	if !equalCookie(&expectedC2, &c) {
		t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &c, &expectedC2)
	}

	h.VisitAllCookie(func(key, value []byte) {
		var cc protocol.Cookie
		if err := cc.ParseBytes(value); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(key, cc.Key()) {
			t.Fatalf("Unexpected cookie key %q. Expected %q", key, cc.Key())
		}
		switch {
		case bytes.Equal(key, []byte("foobar")):
			if !equalCookie(&expectedC1, &cc) {
				t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &cc, &expectedC1)
			}
		case bytes.Equal(key, []byte("йцук")):
			if !equalCookie(&expectedC2, &cc) {
				t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &cc, &expectedC2)
			}
		default:
			t.Fatalf("unexpected cookie key %q", key)
		}
	})

	w := &bytes.Buffer{}
	zw := netpoll.NewWriter(w)
	if err := WriteHeader(&h, zw); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	h.DelAllCookies()

	var h1 protocol.ResponseHeader
	zr := netpoll.NewReader(w)
	if err := ReadHeader(&h1, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	c.SetKey("foobar")
	if !h1.Cookie(&c) {
		t.Fatalf("Cannot find cookie %q", c.Key())
	}
	if !equalCookie(&expectedC1, &c) {
		t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &c, &expectedC1)
	}

	h1.DelCookie("foobar")
	if h.Cookie(&c) {
		t.Fatalf("Unexpected cookie found: %v", &c)
	}
	if h1.Cookie(&c) {
		t.Fatalf("Unexpected cookie found: %v", &c)
	}

	c.SetKey("йцук")
	if !h1.Cookie(&c) {
		t.Fatalf("cannot find cookie %q", c.Key())
	}
	if !equalCookie(&expectedC2, &c) {
		t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &c, &expectedC2)
	}

	h1.DelCookie("йцук")
	if h.Cookie(&c) {
		t.Fatalf("Unexpected cookie found: %v", &c)
	}
	if h1.Cookie(&c) {
		t.Fatalf("Unexpected cookie found: %v", &c)
	}
}

func equalCookie(c1, c2 *protocol.Cookie) bool {
	if !bytes.Equal(c1.Key(), c2.Key()) {
		return false
	}
	if !bytes.Equal(c1.Value(), c2.Value()) {
		return false
	}
	if !c1.Expire().Equal(c2.Expire()) {
		return false
	}
	if !bytes.Equal(c1.Domain(), c2.Domain()) {
		return false
	}
	if !bytes.Equal(c1.Path(), c2.Path()) {
		return false
	}
	return true
}
