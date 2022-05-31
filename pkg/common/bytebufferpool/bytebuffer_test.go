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

package bytebufferpool

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestByteBufferReadFrom(t *testing.T) {
	prefix := "foobar"
	expectedS := "asadfsdafsadfasdfisdsdfa"
	prefixLen := int64(len(prefix))
	expectedN := int64(len(expectedS))

	var bb ByteBuffer
	bb.WriteString(prefix)

	rf := (io.ReaderFrom)(&bb)
	for i := 0; i < 20; i++ {
		r := bytes.NewBufferString(expectedS)
		n, err := rf.ReadFrom(r)
		if n != expectedN {
			t.Fatalf("unexpected n=%d. Expecting %d. iteration %d", n, expectedN, i)
		}
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		bbLen := int64(bb.Len())
		expectedLen := prefixLen + int64(i+1)*expectedN
		if bbLen != expectedLen {
			t.Fatalf("unexpected byteBuffer length: %d. Expecting %d", bbLen, expectedLen)
		}
		for j := 0; j < i; j++ {
			start := prefixLen + int64(j)*expectedN
			b := bb.B[start : start+expectedN]
			if string(b) != expectedS {
				t.Fatalf("unexpected byteBuffer contents: %q. Expecting %q", b, expectedS)
			}
		}
	}
}

func TestByteBufferWriteTo(t *testing.T) {
	expectedS := "foobarbaz"
	var bb ByteBuffer
	bb.WriteString(expectedS[:3])
	bb.WriteString(expectedS[3:])

	wt := (io.WriterTo)(&bb)
	var w bytes.Buffer
	for i := 0; i < 10; i++ {
		n, err := wt.WriteTo(&w)
		if n != int64(len(expectedS)) {
			t.Fatalf("unexpected n returned from WriteTo: %d. Expecting %d", n, len(expectedS))
		}
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		s := w.String()
		if s != expectedS {
			t.Fatalf("unexpected string written %q. Expecting %q", s, expectedS)
		}
		w.Reset()
	}
}

func TestByteBufferGetPutSerial(t *testing.T) {
	testByteBufferGetPut(t)
}

func TestByteBufferGetPutConcurrent(t *testing.T) {
	concurrency := 10
	ch := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			testByteBufferGetPut(t)
			ch <- struct{}{}
		}()
	}

	for i := 0; i < concurrency; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("timeout!")
		}
	}
}

func testByteBufferGetPut(t *testing.T) {
	for i := 0; i < 10; i++ {
		expectedS := fmt.Sprintf("num %d", i)
		b := Get()
		b.B = append(b.B, "num "...)
		b.B = append(b.B, fmt.Sprintf("%d", i)...)
		if string(b.B) != expectedS {
			t.Fatalf("unexpected result: %q. Expecting %q", b.B, expectedS)
		}
		Put(b)
	}
}

func testByteBufferGetString(t *testing.T) {
	for i := 0; i < 10; i++ {
		expectedS := fmt.Sprintf("num %d", i)
		b := Get()
		b.SetString(expectedS)
		if b.String() != expectedS {
			t.Fatalf("unexpected result: %q. Expecting %q", b.B, expectedS)
		}
		Put(b)
	}
}

func TestByteBufferGetStringSerial(t *testing.T) {
	testByteBufferGetString(t)
}

func TestByteBufferGetStringConcurrent(t *testing.T) {
	concurrency := 10
	ch := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			testByteBufferGetString(t)
			ch <- struct{}{}
		}()
	}

	for i := 0; i < concurrency; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("timeout!")
		}
	}
}
