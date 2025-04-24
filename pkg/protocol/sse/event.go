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
	"fmt"
	"sync"
	"time"
)

const (
	fieldID = 1 << iota
	fieldType
	fieldData
	fieldRetry
)

// Event represents a Server-Sent Event (SSE).
type Event struct {
	ID   string
	Type string // aka `event` field, which means event type
	Data []byte

	// hertz only supports reading and writing the field,
	// and will not take care of retry policy, please implement on your own.
	Retry time.Duration

	bitset uint8
}

var poolEvent = sync.Pool{}

// NewEvent creates a new event.
//
// Call `Release` when you're done with the event.
func NewEvent() *Event {
	if v := poolEvent.Get(); v != nil {
		ret := v.(*Event)
		ret.Reset()
		return ret
	}
	return &Event{}
}

// Release releases the event back to the pool.
func (e *Event) Release() {
	poolEvent.Put(e)
}

// String returns a string representation of the event.
func (e *Event) String() string {
	return fmt.Sprintf("Event{ID:%q, Type:%q, Retry:%s, Data:%q}", e.ID, e.Type, e.Retry, e.Data)
}

// Reset resets the event fields.
func (e *Event) Reset() {
	e.ID = ""
	e.Type = ""
	e.Retry = time.Duration(0)
	e.Data = e.Data[:0]
	e.bitset = 0
}

// Clone creates a copy of the event.
//
// When it's no longer needed, call `Release` to return it to the pool.
func (e *Event) Clone() *Event {
	p := NewEvent()
	p.ID = e.ID
	p.Type = e.Type
	p.Retry = e.Retry
	p.Data = append(p.Data[:0], e.Data...)
	p.bitset = e.bitset
	return p
}

// IsSetID returns true if the event ID is set.
//
// Please use SetID to set the event ID for differentiating notset or empty
func (e *Event) IsSetID() bool {
	return e.bitset&fieldID != 0 || len(e.ID) > 0
}

// IsSetType returns true if the event type is set.
//
// Please use SetEvent to set the event type for differentiating notset or empty
func (e *Event) IsSetType() bool {
	return e.bitset&fieldType != 0 || len(e.Type) > 0
}

// IsSetRetry returns true if the retry duration is set.
//
// Please use SetRetry to set the event retry duration for differentiating notset or empty
func (e *Event) IsSetRetry() bool {
	return e.bitset&fieldRetry != 0 || e.Retry != 0
}

// IsSetData returns true if the event data is set.
//
// Please use SetData to set or AppendData to append the event data for differentiating notset or empty
func (e *Event) IsSetData() bool {
	return e.bitset&fieldData != 0 || len(e.Data) > 0
}

// SetID sets the event ID.
func (e *Event) SetID(id string) {
	e.ID = id
	e.bitset |= fieldID
}

// SetEvent sets the event type.
func (e *Event) SetEvent(eventType string) {
	e.Type = eventType
	e.bitset |= fieldType
}

// SetData sets the event data.
func (e *Event) SetData(data []byte) {
	e.Data = append(e.Data[:0], data...)
	e.bitset |= fieldData
}

// SetDataString sets the event data from a string.
func (e *Event) SetDataString(data string) {
	e.Data = append(e.Data[:0], data...)
	e.bitset |= fieldData
}

// AppendData appends data to the event data.
func (e *Event) AppendData(data []byte) {
	e.Data = append(e.Data, data...)
	e.bitset |= fieldData
}

// AppendDataString appends string data to the event data.
func (e *Event) AppendDataString(data string) {
	e.Data = append(e.Data, data...)
	e.bitset |= fieldData
}

// SetRetry sets the retry duration.
func (e *Event) SetRetry(retry time.Duration) {
	e.Retry = retry
	e.bitset |= fieldRetry
}
