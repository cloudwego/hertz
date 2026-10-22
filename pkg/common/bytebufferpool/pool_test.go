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
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestPoolMaxRetainedCapacity(t *testing.T) {
	for _, tt := range []struct {
		name     string
		limit    int
		autoMax  uint64
		capacity int
		keep     bool
	}{
		{"default", 0, 0, 1024, true},
		{"negative", -1, 0, 1024, true},
		{"below", 512, 0, 511, true},
		{"equal", 512, 0, 512, true},
		{"above", 512, 0, 513, false},
		{"auto_smaller", 512, 128, 256, false},
		{"auto_larger", 512, 1024, 513, false},
		{"auto_without_limit", 0, 128, 256, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pool{maxSize: tt.autoMax}
			p.SetMaxRetainedCapacity(tt.limit)
			// Keep length small to verify that the limit applies to capacity.
			b := &ByteBuffer{B: make([]byte, 1, tt.capacity)}
			p.Put(b)
			// This isolated pool has no concurrent users. Inspect Reset's effect
			// because sync.Pool may discard even an accepted buffer (e.g. with -race).
			if reset := len(b.B) == 0; reset != tt.keep {
				t.Fatalf("buffer reset = %v, want %v", reset, tt.keep)
			}
			if v := p.pool.Get(); !tt.keep && v != nil {
				t.Fatal("oversized buffer retained")
			}
		})
	}
}

func TestPoolMaxRetainedCapacityAfterCalibration(t *testing.T) {
	var p Pool
	p.SetMaxRetainedCapacity(512)
	p.calls[index(4096)] = calibrateCallsThreshold
	p.Put(&ByteBuffer{B: make([]byte, 4096)})
	if p.defaultSize != 4096 || p.maxSize != 4096 {
		t.Fatal("expected calibration from large buffers")
	}
	if p.pool.Get() != nil {
		t.Fatal("calibration bypassed the configured limit")
	}
	if b := p.Get(); cap(b.B) > 512 {
		t.Fatalf("preallocated capacity = %d, exceeds limit", cap(b.B))
	}
	// Disabling the limit restores automatic preallocation.
	p.SetMaxRetainedCapacity(0)
	if b := p.Get(); cap(b.B) != 4096 {
		t.Fatalf("preallocated capacity = %d, want 4096", cap(b.B))
	}
}

func TestDefaultPoolMaxRetainedCapacity(t *testing.T) {
	SetMaxRetainedCapacity(512)
	defer SetMaxRetainedCapacity(0)
	b := &ByteBuffer{B: make([]byte, 1, 4096)}
	Put(b)
	if len(b.B) == 0 {
		t.Fatal("oversized buffer accepted by default pool")
	}
}

func TestPoolMaxRetainedCapacityConcurrent(t *testing.T) {
	var p Pool
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				p.SetMaxRetainedCapacity(i % 1024)
				b := p.Get()
				b.B = allocNBytes(b.B, 1025)
				p.Put(b)
			}
		}()
	}
	wg.Wait()
}

func TestIndex(t *testing.T) {
	testIndex(t, 0, 0)
	testIndex(t, 1, 0)

	testIndex(t, minSize-1, 0)
	testIndex(t, minSize, 0)
	testIndex(t, minSize+1, 1)

	testIndex(t, 2*minSize-1, 1)
	testIndex(t, 2*minSize, 1)
	testIndex(t, 2*minSize+1, 2)

	testIndex(t, maxSize-1, steps-1)
	testIndex(t, maxSize, steps-1)
	testIndex(t, maxSize+1, steps-1)
}

func testIndex(t *testing.T, n, expectedIdx int) {
	idx := index(n)
	if idx != expectedIdx {
		t.Fatalf("unexpected idx for n=%d: %d. Expecting %d", n, idx, expectedIdx)
	}
}

func TestPoolCalibrate(t *testing.T) {
	for i := 0; i < steps*calibrateCallsThreshold; i++ {
		n := 1004
		if i%15 == 0 {
			n = rand.Intn(15234)
		}
		testGetPut(t, n)
	}
}

func TestPoolVariousSizesSerial(t *testing.T) {
	testPoolVariousSizes(t)
}

func TestPoolVariousSizesConcurrent(t *testing.T) {
	concurrency := 5
	ch := make(chan struct{})
	for i := 0; i < concurrency; i++ {
		go func() {
			testPoolVariousSizes(t)
			ch <- struct{}{}
		}()
	}
	for i := 0; i < concurrency; i++ {
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout")
		}
	}
}

func testPoolVariousSizes(t *testing.T) {
	for i := 0; i < steps+1; i++ {
		n := 1 << uint32(i)

		testGetPut(t, n)
		testGetPut(t, n+1)
		testGetPut(t, n-1)

		for j := 0; j < 10; j++ {
			testGetPut(t, j+n)
		}
	}
}

func testGetPut(t *testing.T, n int) {
	bb := Get()
	if len(bb.B) > 0 {
		t.Fatalf("non-empty byte buffer returned from acquire")
	}
	bb.B = allocNBytes(bb.B, n)
	Put(bb)
}

func allocNBytes(dst []byte, n int) []byte {
	diff := n - cap(dst)
	if diff <= 0 {
		return dst[:n]
	}
	return append(dst, make([]byte, diff)...)
}
