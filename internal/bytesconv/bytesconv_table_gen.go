//go:build ignore
// +build ignore

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

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
)

const (
	toLower = 'a' - 'A'
)

func main() {
	hex2intTable := func() [256]byte {
		var b [256]byte
		for i := 0; i < 256; i++ {
			c := byte(16)
			if i >= '0' && i <= '9' {
				c = byte(i) - '0'
			} else if i >= 'a' && i <= 'f' {
				c = byte(i) - 'a' + 10
			} else if i >= 'A' && i <= 'F' {
				c = byte(i) - 'A' + 10
			}
			b[i] = c
		}
		return b
	}()

	toLowerTable := func() [256]byte {
		var a [256]byte
		for i := 0; i < 256; i++ {
			c := byte(i)
			if c >= 'A' && c <= 'Z' {
				c += toLower
			}
			a[i] = c
		}
		return a
	}()

	toUpperTable := func() [256]byte {
		var a [256]byte
		for i := 0; i < 256; i++ {
			c := byte(i)
			if c >= 'a' && c <= 'z' {
				c -= toLower
			}
			a[i] = c
		}
		return a
	}()

	quotedArgShouldEscapeTable := func() [256]byte {
		// According to RFC 3986 §2.3
		var a [256]byte
		for i := 0; i < 256; i++ {
			a[i] = 1
		}

		// ALPHA
		for i := int('a'); i <= int('z'); i++ {
			a[i] = 0
		}
		for i := int('A'); i <= int('Z'); i++ {
			a[i] = 0
		}

		// DIGIT
		for i := int('0'); i <= int('9'); i++ {
			a[i] = 0
		}

		// Unreserved characters
		for _, v := range `-_.~` {
			a[v] = 0
		}

		return a
	}()

	quotedPathShouldEscapeTable := func() [256]byte {
		// The implementation here equal to net/url shouldEscape(s, encodePath)
		//
		// The RFC allows : @ & = + $ but saves / ; , for assigning
		// meaning to individual path segments. This package
		// only manipulates the path as a whole, so we allow those
		// last three as well. That leaves only ? to escape.
		a := quotedArgShouldEscapeTable

		for _, v := range `$&+,/:;=@` {
			a[v] = 0
		}

		return a
	}()

	validCookieValueTable := func() [256]byte {
		// The implementation here is equal to net/http validCookieValueByte(b byte)
		// see https://datatracker.ietf.org/doc/html/rfc6265#section-4.1.1
		var a [256]byte
		for i := 0; i < 256; i++ {
			a[i] = 0
		}

		for i := 0x20; i < 0x7f; i++ {
			a[i] = 1
		}

		a['"'] = 0
		a[';'] = 0
		a['\\'] = 0

		return a
	}()

	w := new(bytes.Buffer)
	w.WriteString(pre)
	fmt.Fprintf(w, "const Hex2intTable = %q\n", hex2intTable)
	fmt.Fprintf(w, "const ToLowerTable = %q\n", toLowerTable)
	fmt.Fprintf(w, "const ToUpperTable = %q\n", toUpperTable)
	fmt.Fprintf(w, "const QuotedArgShouldEscapeTable = %q\n", quotedArgShouldEscapeTable)
	fmt.Fprintf(w, "const QuotedPathShouldEscapeTable = %q\n", quotedPathShouldEscapeTable)
	fmt.Fprintf(w, "const ValidCookieValueTable = %q\n", validCookieValueTable)

	if err := ioutil.WriteFile("bytesconv_table.go", w.Bytes(), 0o660); err != nil {
		log.Fatal(err)
	}
}

const pre = `/*
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

package bytesconv

// Code generated by go run bytesconv_table_gen.go; DO NOT EDIT.
// See bytesconv_table_gen.go for more information about these tables.

`
