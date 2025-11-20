/*
 * Copyright 2023 CloudWeGo Authors
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

package resp

import (
	"errors"
	"runtime"
	"sync"

	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/http1/ext"
)

var chunkWriterPool sync.Pool

func init() {
	chunkWriterPool = sync.Pool{
		New: func() interface{} {
			return &chunkedBodyWriter{}
		},
	}
}

type chunkedBodyWriter struct {
	r *protocol.Response
	w network.Writer

	err         error
	finalized   bool
	wroteHeader bool
}

var errChunkedFinished = errors.New("chunked response is finished; no more data will be written.")

// Write implements network.ExtWriter.Write / io.Writer.Write
func (c *chunkedBodyWriter) Write(p []byte) (n int, err error) {
	if c.finalized {
		return 0, errChunkedFinished
	}
	if c.err != nil {
		return 0, c.err
	}
	if err := c.WriteHeader(); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		// prevent from sending zero-len chunk which indicates stream ends.
		// callers may write with zero-len buf unintentionally.
		// use Finalize() instead.
		return 0, nil
	}
	if err := c.writeChunk(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// WriteHeader writes the response header for chunked encoding
func (c *chunkedBodyWriter) WriteHeader() error {
	if c.wroteHeader {
		return c.err
	}
	c.wroteHeader = true
	c.r.Header.SetContentLength(-1)
	if c.err = WriteHeader(&c.r.Header, c.w); c.err != nil {
		return c.err
	}
	return nil
}

func (c *chunkedBodyWriter) writeChunk(b []byte) error {
	if c.err = ext.WriteChunk(c.w, b, false); c.err != nil {
		return c.err
	}
	return nil
}

func (c *chunkedBodyWriter) Flush() error {
	return c.w.Flush()
}

// Finalize will write the ending chunk as well as trailer and flush the writer.
// Warning: do not call this method by yourself, unless you know what you are doing.
func (c *chunkedBodyWriter) Finalize() error {
	if c.finalized || c.err != nil {
		return c.err
	}
	c.finalized = true
	if err := c.WriteHeader(); err != nil {
		return err
	}
	// zero-len chunk
	if err := c.writeChunk(nil); err != nil {
		return err
	}
	// trailer which ends with \r\n
	_, c.err = c.w.WriteBinary(c.r.Header.Trailer().Header())
	if c.err == nil {
		c.err = c.Flush()
	}
	return c.err
}

func (c *chunkedBodyWriter) release() {
	c.r = nil
	c.w = nil
	c.err = nil
	c.finalized = false
	c.wroteHeader = false
	chunkWriterPool.Put(c)
}

// NewChunkedBodyWriter creates a new chunked body writer.
func NewChunkedBodyWriter(r *protocol.Response, w network.Writer) network.ExtWriter {
	extWriter := chunkWriterPool.Get().(*chunkedBodyWriter)
	extWriter.r = r
	extWriter.w = w
	runtime.SetFinalizer(extWriter, (*chunkedBodyWriter).release)
	return extWriter
}
