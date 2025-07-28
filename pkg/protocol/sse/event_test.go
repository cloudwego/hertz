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
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestEvent_SetAndIsSet(t *testing.T) {
	e := NewEvent()
	defer e.Release()

	// Test initial state
	assert.Assert(t, !e.IsSetID())
	assert.Assert(t, !e.IsSetType())
	assert.Assert(t, !e.IsSetRetry())
	assert.Assert(t, !e.IsSetData())

	// Test SetID and IsSetID
	e.SetID("test-id")
	assert.Assert(t, e.IsSetID())
	assert.DeepEqual(t, "test-id", e.ID)

	// Test SetEvent and IsSetType
	e.SetEvent("test-event")
	assert.Assert(t, e.IsSetType())
	assert.DeepEqual(t, "test-event", e.Type)

	// Test SetRetry and IsSetRetry
	r := 3 * time.Second
	e.SetRetry(r)
	assert.Assert(t, e.IsSetRetry())
	assert.DeepEqual(t, r, e.Retry)

	// Test SetData and IsSetData
	d := []byte("test-data")
	e.SetData(d)
	assert.Assert(t, e.IsSetData())
	assert.DeepEqual(t, d, e.Data)
	e.Reset()
	assert.Assert(t, e.IsSetData() == false)
	e.SetDataString(string(d))
	assert.Assert(t, e.IsSetData())
	assert.DeepEqual(t, d, e.Data)
}

func TestEvent_AppendData(t *testing.T) {
	e := NewEvent()
	defer e.Release()

	// Test AppendData
	e.AppendData([]byte("first"))
	assert.Assert(t, e.IsSetData())
	assert.DeepEqual(t, []byte("first"), e.Data)

	// Append more data
	e.AppendDataString("second")
	assert.DeepEqual(t, []byte("firstsecond"), e.Data)
}

func TestEvent_Reset(t *testing.T) {
	e := NewEvent()
	defer e.Release()

	// Set all fields
	e.SetID("test-id")
	e.SetEvent("test-event")
	e.SetRetry(3 * time.Second)
	e.SetData([]byte("test-data"))

	// Verify all fields are set
	assert.Assert(t, e.IsSetID())
	assert.Assert(t, e.IsSetType())
	assert.Assert(t, e.IsSetRetry())
	assert.Assert(t, e.IsSetData())

	// Reset and verify all fields are cleared
	e.Reset()
	assert.Assert(t, !e.IsSetID())
	assert.Assert(t, !e.IsSetType())
	assert.Assert(t, !e.IsSetRetry())
	assert.Assert(t, !e.IsSetData())
	assert.DeepEqual(t, "", e.ID)
	assert.DeepEqual(t, "", e.Type)
	assert.DeepEqual(t, time.Duration(0), e.Retry)
	assert.DeepEqual(t, 0, len(e.Data))
}

func TestEvent_Clone(t *testing.T) {
	e1 := NewEvent()
	e1.SetID("test-id")
	e1.SetEvent("test-event")
	e1.SetRetry(3 * time.Second)
	e1.SetData([]byte("test-data"))
	e2 := e1.Clone()
	assert.Assert(t, e2.IsSetID())
	assert.Assert(t, e2.IsSetType())
	assert.Assert(t, e2.IsSetRetry())
	assert.Assert(t, e2.IsSetData())
	assert.DeepEqual(t, "test-id", e2.ID)
	assert.DeepEqual(t, "test-event", e2.Type)
	assert.DeepEqual(t, 3*time.Second, e2.Retry)
	assert.DeepEqual(t, []byte("test-data"), e2.Data)
	e1.Release()
	e2.Release()
}

func TestEvent_PoolAndRelease(t *testing.T) {
	e1 := NewEvent()
	e1.SetID("test-id")
	e1.Release()

	// Get another event from the pool, should be the same instance but reset
	e2 := NewEvent()
	assert.Assert(t, !e2.IsSetID())
	assert.DeepEqual(t, "", e2.ID)
	e2.Release()
}
