/*
 * Copyright 2025 CloudWeGo Authors
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

package sse

import (
	"errors"
	"strconv"
	"sync"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

// Writer represents a writer for Server-Sent Events (SSE).
//
// It is used to write individual events to the response body.
type Writer struct {
	w network.ExtWriter

	mu sync.Mutex
}

// NewWriter creates a new SSE writer.
func NewWriter(c *app.RequestContext) *Writer {
	// make sure proxies won't cache the data
	c.Response.Header.Set("Cache-Control", "no-cache")
	// browsers may need charset=utf-8 for logging responses
	// even though it's unnecessary as per spec, coz chunks must be in utf8.
	c.Response.Header.SetContentType("text/event-stream; charset=utf-8")
	w := c.Response.GetHijackWriter()
	if w == nil {
		w = resp.NewChunkedBodyWriter(&c.Response, c.GetWriter())
		c.Response.HijackWriter(w)
	}
	return &Writer{w: w}
}

var (
	errIDContainsCRLR   = errors.New(`id field contains '\r' or '\n'`)
	errTypeContainsCRLR = errors.New(`event field contains '\r' or '\n'`)
)

// WriteEvent writes a single SSE event to the response body.
//
// If id, eventType, or data are zero-length, they will be ignored.
// It returns an error if the event contains invalid characters or if the underlying writer fails.
func (w *Writer) WriteEvent(id, eventType string, data []byte) error {
	return w.Write(&Event{
		ID:   id,
		Type: eventType,
		Data: data,
	})
}

// WriteKeepAlive writes a comment line with "keep-alive" to the response body.
//
// It keeps the underlying connection alive, which is useful when using proxy servers.
func (w *Writer) WriteKeepAlive() error {
	return w.WriteComment("keep-alive")
}

// WriteComment writes comment lines to the response body.
//
// Client-side will ignore lines starting with a U+003A COLON character (:)
// see: https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation
func (w *Writer) WriteComment(s string) error {
	p := bytebufferpool.Get()
	defer bytebufferpool.Put(p)

	buf := p.B[:0]
	for data := bytesconv.S2b(s); len(data) > 0; {
		i, b, _ := scanEOL(data, true)
		buf = append(buf, ':')
		buf = append(buf, b...)
		buf = append(buf, '\n')
		data = data[i:]
	}
	if len(buf) == 0 {
		buf = append(buf, ':')
	}
	p.B = buf
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.w.Write(p.B); err != nil {
		return err
	}
	return w.w.Flush()
}

// Write writes a single SSE event to the response body.
//
// It returns an error if the event contains invalid characters or underlying writer fails.
func (w *Writer) Write(e *Event) error {
	p := bytebufferpool.Get()
	defer bytebufferpool.Put(p)

	buf := p.B[:0]

	if e.IsSetID() {
		if hasCRLF(e.ID) {
			return errIDContainsCRLR
		}
		buf = append(append(append(buf, "id: "...), e.ID...), '\n')
	}

	if e.IsSetType() {
		if e.Type == "message" {
			buf = append(buf, "event: message\n"...) // fast path for message
		} else {
			if hasCRLF(e.Type) {
				return errTypeContainsCRLR
			}
			buf = append(append(append(buf, "event: "...), e.Type...), '\n')
		}
	}

	if e.IsSetRetry() {
		buf = append(buf, "retry: "...)
		buf = strconv.AppendInt(buf, e.Retry.Milliseconds(), 10)
		buf = append(buf, '\n')
	}

	if e.IsSetData() {
		data := e.Data
		// replace EOLs with multiple "data: " lines
		for len(data) > 0 {
			i, b, _ := scanEOL(data, true)
			buf = append(buf, "data: "...)
			buf = append(buf, b...)
			buf = append(buf, '\n')
			data = data[i:]
		}
	}
	p.B = append(buf, '\n') // end of event
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.w.Write(p.B); err != nil {
		return err
	}
	return w.w.Flush()
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Finalize()
}
