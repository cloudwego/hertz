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

package stackless

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewFuncSimple(t *testing.T) {
	var n uint64
	f := NewFunc(func(ctx interface{}) {
		atomic.AddUint64(&n, uint64(ctx.(int)))
	})

	iterations := 4 * 1024
	for i := 0; i < iterations; i++ {
		if !f(2) {
			t.Fatalf("f mustn't return false")
		}
	}
	if n != uint64(2*iterations) {
		t.Fatalf("Unexpected n: %d. Expecting %d", n, 2*iterations)
	}
}

func TestNewFuncMulti(t *testing.T) {
	var n1, n2 uint64
	f1 := NewFunc(func(ctx interface{}) {
		atomic.AddUint64(&n1, uint64(ctx.(int)))
	})
	f2 := NewFunc(func(ctx interface{}) {
		atomic.AddUint64(&n2, uint64(ctx.(int)))
	})

	iterations := 4 * 1024

	f1Done := make(chan error, 1)
	go func() {
		var err error
		for i := 0; i < iterations; i++ {
			if !f1(3) {
				err = fmt.Errorf("f1 mustn't return false")
				break
			}
		}
		f1Done <- err
	}()

	f2Done := make(chan error, 1)
	go func() {
		var err error
		for i := 0; i < iterations; i++ {
			if !f2(5) {
				err = fmt.Errorf("f2 mustn't return false")
				break
			}
		}
		f2Done <- err
	}()

	select {
	case err := <-f1Done:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}

	select {
	case err := <-f2Done:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}

	if n1 != uint64(3*iterations) {
		t.Fatalf("unexpected n1: %d. Expecting %d", n1, 3*iterations)
	}
	if n2 != uint64(5*iterations) {
		t.Fatalf("unexpected n2: %d. Expecting %d", n2, 5*iterations)
	}
}
