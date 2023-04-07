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
 */

package standard

import (
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
)

var bufferPool = sync.Pool{}

func init() {
	bufferPool.New = func() interface{} {
		return &linkBufferNode{}
	}
}

type linkBufferNode struct {
	buf      []byte          // buffer
	off      int             // read-offset
	malloc   int             // write-offset
	next     *linkBufferNode // the next node of the linked buffer
	readOnly bool            // whether this node is a read only node
}

type linkBuffer struct {
	// The release head
	head *linkBufferNode
	// The read pointer to point current read node
	// There is no need to save current write node
	read *linkBufferNode
	// Tail node
	write *linkBufferNode
	len   int
}

func (l *linkBuffer) release() {
	for l.head != nil {
		node := l.head
		l.head = l.head.next
		node.Release()
	}
}

// newBufferNode creates a new node with buffer size
func newBufferNode(size int) *linkBufferNode {
	buf := bufferPool.Get().(*linkBufferNode)
	buf.buf = malloc(size, size)
	return buf
}

// Reset resets this node.
//
// NOTE: Reset won't recycle the buffer of node.
func (b *linkBufferNode) Reset() {
	b.buf = b.buf[:0]
	b.off, b.malloc = 0, 0
	b.readOnly = false
}

// Len calculates the data size of this node.
func (b *linkBufferNode) Len() int {
	return b.malloc - b.off
}

func (b *linkBufferNode) recyclable() bool {
	return cap(b.buf) <= block8k && !b.readOnly
}

// Cap returns the capacity of the node buffer
func (b *linkBufferNode) Cap() int {
	return cap(b.buf) - b.malloc
}

// Release will recycle the buffer of node
func (b *linkBufferNode) Release() {
	if !b.readOnly {
		free(b.buf)
	}
	b.readOnly = false
	b.buf = nil
	b.next = nil
	b.malloc, b.off = 0, 0
	bufferPool.Put(b)
}

// malloc limits the cap of the buffer from mcache.
func malloc(size, capacity int) []byte {
	if capacity > mallocMax {
		return make([]byte, size, capacity)
	}
	return mcache.Malloc(size, capacity)
}

// free limits the cap of the buffer from mcache.
func free(buf []byte) {
	if cap(buf) > mallocMax {
		return
	}
	mcache.Free(buf)
}
