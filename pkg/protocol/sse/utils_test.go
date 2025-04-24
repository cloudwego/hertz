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

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

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
