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
 * Copyright 2016 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

package http2

import "testing"

func TestRandomScheduler(t *testing.T) {
	ws := NewRandomWriteScheduler()
	ws.Push(makeWriteHeadersRequest(3))
	ws.Push(makeWriteHeadersRequest(4))
	ws.Push(makeWriteHeadersRequest(1))
	ws.Push(makeWriteHeadersRequest(2))
	ws.Push(makeWriteNonStreamRequest())
	ws.Push(makeWriteNonStreamRequest())

	// Pop all frames. Should get the non-stream requests first,
	// followed by the stream requests in any order.
	var order []FrameWriteRequest
	for {
		wr, ok := ws.Pop()
		if !ok {
			break
		}
		order = append(order, wr)
	}
	t.Logf("got frames: %v", order)
	if len(order) != 6 {
		t.Fatalf("got %d frames, expected 6", len(order))
	}
	if order[0].StreamID() != 0 || order[1].StreamID() != 0 {
		t.Fatal("expected non-stream frames first", order[0], order[1])
	}
	got := make(map[uint32]bool)
	for _, wr := range order[2:] {
		got[wr.StreamID()] = true
	}
	for id := uint32(1); id <= 4; id++ {
		if !got[id] {
			t.Errorf("frame not found for stream %d", id)
		}
	}

	// Verify that we clean up maps for empty queues in all cases (golang.org/issue/33812)
	const arbitraryStreamID = 123
	ws.Push(makeHandlerPanicRST(arbitraryStreamID))
	rws := ws.(*randomWriteScheduler)
	if got, want := len(rws.sq), 1; got != want {
		t.Fatalf("len of 123 stream = %v; want %v", got, want)
	}
	_, ok := ws.Pop()
	if !ok {
		t.Fatal("expected to be able to Pop")
	}
	if got, want := len(rws.sq), 0; got != want {
		t.Fatalf("len of 123 stream = %v; want %v", got, want)
	}
}
