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
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

// mockWriteFlusher implements the writeFlusher interface for testing
type mw struct {
	buf         bytes.Buffer
	flushCalled bool
	writeErr    error
	flushErr    error
	finalizeErr error
}

func (m *mw) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.buf.Write(p)
}

func (m *mw) Flush() error {
	m.flushCalled = true
	return m.flushErr
}

func (m *mw) String() string {
	return m.buf.String()
}

func (m *mw) Finalize() error {
	return m.finalizeErr
}

func TestWriter_WriteEvent(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		eventType string
		data      []byte
		wantErr   bool
		expected  string
	}{
		{
			name:      "Basic event",
			id:        "123",
			eventType: "message",
			data:      []byte("test data"),
			wantErr:   false,
			expected:  "id: 123\nevent: message\ndata: test data\n\n",
		},
		{
			name:      "Empty fields",
			id:        "",
			eventType: "",
			data:      nil,
			wantErr:   false,
			expected:  "\n",
		},
		{
			name:      "ID with CRLF",
			id:        "test\nid",
			eventType: "update",
			data:      []byte("test data"),
			wantErr:   true,
			expected:  "",
		},
		{
			name:      "Event type with CRLF",
			id:        "123",
			eventType: "up\ndate",
			data:      []byte("test data"),
			wantErr:   true,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mw{}
			w := &Writer{w: m}

			err := w.WriteEvent(tt.id, tt.eventType, tt.data)

			if tt.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.Assert(t, err == nil)
				assert.DeepEqual(t, tt.expected, m.String())
				assert.Assert(t, m.flushCalled)
			}
		})
	}
}

func TestWriter_Write(t *testing.T) {
	tests := []struct {
		name     string
		event    *Event
		writeErr error
		flushErr error
		wantErr  bool
		expected string
	}{
		{
			name: "Complete event",
			event: func() *Event {
				e := NewEvent()
				e.SetID("123")
				e.SetEvent("update")
				e.SetRetry(3 * time.Second)
				e.SetData([]byte("test data"))
				return e
			}(),
			wantErr:  false,
			expected: "id: 123\nevent: update\nretry: 3000\ndata: test data\n\n",
		},
		{
			name: "Multiline data",
			event: func() *Event {
				e := NewEvent()
				e.SetData([]byte("line1\rline2\nline3\r\nline4"))
				return e
			}(),
			wantErr:  false,
			expected: "data: line1\ndata: line2\ndata: line3\ndata: line4\n\n",
		},
		{
			name: "Write error",
			event: func() *Event {
				e := NewEvent()
				e.SetData([]byte("test data"))
				return e
			}(),
			writeErr: errors.New("write error"),
			wantErr:  true,
			expected: "",
		},
		{
			name: "Flush error",
			event: func() *Event {
				e := NewEvent()
				e.SetData([]byte("test data"))
				return e
			}(),
			flushErr: errors.New("flush error"),
			wantErr:  true,
			expected: "data: test data\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mw{
				writeErr: tt.writeErr,
				flushErr: tt.flushErr,
			}
			w := &Writer{w: m}

			err := w.Write(tt.event)

			if tt.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.Assert(t, err == nil)
				assert.DeepEqual(t, tt.expected, m.String())
				assert.Assert(t, m.flushCalled)
			}
		})
	}
}

func TestNewWriter(t *testing.T) {
	c := app.NewContext(0)
	w := NewWriter(c)
	assert.Assert(t, w != nil)
	assert.DeepEqual(t, "no-cache", string(c.Response.Header.Peek("Cache-Control")))
	assert.DeepEqual(t, "text/event-stream; charset=utf-8", string(c.Response.Header.Peek("Content-Type")))
}

func TestWriter_WriteComment(t *testing.T) {
	m := &mw{}
	w := &Writer{w: m}

	err := w.WriteComment("test\ncomment")

	assert.Assert(t, err == nil)
	assert.DeepEqual(t, ":test\n:comment\n", m.String())
	assert.Assert(t, m.flushCalled)

	// empty string
	m = &mw{}
	w = &Writer{w: m}
	err = w.WriteComment("")
	assert.Assert(t, err == nil)
	assert.DeepEqual(t, ":", m.String())

	// keep-alive
	m = &mw{}
	w = &Writer{w: m}
	err = w.WriteKeepAlive()
	assert.Assert(t, err == nil)
	assert.DeepEqual(t, ":keep-alive\n", m.String())
}

func TestWriter_Close(t *testing.T) {
	// Create a mock writeFlusher
	m := &mw{}

	// Create a Writer with the mock
	w := &Writer{w: m}

	// Test Close method
	err := w.Close()

	// Verify no error occurred
	assert.Nil(t, err)

	// Set an error to be returned by Finalize
	expectedErr := errors.New("finalize error")
	m.finalizeErr = expectedErr

	// Test Close method with error
	err = w.Close()

	// Verify the error is propagated
	assert.DeepEqual(t, expectedErr, err)
}
