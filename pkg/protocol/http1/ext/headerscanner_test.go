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

package ext

import (
	"bufio"
	"errors"
	"net/http"
	"strings"
	"testing"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestHasHeaderValue(t *testing.T) {
	s := []byte("Expect: 100-continue, User-Agent: foo, Host: 127.0.0.1, Connection: Keep-Alive, Content-Length: 5")
	assert.True(t, HasHeaderValue(s, []byte("Connection: Keep-Alive")))
	assert.False(t, HasHeaderValue(s, []byte("Connection: Keep-Alive1")))
}

func TestResponseHeaderMultiLineValue(t *testing.T) {
	firstLine := "HTTP/1.1 200 OK\r\n"
	rawHeaders := "EmptyValue1:\r\n" +
		"Content-Type: foo/bar;\r\n\tnewline;\r\n another/newline\r\n" +
		"Foo: Bar\r\n" +
		"Multi-Line: one;\r\n two\r\n" +
		"Values: v1;\r\n v2; v3;\r\n v4;\tv5\r\n" +
		"\r\n"

	// compared with http response
	response, err := http.ReadResponse(bufio.NewReader(strings.NewReader(firstLine+rawHeaders)), nil)
	assert.Nil(t, err)
	defer func() { response.Body.Close() }()

	hs := &HeaderScanner{}
	hs.B = []byte(rawHeaders)
	hs.DisableNormalizing = false
	hmap := make(map[string]string, len(response.Header))
	for hs.Next() {
		if len(hs.Key) > 0 {
			hmap[string(hs.Key)] = string(hs.Value)
		}
	}

	for name, vals := range response.Header {
		got := hmap[name]
		want := vals[0]
		assert.DeepEqual(t, want, got)
	}
}

func TestHeaderScannerError(t *testing.T) {
	t.Run("TestHeaderScannerErrorInvalidName", func(t *testing.T) {
		rawHeaders := "Host: go.dev\r\nGopher-New-\r\n Line: This is a header on multiple lines\r\n\r\n"
		testTestHeaderScannerError(t, rawHeaders, errInvalidName)
	})
	t.Run("TestHeaderScannerErrorNeedMore", func(t *testing.T) {
		rawHeaders := "This is a header on multiple lines"
		testTestHeaderScannerError(t, rawHeaders, errs.ErrNeedMore)

		rawHeaders = "Gopher-New-\r\n Line"
		testTestHeaderScannerError(t, rawHeaders, errs.ErrNeedMore)
	})
}

func testTestHeaderScannerError(t *testing.T, rawHeaders string, expectError error) {
	hs := &HeaderScanner{}
	hs.B = []byte(rawHeaders)
	hs.DisableNormalizing = false
	for hs.Next() {
	}
	assert.NotNil(t, hs.Err)
	assert.True(t, errors.Is(hs.Err, expectError))
}
