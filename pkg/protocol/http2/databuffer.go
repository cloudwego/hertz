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

import (
	"io"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
)

// dataBuffer is an io.ReadWriter backed by a list of data chunks.
// Each dataBuffer is used to read DATA frames on a single stream.
// The buffer is divided into chunks so the server can limit the
// total memory used by a single connection without limiting the
// request body size on any single stream.
type dataBuffer struct {
	buf    *bytebufferpool.ByteBuffer
	offset uint64
}

func newDataBuffer(buf *bytebufferpool.ByteBuffer) *dataBuffer {
	return &dataBuffer{buf: buf, offset: 0}
}

// Read copies bytes from the buffer into p.
// It is an error to read when no data is available.
func (b *dataBuffer) Read(p []byte) (int, error) {
	if b.Len() == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.buf.B[b.offset:])
	b.offset += uint64(n)
	return n, nil
}

// Len returns the number of bytes of the unread portion of the buffer.
func (b *dataBuffer) Len() int {
	return b.buf.Len() - int(b.offset)
}

// Write appends p to the buffer.
func (b *dataBuffer) Write(p []byte) (int, error) {
	return b.buf.Write(p)
}

func (b *dataBuffer) Reset() {
	b.offset = 0
	b.buf.Reset()
	bytebufferpool.Put(b.buf)
}
