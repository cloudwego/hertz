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
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
)

type mockBodyStream struct {
	reader io.Reader
	closed bool
}

func (m *mockBodyStream) Read(p []byte) (n int, err error) {
	return m.reader.Read(p)
}

func (m *mockBodyStream) Close() error {
	m.closed = true
	return nil
}

func TestNewReader(t *testing.T) {
	tests := []struct {
		name        string
		contentType []byte
		body        []byte
		wantErr     bool
	}{
		{
			name:        "Valid content type",
			contentType: bytestr.MIMETextEventStream,
			body:        []byte("event: message\ndata: test\n\n"),
			wantErr:     false,
		},
		{
			name:        "Invalid content type",
			contentType: []byte("text/plain"),
			body:        []byte("event: message\ndata: test\n\n"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &protocol.Response{}
			resp.Header.SetContentType(string(tt.contentType))
			resp.SetBody(tt.body)

			r, err := NewReader(resp)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				assert.Assert(t, r == nil)
			} else {
				assert.Assert(t, err == nil)
				assert.Assert(t, r != nil)
			}
		})
	}
}

func TestReader_ReadEvent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Event
		wantErr  bool
	}{
		{
			name:  "Basic event",
			input: "id: 123\nevent: update\ndata: test data\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetID("123")
				e.SetEvent("update")
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Event with retry",
			input: "id: 123\nevent: update\nretry: 3000\ndata: test data\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetID("123")
				e.SetEvent("update")
				e.SetRetry(3000 * time.Millisecond)
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Event with multiline data",
			input: "id: 123\revent: update\r\ndata: line1\rdata: line2\r\ndata: line3\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetID("123")
				e.SetEvent("update")
				e.SetData([]byte("line1\nline2\nline3"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Event with BOM",
			input: "\xEF\xBB\xBFid: 123\nevent: update\ndata: test data\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetID("123")
				e.SetEvent("update")
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Event with comments",
			input: ": this is a comment\nid: 123\n: another comment\nevent: update\ndata: test data\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetID("123")
				e.SetEvent("update")
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Event with no colon in field",
			input: "id\nevent: update\ndata: test data\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetID("")
				e.SetEvent("update")
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Event with no space after colon",
			input: "id:123\nevent:update\ndata:test data\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetID("123")
				e.SetEvent("update")
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Event with ID containing null character (should be ignored)",
			input: "id: test\u0000id\nevent: update\ndata: test data\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetEvent("update")
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Event with invalid retry value",
			input: "id: 123\nevent: update\nretry: invalid\ndata: test data\n\n",
			expected: func() *Event {
				e := NewEvent()
				e.SetID("123")
				e.SetEvent("update")
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr: false,
		},
		{
			name:  "Empty event",
			input: "\n\n",
			expected: func() *Event {
				e := NewEvent()
				// Empty event doesn't set any fields, so bitset remains 0
				return e
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &protocol.Response{}
			resp.Header.SetContentType(string(bytestr.MIMETextEventStream))
			resp.SetBody([]byte(tt.input))

			r, err := NewReader(resp)
			assert.Assert(t, err == nil)

			e := NewEvent()
			err = r.Read(e)

			if tt.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.Assert(t, err == nil)
				assert.DeepEqual(t, tt.expected.ID, e.ID)
				assert.DeepEqual(t, tt.expected.Type, e.Type)
				assert.DeepEqual(t, tt.expected.Retry, e.Retry)
				assert.DeepEqual(t, tt.expected.Data, e.Data)

				// LastEventID check
				if e.ID != "" {
					assert.DeepEqual(t, r.LastEventID(), e.ID)
				}
			}

			e.Release()
		})
	}
}

func TestReader_ReadEvent_WithBodyStream(t *testing.T) {
	input := "id: 123\nevent: update\ndata: test data\n\n"

	resp := &protocol.Response{}
	resp.Header.SetContentType(string(bytestr.MIMETextEventStream))

	// Create a mock body stream
	ms := &mockBodyStream{
		reader: strings.NewReader(input),
	}
	resp.SetBodyStream(ms, -1)

	r, err := NewReader(resp)
	assert.Assert(t, err == nil)

	e := NewEvent()
	err = r.Read(e)
	assert.Assert(t, err == nil)

	// Verify event data
	assert.DeepEqual(t, "123", e.ID)
	assert.DeepEqual(t, "update", e.Type)
	assert.DeepEqual(t, []byte("test data"), e.Data)

	// LastEventID check
	if e.ID != "" {
		assert.DeepEqual(t, r.LastEventID(), e.ID)
	}

	// Test Close
	err = r.Close()
	assert.Assert(t, err == nil)
	assert.Assert(t, ms.closed)

	e.Release()
}

type mockReadForceClose struct {
	readFunc  func(b []byte) (int, error)
	closeFunc func() error
}

func (m *mockReadForceClose) Read(b []byte) (int, error) {
	return m.readFunc(b)
}

func (m *mockReadForceClose) ForceClose() error {
	return m.closeFunc()
}

func TestReader_ReadEvent_Error(t *testing.T) {
	// Create a reader that will return an error
	errReader := &bytes.Reader{}

	resp := &protocol.Response{}
	resp.Header.SetContentType(string(bytestr.MIMETextEventStream))
	resp.SetBodyStream(errReader, -1)

	r, err := NewReader(resp)
	assert.Assert(t, err == nil)

	e := NewEvent()
	err = r.Read(e)

	// The error from bytes.Reader will be io.EOF
	assert.Assert(t, err == io.EOF)

	e.Release()
}

func TestReader_ForEach(t *testing.T) {
	// mock Read & ForceClose
	mr := &mockReadForceClose{}
	ch := make(chan error, 1)
	defer close(ch)
	mr.readFunc = func(b []byte) (int, error) {
		return 0, <-ch
	}
	mr.closeFunc = func() error {
		ch <- errors.New("closed")
		return nil
	}

	// create protocol.Response
	resp := &protocol.Response{}
	resp.Header.SetContentType(string(bytestr.MIMETextEventStream))
	resp.SetBodyStream(mr, -1)
	r, err := NewReader(resp)
	assert.Assert(t, err == nil)

	// test ForEach with context
	ctx, cancel := context.WithCancel(context.Background())
	go func() { // cancel after 50ms
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	err = r.ForEach(ctx, func(e *Event) error {
		panic("must not called")
	})
	assert.Assert(t, err == ctx.Err())
}

func TestReader_SetMaxBufferSize(t *testing.T) {
	// Test that default buffer size fails for events > 64KB
	t.Run("default buffer size fails for large events", func(t *testing.T) {
		// Create a response with a large event (65KB) - just over default 64KB
		largeData := strings.Repeat("x", 65*1024)
		input := "event: large\ndata: " + largeData + "\n\n"

		resp := &protocol.Response{}
		resp.Header.SetContentType(string(bytestr.MIMETextEventStream))
		resp.SetBody([]byte(input))

		r, err := NewReader(resp)
		assert.Assert(t, err == nil)

		// Don't call SetMaxBufferSize, use default (64KB)
		// Reading should fail because the line is too long
		e := NewEvent()
		err = r.Read(e)
		assert.Assert(t, errors.Is(err, bufio.ErrTooLong))
		e.Release()
	})

	// Test with custom buffer size for large events
	t.Run("custom buffer size", func(t *testing.T) {
		// Create a response with a large event (65KB) - just over default 64KB
		largeData := strings.Repeat("x", 65*1024)
		input := "event: large\ndata: " + largeData + "\n\n"

		resp := &protocol.Response{}
		resp.Header.SetContentType(string(bytestr.MIMETextEventStream))
		resp.SetBody([]byte(input))

		r, err := NewReader(resp)
		assert.Assert(t, err == nil)

		// Set max buffer size to 70KB to handle the large event
		r.SetMaxBufferSize(70 * 1024)

		// Should be able to read the large event
		e := NewEvent()
		err = r.Read(e)
		assert.Assert(t, err == nil)
		assert.DeepEqual(t, "large", e.Type)
		assert.DeepEqual(t, largeData, string(e.Data))
		e.Release()

		// Test panic when SetMaxBufferSize is called after reading
		defer func() {
			if r := recover(); r == nil {
				t.Error("SetMaxBufferSize should panic after reading has started")
			}
		}()
		r.SetMaxBufferSize(80 * 1024)
	})
}
