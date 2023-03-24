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
	"io"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/nocopy"
)

const (
	argsNoValue  = true
	ArgsHasValue = false
)

var nilByteSlice = []byte{}

type argsScanner struct {
	b []byte
}

type Args struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	args []argsKV
	buf  []byte
}

// Set sets 'key=value' argument.
func (a *Args) Set(key, value string) {
	a.args = setArg(a.args, key, value, ArgsHasValue)
}

// Reset clears query args.
func (a *Args) Reset() {
	a.args = a.args[:0]
}

// CopyTo copies all args to dst.
func (a *Args) CopyTo(dst *Args) {
	dst.Reset()
	dst.args = copyArgs(dst.args, a.args)
}

// Del deletes argument with the given key from query args.
func (a *Args) Del(key string) {
	a.args = delAllArgs(a.args, key)
}

// DelBytes deletes argument with the given key from query args.
func (a *Args) DelBytes(key []byte) {
	a.args = delAllArgs(a.args, bytesconv.B2s(key))
}

func (s *argsScanner) next(kv *argsKV) bool {
	if len(s.b) == 0 {
		return false
	}
	kv.noValue = ArgsHasValue

	isKey := true
	k := 0
	for i, c := range s.b {
		switch c {
		case '=':
			if isKey {
				isKey = false
				kv.key = decodeArgAppend(kv.key[:0], s.b[:i])
				k = i + 1
			}
		case '&':
			if isKey {
				kv.key = decodeArgAppend(kv.key[:0], s.b[:i])
				kv.value = kv.value[:0]
				kv.noValue = argsNoValue
			} else {
				kv.value = decodeArgAppend(kv.value[:0], s.b[k:i])
			}
			s.b = s.b[i+1:]
			return true
		}
	}

	if isKey {
		kv.key = decodeArgAppend(kv.key[:0], s.b)
		kv.value = kv.value[:0]
		kv.noValue = argsNoValue
	} else {
		kv.value = decodeArgAppend(kv.value[:0], s.b[k:])
	}
	s.b = s.b[len(s.b):]
	return true
}

func decodeArgAppend(dst, src []byte) []byte {
	if bytes.IndexByte(src, '%') < 0 && bytes.IndexByte(src, '+') < 0 {
		// fast path: src doesn't contain encoded chars
		return append(dst, src...)
	}

	// slow path
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == '%' {
			if i+2 >= len(src) {
				return append(dst, src[i:]...)
			}
			x2 := bytesconv.Hex2intTable[src[i+2]]
			x1 := bytesconv.Hex2intTable[src[i+1]]
			if x1 == 16 || x2 == 16 {
				dst = append(dst, '%')
			} else {
				dst = append(dst, x1<<4|x2)
				i += 2
			}
		} else if c == '+' {
			dst = append(dst, ' ')
		} else {
			dst = append(dst, c)
		}
	}
	return dst
}

func allocArg(h []argsKV) ([]argsKV, *argsKV) {
	n := len(h)
	if cap(h) > n {
		h = h[:n+1]
	} else {
		h = append(h, argsKV{})
	}
	return h, &h[n]
}

func releaseArg(h []argsKV) []argsKV {
	return h[:len(h)-1]
}

func updateArgBytes(h []argsKV, key, value []byte) []argsKV {
	n := len(h)
	for i := 0; i < n; i++ {
		kv := &h[i]
		if kv.noValue && bytes.Equal(key, kv.key) {
			kv.value = append(kv.value[:0], value...)
			kv.noValue = ArgsHasValue
			return h
		}
	}
	return h
}

func setArgBytes(h []argsKV, key, value []byte, noValue bool) []argsKV {
	n := len(h)
	for i := 0; i < n; i++ {
		kv := &h[i]
		if bytes.Equal(key, kv.key) {
			if noValue {
				kv.value = kv.value[:0]
			} else {
				kv.value = append(kv.value[:0], value...)
			}
			kv.noValue = noValue
			return h
		}
	}
	return appendArgBytes(h, key, value, noValue)
}

func setArg(h []argsKV, key, value string, noValue bool) []argsKV {
	n := len(h)
	for i := 0; i < n; i++ {
		kv := &h[i]
		if key == string(kv.key) {
			if noValue {
				kv.value = kv.value[:0]
			} else {
				kv.value = append(kv.value[:0], value...)
			}
			kv.noValue = noValue
			return h
		}
	}
	return appendArg(h, key, value, noValue)
}

func peekArgBytes(h []argsKV, k []byte) []byte {
	for i, n := 0, len(h); i < n; i++ {
		kv := &h[i]
		if bytes.Equal(kv.key, k) {
			if kv.value != nil {
				return kv.value
			}
			return nilByteSlice
		}
	}
	return nil
}

func peekAllArgBytesToDst(dst [][]byte, h []argsKV, k []byte) [][]byte {
	for i, n := 0, len(h); i < n; i++ {
		kv := &h[i]
		if bytes.Equal(kv.key, k) {
			dst = append(dst, kv.value)
		}
	}
	return dst
}

func delAllArgsBytes(args []argsKV, key []byte) []argsKV {
	return delAllArgs(args, bytesconv.B2s(key))
}

func delAllArgs(args []argsKV, key string) []argsKV {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		if key == string(kv.key) {
			tmp := *kv
			copy(args[i:], args[i+1:])
			n--
			i--
			args[n] = tmp
			args = args[:n]
		}
	}
	return args
}

// Has returns true if the given key exists in Args.
func (a *Args) Has(key string) bool {
	return hasArg(a.args, key)
}

func hasArg(h []argsKV, key string) bool {
	for i, n := 0, len(h); i < n; i++ {
		kv := &h[i]
		if key == string(kv.key) {
			return true
		}
	}
	return false
}

// String returns string representation of query args.
func (a *Args) String() string {
	return string(a.QueryString())
}

// decodeArgAppendNoPlus is almost identical to decodeArgAppend, but it doesn't
// substitute '+' with ' '.
//
// The function is copy-pasted from decodeArgAppend due to the performance
// reasons only.
func decodeArgAppendNoPlus(dst, src []byte) []byte {
	if bytes.IndexByte(src, '%') < 0 {
		// fast path: src doesn't contain encoded chars
		return append(dst, src...)
	}

	// slow path
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == '%' {
			if i+2 >= len(src) {
				return append(dst, src[i:]...)
			}
			x2 := bytesconv.Hex2intTable[src[i+2]]
			x1 := bytesconv.Hex2intTable[src[i+1]]
			if x1 == 16 || x2 == 16 {
				dst = append(dst, '%')
			} else {
				dst = append(dst, x1<<4|x2)
				i += 2
			}
		} else {
			dst = append(dst, c)
		}
	}
	return dst
}

func peekArgStr(h []argsKV, k string) []byte {
	for i, n := 0, len(h); i < n; i++ {
		kv := &h[i]
		if string(kv.key) == k {
			return kv.value
		}
	}
	return nil
}

func peekArgStrExists(h []argsKV, k string) (string, bool) {
	for i, n := 0, len(h); i < n; i++ {
		kv := &h[i]
		if string(kv.key) == k {
			return string(kv.value), true
		}
	}
	return "", false
}

// QueryString returns query string for the args.
//
// The returned value is valid until the next call to Args methods.
func (a *Args) QueryString() []byte {
	a.buf = a.AppendBytes(a.buf[:0])
	return a.buf
}

// ParseBytes parses the given b containing query args.
func (a *Args) ParseBytes(b []byte) {
	a.Reset()

	var s argsScanner
	s.b = b

	var kv *argsKV
	a.args, kv = allocArg(a.args)
	for s.next(kv) {
		if len(kv.key) > 0 || len(kv.value) > 0 {
			a.args, kv = allocArg(a.args)
		}
	}
	a.args = releaseArg(a.args)

	if len(a.args) == 0 {
		return
	}
}

// Peek returns query arg value for the given key.
//
// Returned value is valid until the next Args call.
func (a *Args) Peek(key string) []byte {
	return peekArgStr(a.args, key)
}

func (a *Args) PeekExists(key string) (string, bool) {
	return peekArgStrExists(a.args, key)
}

// PeekAll returns all the arg values for the given key.
func (a *Args) PeekAll(key string) [][]byte {
	var values [][]byte
	a.VisitAll(func(k, v []byte) {
		if bytesconv.B2s(k) == key {
			values = append(values, v)
		}
	})
	return values
}

func visitArgs(args []argsKV, f func(k, v []byte)) {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		f(kv.key, kv.value)
	}
}

// Len returns the number of query args.
func (a *Args) Len() int {
	return len(a.args)
}

// AppendBytes appends query string to dst and returns the extended dst.
func (a *Args) AppendBytes(dst []byte) []byte {
	for i, n := 0, len(a.args); i < n; i++ {
		kv := &a.args[i]
		dst = bytesconv.AppendQuotedArg(dst, kv.key)
		if !kv.noValue {
			dst = append(dst, '=')
			if len(kv.value) > 0 {
				dst = bytesconv.AppendQuotedArg(dst, kv.value)
			}
		}
		if i+1 < n {
			dst = append(dst, '&')
		}
	}
	return dst
}

// VisitAll calls f for each existing arg.
//
// f must not retain references to key and value after returning.
// Make key and/or value copies if you need storing them after returning.
func (a *Args) VisitAll(f func(key, value []byte)) {
	visitArgs(a.args, f)
}

// WriteTo writes query string to w.
//
// WriteTo implements io.WriterTo interface.
func (a *Args) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(a.QueryString())
	return int64(n), err
}

// Add adds 'key=value' argument.
//
// Multiple values for the same key may be added.
func (a *Args) Add(key, value string) {
	a.args = appendArg(a.args, key, value, ArgsHasValue)
}
