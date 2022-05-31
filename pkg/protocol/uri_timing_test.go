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
	"testing"
)

func BenchmarkURIParsePath(b *testing.B) {
	benchmarkURIParse(b, "google.com", "/foo/bar")
}

func BenchmarkURIParsePathQueryString(b *testing.B) {
	benchmarkURIParse(b, "google.com", "/foo/bar?query=string&other=value")
}

func BenchmarkURIParsePathQueryStringHash(b *testing.B) {
	benchmarkURIParse(b, "google.com", "/foo/bar?query=string&other=value#hashstring")
}

func BenchmarkURIParseHostname(b *testing.B) {
	benchmarkURIParse(b, "google.com", "http://foobar.com/foo/bar?query=string&other=value#hashstring")
}

func BenchmarkURIFullURI(b *testing.B) {
	host := []byte("foobar.com")
	requestURI := []byte("/foobar/baz?aaa=bbb&ccc=ddd")
	uriLen := len(host) + len(requestURI) + 7

	b.RunParallel(func(pb *testing.PB) {
		var u URI
		u.Parse(host, requestURI)
		for pb.Next() {
			uri := u.FullURI()
			if len(uri) != uriLen {
				b.Fatalf("unexpected uri len %d. Expecting %d", len(uri), uriLen)
			}
		}
	})
}

func benchmarkURIParse(b *testing.B, host, uri string) {
	strHost, strURI := []byte(host), []byte(uri)

	b.RunParallel(func(pb *testing.PB) {
		var u URI
		for pb.Next() {
			u.Parse(strHost, strURI)
		}
	})
}
