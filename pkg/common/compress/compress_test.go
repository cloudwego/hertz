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

package compress

import (
	"io"
	"testing"
)

func TestCompressNewCompressWriterPoolMap(t *testing.T) {
	pool := newCompressWriterPoolMap()
	if len(pool) != 12 {
		t.Fatalf("Unexpected number for WriterPoolMap: %d. Expecting 12", len(pool))
	}
}

func TestCompressAppendGunzipBytes(t *testing.T) {
	dst1 := []byte("")
	// src unzip -> "hello". The src must the string that has been gunzipped.
	src1 := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 7, 0, 0, 0, 255, 255}
	expectedRes1 := "hello"
	res1, err1 := AppendGunzipBytes(dst1, src1)
	// gzip will wrap io.EOF to io.ErrUnexpectedEOF
	// just ignore in this case
	if err1 != io.ErrUnexpectedEOF {
		t.Fatalf("Unexpected error: %s", err1)
	}
	if string(res1) != expectedRes1 {
		t.Fatalf("Unexpected : %s. Expecting : %s", res1, expectedRes1)
	}

	dst2 := []byte("!!!")
	src2 := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 7, 0, 0, 0, 255, 255}
	expectedRes2 := "!!!hello"
	res2, err2 := AppendGunzipBytes(dst2, src2)
	if err2 != io.ErrUnexpectedEOF {
		t.Fatalf("Unexpected error: %s", err2)
	}
	if string(res2) != expectedRes2 {
		t.Fatalf("Unexpected : %s. Expecting : %s", res2, expectedRes2)
	}

	dst3 := []byte("!!!")
	src3 := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 0, 0, 0, 255, 255}
	expectedRes3 := "!!!"
	res3, err3 := AppendGunzipBytes(dst3, src3)
	if err3 != io.ErrUnexpectedEOF {
		t.Fatalf("Unexpected error: %s", err3)
	}
	if string(res3) != expectedRes3 {
		t.Fatalf("Unexpected : %s. Expecting : %s", res3, expectedRes3)
	}
}

func TestCompressAppendGzipBytesLevel(t *testing.T) {
	// test the byteSliceWriter case for WriteGzipLevel
	dst1 := []byte("")
	src1 := []byte("hello")
	res1 := AppendGzipBytesLevel(dst1, src1, 5)
	expectedRes1 := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 7, 4, 0, 0, 255, 255, 134, 166, 16, 54, 5, 0, 0, 0}
	if string(res1) != string(expectedRes1) {
		t.Fatalf("Unexpected : %s. Expecting : %s", res1, expectedRes1)
	}
}

func TestCompressWriteGzipLevel(t *testing.T) {
	// test default case for WriteGzipLevel
	var w defaultByteWriter
	p := []byte("hello")
	expectedW := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 7, 4, 0, 0, 255, 255, 134, 166, 16, 54, 5, 0, 0, 0}
	num, err := WriteGzipLevel(&w, p, 5)
	if string(expectedW) != string(w.b) {
		t.Fatalf("Unexpected : %s. Expecting: %s.", w.b, expectedW)
	}
	if num != len(p) {
		t.Fatalf("Unexpected number of compressed bytes: %d", num)
	}
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}

type defaultByteWriter struct {
	b []byte
}

func (w *defaultByteWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}
