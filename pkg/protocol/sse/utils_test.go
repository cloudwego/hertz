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
	"testing"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func TestSetGetLastEventID(t *testing.T) {
	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)

	SetLastEventID(req, "123")
	assert.DeepEqual(t, "123", GetLastEventID(req))
}

func TestAddAcceptMIME(t *testing.T) {
	// Test case 1: Empty Accept header
	req := protocol.AcquireRequest()
	defer protocol.ReleaseRequest(req)

	AddAcceptMIME(req)

	acceptHeader := req.Header.Peek("Accept")
	assert.DeepEqual(t, string(bytestr.MIMETextEventStream), string(acceptHeader))

	// Test case 2: Existing Accept header without text/event-stream
	req.Reset()
	req.Header.Set("Accept", "text/html, application/json")

	AddAcceptMIME(req)

	acceptHeader = req.Header.Peek("Accept")
	assert.DeepEqual(t, "text/html, application/json, text/event-stream", string(acceptHeader))

	// Test case 3: Existing Accept header already containing text/event-stream
	req.Reset()
	req.Header.Set("Accept", "text/html, text/event-stream, application/json")

	AddAcceptMIME(req)

	acceptHeader = req.Header.Peek("Accept")
	assert.DeepEqual(t, "text/html, text/event-stream, application/json", string(acceptHeader))
}

func TestHasCRLF(t *testing.T) {
	assert.Assert(t, hasCRLF("\nThis is a test string"))
	assert.Assert(t, hasCRLF("This is \na test string"))
	assert.Assert(t, hasCRLF("This is a test string\n"))
	assert.Assert(t, hasCRLF("\rThis is a test string"))
	assert.Assert(t, hasCRLF("This is \rna test string"))
	assert.Assert(t, hasCRLF("This is a test string\r"))
	assert.Assert(t, hasCRLF("This is a test string") == false)
}

func TestSseEventType(t *testing.T) {
	assert.DeepEqual(t, "message", sseEventType([]byte("message")))
	assert.DeepEqual(t, "custom", sseEventType([]byte("custom")))
}
