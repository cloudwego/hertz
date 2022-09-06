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
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestURI_Username(t *testing.T) {
	var req Request
	req.SetRequestURI("http://user:pass@example.com/foo/bar")
	uri := req.URI()
	user1 := string(uri.username)
	fmt.Printf("1--- uri:%s user:%s\n", uri.RequestURI(), user1)
	req.Header.SetRequestURIBytes([]byte("/foo/bar"))
	uri = req.URI()
	user2 := string(uri.username)
	fmt.Printf("2--- uri:%s user:%s\n", uri.RequestURI(), user2)
	if user1 != user2 {
		t.Fatal("user1 != user2")
	}
}

func TestURICopyToQueryArgs(t *testing.T) {
	t.Parallel()

	var u URI
	a := u.QueryArgs()
	a.Set("foo", "bar")

	var u1 URI
	u.CopyTo(&u1)
	a1 := u1.QueryArgs()

	if string(a1.Peek("foo")) != "bar" {
		t.Fatalf("unexpected query args value %q. Expecting %q", a1.Peek("foo"), "bar")
	}
}

func TestURICopyTo(t *testing.T) {
	t.Parallel()

	var u URI
	var copyU URI
	u.CopyTo(&copyU)
	if !reflect.DeepEqual(&u, &copyU) { //nolint:govet
		t.Fatalf("URICopyTo fail, u: \n%+v\ncopyu: \n%+v\n", &u, &copyU) //nolint:govet
	}

	u.UpdateBytes([]byte("https://google.com/foo?bar=baz&baraz#qqqq"))
	u.CopyTo(&copyU)
	if !reflect.DeepEqual(&u, &copyU) { //nolint:govet
		t.Fatalf("URICopyTo fail, u: \n%+v\ncopyu: \n%+v\n", &u, &copyU) //nolint:govet
	}
}

func TestURILastPathSegment(t *testing.T) {
	t.Parallel()

	testURILastPathSegment(t, "", "")
	testURILastPathSegment(t, "/", "")
	testURILastPathSegment(t, "/foo/bar/", "")
	testURILastPathSegment(t, "/foobar.js", "foobar.js")
	testURILastPathSegment(t, "/foo/bar/baz.html", "baz.html")
}

func testURILastPathSegment(t *testing.T, path, expectedSegment string) {
	var u URI
	u.SetPath(path)
	segment := u.LastPathSegment()
	if string(segment) != expectedSegment {
		t.Fatalf("unexpected last path segment for path %q: %q. Expecting %q", path, segment, expectedSegment)
	}
}

func TestURIPathEscape(t *testing.T) {
	t.Parallel()

	testURIPathEscape(t, "/foo/bar", "/foo/bar")
	testURIPathEscape(t, "/f_o-o=b:ar,b.c&q", "/f_o-o=b:ar,b.c&q")
	testURIPathEscape(t, "/aa?bb.тест~qq", "/aa%3Fbb.%D1%82%D0%B5%D1%81%D1%82~qq")
}

func TestURIUpdate(t *testing.T) {
	t.Parallel()

	// full uri
	testURIUpdate(t, "http://foo.bar/baz?aaa=22#aaa", "https://aaa.com/bb", "https://aaa.com/bb")
	// empty uri
	testURIUpdate(t, "http://aaa.com/aaa.html?234=234#add", "", "http://aaa.com/aaa.html?234=234#add")

	// request uri
	testURIUpdate(t, "ftp://aaa/xxx/yyy?aaa=bb#aa", "/boo/bar?xx", "ftp://aaa/boo/bar?xx")

	// relative uri
	testURIUpdate(t, "http://foo.bar/baz/xxx.html?aaa=22#aaa", "bb.html?xx=12#pp", "http://foo.bar/baz/bb.html?xx=12#pp")
	testURIUpdate(t, "http://xx/a/b/c/d", "../qwe/p?zx=34", "http://xx/a/b/qwe/p?zx=34")
	testURIUpdate(t, "https://qqq/aaa.html?foo=bar", "?baz=434&aaa#xcv", "https://qqq/aaa.html?baz=434&aaa#xcv")
	testURIUpdate(t, "http://foo.bar/baz", "~a/%20b=c,тест?йцу=ке", "http://foo.bar/~a/%20b=c,%D1%82%D0%B5%D1%81%D1%82?йцу=ке")
	testURIUpdate(t, "http://foo.bar/baz", "/qwe#fragment", "http://foo.bar/qwe#fragment")
	testURIUpdate(t, "http://foobar/baz/xxx", "aaa.html#bb?cc=dd&ee=dfd", "http://foobar/baz/aaa.html#bb?cc=dd&ee=dfd")

	// hash
	testURIUpdate(t, "http://foo.bar/baz#aaa", "#fragment", "http://foo.bar/baz#fragment")

	// uri without scheme
	testURIUpdate(t, "https://foo.bar/baz", "//aaa.bbb/cc?dd", "https://aaa.bbb/cc?dd")
	testURIUpdate(t, "http://foo.bar/baz", "//aaa.bbb/cc?dd", "http://aaa.bbb/cc?dd")
}

func testURIUpdate(t *testing.T, base, update, result string) {
	var u URI
	u.Parse(nil, []byte(base))
	u.Update(update)
	s := u.String()
	if s != result {
		t.Fatalf("unexpected result %q. Expecting %q. base=%q, update=%q", s, result, base, update)
	}
}

func testURIPathEscape(t *testing.T, path, expectedRequestURI string) {
	var u URI
	u.SetPath(path)
	requestURI := u.RequestURI()
	if string(requestURI) != expectedRequestURI {
		t.Fatalf("unexpected requestURI %q. Expecting %q. path %q", requestURI, expectedRequestURI, path)
	}
}

func TestDelArgs(t *testing.T) {
	var args Args
	args.Set("foo", "bar")
	assert.DeepEqual(t, string(args.Peek("foo")), "bar")
	args.Del("foo")
	assert.DeepEqual(t, string(args.Peek("foo")), "")

	args.Set("foo2", "bar2")
	assert.DeepEqual(t, string(args.Peek("foo2")), "bar2")
	args.DelBytes([]byte("foo2"))
	assert.DeepEqual(t, string(args.Peek("foo2")), "")
}

func TestURIFullURI(t *testing.T) {
	t.Parallel()

	var args Args

	// empty scheme, path and hash
	testURIFullURI(t, "", "foobar.com", "", "", &args, "http://foobar.com/")

	// empty scheme and hash
	testURIFullURI(t, "", "aaa.com", "/foo/bar", "", &args, "http://aaa.com/foo/bar")
	// empty hash
	testURIFullURI(t, "fTP", "XXx.com", "/foo", "", &args, "ftp://xxx.com/foo")

	// empty args
	testURIFullURI(t, "https", "xx.com", "/", "aaa", &args, "https://xx.com/#aaa")

	// non-empty args and non-ASCII path
	args.Set("foo", "bar")
	args.Set("xxx", "йух")
	testURIFullURI(t, "", "xxx.com", "/тест123", "2er", &args, "http://xxx.com/%D1%82%D0%B5%D1%81%D1%82123?foo=bar&xxx=%D0%B9%D1%83%D1%85#2er")

	// test with empty args and non-empty query string
	var u URI
	u.Parse([]byte("google.com"), []byte("/foo?bar=baz&baraz#qqqq"))
	uri := u.FullURI()
	expectedURI := "http://google.com/foo?bar=baz&baraz#qqqq"
	if string(uri) != expectedURI {
		t.Fatalf("Unexpected URI: %q. Expected %q", uri, expectedURI)
	}
}

func testURIFullURI(t *testing.T, scheme, host, path, hash string, args *Args, expectedURI string) {
	var u URI

	u.SetScheme(scheme)
	u.SetHost(host)
	u.SetPath(path)
	u.SetHash(hash)
	args.CopyTo(u.QueryArgs())

	uri := u.FullURI()
	if string(uri) != expectedURI {
		t.Fatalf("Unexpected URI: %q. Expected %q", uri, expectedURI)
	}
}

func TestParsePathWindows(t *testing.T) {
	t.Parallel()

	testParsePathWindows(t, "/../../../../../foo", "/foo")
	testParsePathWindows(t, "/..\\..\\..\\..\\..\\foo", "/foo")
	testParsePathWindows(t, "/..%5c..%5cfoo", "/foo")
}

func testParsePathWindows(t *testing.T, path, expectedPath string) {
	var u URI
	u.Parse(nil, []byte(path))
	parsedPath := u.Path()
	if filepath.Separator == '\\' && string(parsedPath) != expectedPath {
		t.Fatalf("Unexpected Path: %q. Expected %q", parsedPath, expectedPath)
	}
}
