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
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// errNotSSEContentType is returned when the response's content type is not text/event-stream.
var errNotSSEContentType = errors.New("Content-Type returned by server is NOT text/event-stream")

// Reader represents a reader for Server-Sent Events (SSE).
//
// It is used to parse the response body and extract individual events.
type Reader struct {
	resp   *protocol.Response
	s      *bufio.Scanner
	events int32
}

// NewReader creates a new SSE reader from the given response.
//
// It returns an error if the response's content type is not text/event-stream.
func NewReader(resp *protocol.Response) (*Reader, error) {
	if !bytes.HasPrefix(resp.Header.ContentType(), bytestr.MIMETextEventStream) {
		return nil, errNotSSEContentType
	}
	r := &Reader{resp: resp}
	if resp.IsBodyStream() {
		r.s = bufio.NewScanner(resp.BodyStream())
	} else {
		r.s = bufio.NewScanner(bytes.NewReader(resp.Body()))
	}
	return r, nil
}

// ReadEvent reads a single SSE event from the response body.
//
// It populates the provided Event struct with the parsed data.
// Returns nil if an event was successfully read, or an error otherwise.
func (r *Reader) ReadEvent(e *Event) error {
	e.Reset()
	for i := 0; r.s.Scan(); i++ {
		line := r.s.Bytes()

		// Trim UTF8 BOM
		if i == 0 && r.events == 0 && bytes.HasPrefix(line, []byte{0xEF, 0xBB, 0xBF}) {
			line = line[3:]
		}

		if len(line) == 0 {
			// Empty line marks the end of an event
			if e.bitset != 0 {
				r.events++
				return nil
			}
			continue // Skip empty lines at the beginning
		}

		if line[0] == ':' {
			// Comment which starts with colon
			continue
		}

		// Parse field
		var f, v []byte
		i := bytes.IndexByte(line, ':')
		if i < 0 {
			// No colon, the entire line is the field name with an empty value
			f = line
		} else {
			f = line[:i]
			// If the colon is followed by a space, remove it
			if i+1 < len(line) && line[i+1] == ' ' {
				v = line[i+2:]
			} else {
				v = line[i+1:]
			}
		}

		// Process the field
		switch string(f) {
		case "event":
			e.SetEvent(sseEventType(v))
		case "data":
			if len(e.Data) > 0 {
				// If we already have data, append a newline before the new data
				e.Data = append(e.Data, '\n')
			}
			e.AppendData(v)
		case "id":
			id := string(v)
			// Ignore if it contains Null
			if !strings.Contains(id, "\u0000") {
				e.SetID(id)
			}
		case "retry":
			if retry, err := strconv.ParseInt(string(v), 10, 64); err == nil {
				e.SetRetry(time.Duration(retry) * time.Millisecond)
			}
		default:
			// As per spec, ignore if it's not defined.
		}
	}
	// Check if scanner encountered an error
	if err := r.s.Err(); err != nil {
		return err
	}
	if e.bitset == 0 {
		return io.EOF
	}
	r.events++
	return nil
}

// Close closes the underlying response body.
func (r *Reader) Close() error {
	return r.resp.CloseBodyStream()
}
