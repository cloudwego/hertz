/*
 *	Copyright 2022 CloudWeGo Authors
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
 * Copyright 2014 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

package http2

import "testing"

func TestFlowAdd(t *testing.T) {
	var f flow
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(-1) {
		t.Fatal("failed to add -1")
	}
	if got, want := f.available(), int32(0); got != want {
		t.Fatalf("size = %d; want %d", got, want)
	}
	if !f.add(1<<31 - 1) {
		t.Fatal("failed to add 2^31-1")
	}
	if got, want := f.available(), int32(1<<31-1); got != want {
		t.Fatalf("size = %d; want %d", got, want)
	}
	if f.add(1) {
		t.Fatal("adding 1 to max shouldn't be allowed")
	}
}

func TestFlowAddOverflow(t *testing.T) {
	var f flow
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(-1) {
		t.Fatal("failed to add -1")
	}
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(-3) {
		t.Fatal("failed to add -3")
	}
	if got, want := f.available(), int32(-2); got != want {
		t.Fatalf("size = %d; want %d", got, want)
	}
	if !f.add(1<<31 - 1) {
		t.Fatal("failed to add 2^31-1")
	}
	if got, want := f.available(), int32(1+-3+(1<<31-1)); got != want {
		t.Fatalf("size = %d; want %d", got, want)
	}
}
