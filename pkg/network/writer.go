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

package network

import (
	"io"
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
)

const size4K = 1024 * 4

type node struct {
	data     []byte
	readOnly bool
}

var nodePool = sync.Pool{}

func init() {
	nodePool.New = func() interface{} {
		return &node{}
	}
}

type networkWriter struct {
	caches []*node
	w      io.Writer
}

func (w *networkWriter) release() {
	for _, n := range w.caches {
		if !n.readOnly {
			mcache.Free(n.data)
		}
		n.data = nil
		n.readOnly = false
		nodePool.Put(n)
	}
	w.caches = w.caches[:0]
}

func (w *networkWriter) Malloc(length int) (buf []byte, err error) {
	idx := len(w.caches)
	if idx > 0 {
		idx -= 1
		inUse := len(w.caches[idx].data)
		if !w.caches[idx].readOnly && cap(w.caches[idx].data)-inUse >= length {
			end := inUse + length
			w.caches[idx].data = w.caches[idx].data[:end]
			return w.caches[idx].data[inUse:end], nil
		}
	}
	buf = mcache.Malloc(length)
	n := nodePool.Get().(*node)
	n.data = buf
	w.caches = append(w.caches, n)
	return
}

func (w *networkWriter) WriteBinary(b []byte) (length int, err error) {
	length = len(b)
	if length < size4K {
		buf, _ := w.Malloc(length)
		copy(buf, b)
		return
	}
	node := nodePool.Get().(*node)
	node.readOnly = true
	node.data = b
	w.caches = append(w.caches, node)
	return
}

func (w *networkWriter) Flush() (err error) {
	for _, c := range w.caches {
		_, err = w.w.Write(c.data)
		if err != nil {
			break
		}
	}
	w.release()
	return
}

func NewWriter(w io.Writer) Writer {
	return &networkWriter{
		w: w,
	}
}

type ExtWriter interface {
	io.Writer
	Flush() error

	// Finalize will be called by framework before the writer is released.
	// Implementations must guarantee that Finalize is safe for multiple calls.
	Finalize() error
}
