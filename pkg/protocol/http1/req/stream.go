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

package req

import (
	"bytes"
	"io"
	"sync"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
)

var requestStreamPool = sync.Pool{
	New: func() interface{} {
		return &requestStream{}
	},
}

type requestStream struct {
	prefetchedBytes *bytes.Reader
	header          *protocol.RequestHeader
	reader          network.Reader
	offset          int
	contentLength   int
	chunkLeft       int
}

func (rs *requestStream) Read(p []byte) (int, error) {
	defer func() {
		if rs.reader != nil {
			rs.reader.Release() //nolint:errcheck
		}
	}()
	if rs.contentLength == -1 {
		if rs.chunkLeft == 0 {
			chunkSize, err := utils.ParseChunkSize(rs.reader)
			if err != nil {
				return 0, err
			}
			if chunkSize == 0 {
				err = ReadTrailer(rs.header, rs.reader)
				if err != nil && err != io.EOF {
					return 0, err
				}
				return 0, io.EOF
			}

			rs.chunkLeft = chunkSize
		}
		bytesToRead := len(p)

		if bytesToRead > rs.chunkLeft {
			bytesToRead = rs.chunkLeft
		}

		src, err := rs.reader.Peek(bytesToRead)
		copied := copy(p, src)
		rs.reader.Skip(copied)
		rs.chunkLeft -= copied

		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return copied, err
		}

		if rs.chunkLeft == 0 {
			err = utils.SkipCRLF(rs.reader)
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
		}
		return copied, err
	}

	if rs.offset == rs.contentLength {
		return 0, io.EOF
	}
	var n int
	var err error
	// read from the pre-read buffer
	if int(rs.prefetchedBytes.Size()) > rs.offset {
		n, err = rs.prefetchedBytes.Read(p)
		rs.offset += n
		if rs.offset == rs.header.ContentLength() {
			return n, io.EOF
		}
		if err != nil || len(p) == n {
			return n, err
		}
	}

	// read from the wire
	m := len(p) - n
	remain := rs.contentLength - rs.offset

	if m > remain {
		m = remain
	}

	if conn, ok := rs.reader.(io.Reader); ok {
		m, err = conn.Read(p[n:])
	} else {
		var tmp []byte
		tmp, err = rs.reader.Peek(m)
		m = copy(p[n:], tmp)
		rs.reader.Skip(m) // nolint: errcheck
	}
	rs.offset += m
	n += m

	if err != nil {
		// the data on stream may be incomplete
		if err == io.EOF {
			if rs.offset != rs.contentLength {
				err = io.ErrUnexpectedEOF
			}
			// ensure that skipRest works fine
			rs.offset = rs.contentLength
		}
		return n, err
	}
	if rs.offset == rs.contentLength {
		err = io.EOF
	}
	return n, err
}

func (rs *requestStream) skipRest() error {
	// The body length doesn't exceed the maxContentLengthInStream or
	// the bodyStream has been skip rest
	if rs.prefetchedBytes == nil {
		return nil
	}
	// max value of pSize is 8193, it's safe.
	pSize := int(rs.prefetchedBytes.Size())
	if rs.contentLength <= pSize || rs.offset == rs.contentLength {
		return nil
	}

	needSkipLen := 0
	if rs.offset > pSize {
		needSkipLen = rs.contentLength - rs.offset
	} else {
		needSkipLen = rs.contentLength - pSize
	}

	// must skip size
	for {
		skip := rs.reader.Len()
		if skip == 0 {
			_, err := rs.reader.Peek(1)
			if err != nil {
				return err
			}
			skip = rs.reader.Len()
		}
		if skip > needSkipLen {
			skip = needSkipLen
		}
		rs.reader.Skip(skip) //nolint:errcheck
		needSkipLen -= skip
		if needSkipLen == 0 {
			return nil
		}
	}
}

func AcquireRequestStream(b *bytebufferpool.ByteBuffer, r network.Reader, contentLength int, h *protocol.RequestHeader) io.Reader {
	rs := requestStreamPool.Get().(*requestStream)
	rs.prefetchedBytes = bytes.NewReader(b.B)
	rs.contentLength = contentLength
	rs.reader = r
	rs.header = h
	return rs
}

// ReleaseBodyStream releases the body stream
//
// NOTE: Be careful to use this method unless you know what it's for
func ReleaseRequestStream(requestReader io.Reader) (err error) {
	if rs, ok := requestReader.(*requestStream); ok {
		err = rs.skipRest()
		rs.prefetchedBytes = nil
		rs.offset = 0
		rs.reader = nil
		rs.header = nil
		requestStreamPool.Put(rs)
	}
	return
}
