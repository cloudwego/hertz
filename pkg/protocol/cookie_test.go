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
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestCookieAppendBytes(t *testing.T) {
	t.Parallel()

	c := &Cookie{}

	testCookieAppendBytes(t, c, "", "bar", "bar")
	testCookieAppendBytes(t, c, "foo", "", "foo=")
	testCookieAppendBytes(t, c, "ффф", "12 лодлы", "ффф=12 лодлы")

	c.SetDomain("foobar.com")
	testCookieAppendBytes(t, c, "a", "b", "a=b; domain=foobar.com")

	c.SetPath("/a/b")
	testCookieAppendBytes(t, c, "aa", "bb", "aa=bb; domain=foobar.com; path=/a/b")

	c.SetExpire(CookieExpireDelete)
	testCookieAppendBytes(t, c, "xxx", "yyy", "xxx=yyy; expires=Tue, 10 Nov 2009 23:00:00 GMT; domain=foobar.com; path=/a/b")
}

func testCookieAppendBytes(t *testing.T, c *Cookie, key, value, expectedS string) {
	c.SetKey(key)
	c.SetValue(value)
	result := string(c.AppendBytes(nil))
	if result != expectedS {
		t.Fatalf("Unexpected cookie %q. Expecting %q", result, expectedS)
	}
}

func TestParseRequestCookies(t *testing.T) {
	t.Parallel()

	testParseRequestCookies(t, "", "")
	testParseRequestCookies(t, "=", "")
	testParseRequestCookies(t, "foo", "foo")
	testParseRequestCookies(t, "=foo", "foo")
	testParseRequestCookies(t, "bar=", "bar=")
	testParseRequestCookies(t, "xxx=aa;bb=c; =d; ;;e=g", "xxx=aa; bb=c; d; e=g")
	testParseRequestCookies(t, "a;b;c; d=1;d=2", "a; b; c; d=1; d=2")
	testParseRequestCookies(t, "   %D0%B8%D0%B2%D0%B5%D1%82=a%20b%3Bc   ;s%20s=aaa  ", "%D0%B8%D0%B2%D0%B5%D1%82=a%20b%3Bc; s%20s=aaa")
}

func testParseRequestCookies(t *testing.T, s, expectedS string) {
	cookies := parseRequestCookies(nil, []byte(s))
	ss := string(appendRequestCookieBytes(nil, cookies))
	if ss != expectedS {
		t.Fatalf("Unexpected cookies after parsing: %q. Expecting %q. String to parse %q", ss, expectedS, s)
	}
}

func TestAppendRequestCookieBytes(t *testing.T) {
	t.Parallel()

	testAppendRequestCookieBytes(t, "=", "")
	testAppendRequestCookieBytes(t, "foo=", "foo=")
	testAppendRequestCookieBytes(t, "=bar", "bar")
	testAppendRequestCookieBytes(t, "привет=a bc&s s=aaa", "привет=a bc; s s=aaa")
}

func testAppendRequestCookieBytes(t *testing.T, s, expectedS string) {
	kvs := strings.Split(s, "&")
	cookies := make([]argsKV, 0, len(kvs))
	for _, ss := range kvs {
		tmp := strings.SplitN(ss, "=", 2)
		if len(tmp) != 2 {
			t.Fatalf("Cannot find '=' in %q, part of %q", ss, s)
		}
		cookies = append(cookies, argsKV{
			key:   []byte(tmp[0]),
			value: []byte(tmp[1]),
		})
	}

	prefix := "foobar"
	result := string(appendRequestCookieBytes([]byte(prefix), cookies))
	if result[:len(prefix)] != prefix {
		t.Fatalf("unexpected prefix %q. Expecting %q for cookie %q", result[:len(prefix)], prefix, s)
	}
	result = result[len(prefix):]
	if result != expectedS {
		t.Fatalf("Unexpected result %q. Expecting %q for cookie %q", result, expectedS, s)
	}
}

func TestCookieSecureHTTPOnly(t *testing.T) {
	t.Parallel()

	var c Cookie

	if err := c.Parse("foo=bar; HttpOnly; secure"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !c.Secure() {
		t.Fatalf("secure must be set")
	}
	if !c.HTTPOnly() {
		t.Fatalf("HttpOnly must be set")
	}
	s := c.String()
	if !strings.Contains(s, "; secure") {
		t.Fatalf("missing secure flag in cookie %q", s)
	}
	if !strings.Contains(s, "; HttpOnly") {
		t.Fatalf("missing HttpOnly flag in cookie %q", s)
	}
}

func TestCookieSecure(t *testing.T) {
	t.Parallel()

	var c Cookie

	if err := c.Parse("foo=bar; secure"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !c.Secure() {
		t.Fatalf("secure must be set")
	}
	s := c.String()
	if !strings.Contains(s, "; secure") {
		t.Fatalf("missing secure flag in cookie %q", s)
	}

	if err := c.Parse("foo=bar"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.Secure() {
		t.Fatalf("Unexpected secure flag set")
	}
	s = c.String()
	if strings.Contains(s, "secure") {
		t.Fatalf("unexpected secure flag in cookie %q", s)
	}
}

func TestCookieSameSite(t *testing.T) {
	t.Parallel()

	var c Cookie

	if err := c.Parse("foo=bar; samesite"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.SameSite() != CookieSameSiteDefaultMode {
		t.Fatalf("SameSite must be set")
	}
	s := c.String()
	if !strings.Contains(s, "; SameSite") {
		t.Fatalf("missing SameSite flag in cookie %q", s)
	}

	if err := c.Parse("foo=bar; samesite=lax"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.SameSite() != CookieSameSiteLaxMode {
		t.Fatalf("SameSite Lax Mode must be set")
	}
	s = c.String()
	if !strings.Contains(s, "; SameSite=Lax") {
		t.Fatalf("missing SameSite flag in cookie %q", s)
	}

	if err := c.Parse("foo=bar; samesite=strict"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.SameSite() != CookieSameSiteStrictMode {
		t.Fatalf("SameSite Strict Mode must be set")
	}
	s = c.String()
	if !strings.Contains(s, "; SameSite=Strict") {
		t.Fatalf("missing SameSite flag in cookie %q", s)
	}

	if err := c.Parse("foo=bar; samesite=none"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.SameSite() != CookieSameSiteNoneMode {
		t.Fatalf("SameSite None Mode must be set")
	}
	s = c.String()
	if !strings.Contains(s, "; SameSite=None") {
		t.Fatalf("missing SameSite flag in cookie %q", s)
	}

	if err := c.Parse("foo=bar"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	c.SetSameSite(CookieSameSiteNoneMode)
	s = c.String()
	if !strings.Contains(s, "; SameSite=None") {
		t.Fatalf("missing SameSite flag in cookie %q", s)
	}
	if !strings.Contains(s, "; secure") {
		t.Fatalf("missing Secure flag in cookie %q", s)
	}

	if err := c.Parse("foo=bar"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.SameSite() != CookieSameSiteDisabled {
		t.Fatalf("Unexpected SameSite flag set")
	}
	s = c.String()
	if strings.Contains(s, "SameSite") {
		t.Fatalf("unexpected SameSite flag in cookie %q", s)
	}
}

func TestCookieMaxAge(t *testing.T) {
	t.Parallel()

	var c Cookie

	maxAge := 100
	if err := c.Parse("foo=bar; max-age=100"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if maxAge != c.MaxAge() {
		t.Fatalf("max-age must be set")
	}
	s := c.String()
	if !strings.Contains(s, "; max-age=100") {
		t.Fatalf("missing max-age flag in cookie %q", s)
	}

	if err := c.Parse("foo=bar; expires=Tue, 10 Nov 2009 23:00:00 GMT; max-age=100;"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if maxAge != c.MaxAge() {
		t.Fatalf("max-age ignored")
	}
	s = c.String()
	if s != "foo=bar; max-age=100" {
		t.Fatalf("missing max-age in cookie %q", s)
	}

	expires := time.Unix(100, 0)
	c.SetExpire(expires)
	s = c.String()
	if s != "foo=bar; max-age=100" {
		t.Fatalf("expires should be ignored due to max-age: %q", s)
	}

	c.SetMaxAge(0)
	s = c.String()
	if s != "foo=bar; expires=Thu, 01 Jan 1970 00:01:40 GMT" {
		t.Fatalf("missing expires %q", s)
	}
}

func TestCookieHttpOnly(t *testing.T) {
	t.Parallel()

	var c Cookie

	if err := c.Parse("foo=bar; HttpOnly"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !c.HTTPOnly() {
		t.Fatalf("HTTPOnly must be set")
	}
	s := c.String()
	if !strings.Contains(s, "; HttpOnly") {
		t.Fatalf("missing HttpOnly flag in cookie %q", s)
	}

	if err := c.Parse("foo=bar"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if c.HTTPOnly() {
		t.Fatalf("Unexpected HTTPOnly flag set")
	}
	s = c.String()
	if strings.Contains(s, "HttpOnly") {
		t.Fatalf("unexpected HttpOnly flag in cookie %q", s)
	}
}

func TestCookieParse(t *testing.T) {
	t.Parallel()

	testCookieParse(t, "foo", "foo")
	testCookieParse(t, "foo=bar", "foo=bar")
	testCookieParse(t, "foo=", "foo=")
	testCookieParse(t, `foo="bar"`, "foo=bar")
	testCookieParse(t, `"foo"=bar`, `"foo"=bar`)
	testCookieParse(t, "foo=bar; Domain=aaa.com; PATH=/foo/bar", "foo=bar; domain=aaa.com; path=/foo/bar")
	testCookieParse(t, "foo=bar; max-age= 101 ; expires= Tue, 10 Nov 2009 23:00:00 GMT", "foo=bar; max-age=101")
	testCookieParse(t, " xxx = yyy  ; path=/a/b;;;domain=foobar.com ; expires= Tue, 10 Nov 2009 23:00:00 GMT ; ;;",
		"xxx=yyy; expires=Tue, 10 Nov 2009 23:00:00 GMT; domain=foobar.com; path=/a/b")
}

func Test_decodeCookieArg(t *testing.T) {
	src := []byte("          \"aaaaabbbbb\"         ")
	dst := make([]byte, 0)
	dst = decodeCookieArg(dst, src, true)
	assert.DeepEqual(t, []byte("aaaaabbbbb"), dst)
}

func testCookieParse(t *testing.T, s, expectedS string) {
	var c Cookie
	if err := c.Parse(s); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	result := string(c.Cookie())
	if result != expectedS {
		t.Fatalf("unexpected cookies %q. Expecting %q. Original %q", result, expectedS, s)
	}
}

func Test_WarnIfInvalid(t *testing.T) {
	assert.False(t, warnIfInvalid([]byte(";")))
	assert.False(t, warnIfInvalid([]byte("\\")))
	assert.False(t, warnIfInvalid([]byte("\"")))
	assert.True(t, warnIfInvalid([]byte("")))
	for i := 0; i < 5; i++ {
		validCookie := getValidCookie()
		assert.True(t, warnIfInvalid(validCookie))
	}
}

func getValidCookie() []byte {
	var validCookie []byte
	for i := 0; i < 100; i++ {
		r := rand.Intn(0x78-0x20) + 0x20
		if r == ';' || r == '\\' || r == '"' {
			continue
		}
		validCookie = append(validCookie, byte(r))
	}
	return validCookie
}
