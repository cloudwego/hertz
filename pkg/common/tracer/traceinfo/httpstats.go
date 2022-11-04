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
 */

package traceinfo

import (
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
)

var _ HTTPStats = (*httpStats)(nil)

var (
	eventPool   sync.Pool
	once        sync.Once
	maxEventNum int
)

type event struct {
	event  stats.Event
	status stats.Status
	info   string
	time   time.Time
}

// Event implements the Event interface.
func (e *event) Event() stats.Event {
	return e.event
}

// Status implements the Event interface.
func (e *event) Status() stats.Status {
	return e.status
}

// Info implements the Event interface.
func (e *event) Info() string {
	return e.info
}

// Time implements the Event interface.
func (e *event) Time() time.Time {
	return e.time
}

// IsNil implements the Event interface.
func (e *event) IsNil() bool {
	return e == nil
}

func newEvent() interface{} {
	return &event{}
}

func (e *event) zero() {
	e.event = nil
	e.status = 0
	e.info = ""
	e.time = time.Time{}
}

// Recycle reuses the event.
func (e *event) Recycle() {
	e.zero()
	eventPool.Put(e)
}

type httpStats struct {
	sync.RWMutex
	level stats.Level

	eventMap []Event

	sendSize int
	recvSize int

	err      error
	panicErr interface{}
}

func init() {
	eventPool.New = newEvent
}

// Record implements the HTTPStats interface.
func (h *httpStats) Record(e stats.Event, status stats.Status, info string) {
	if e.Level() > h.level {
		return
	}
	eve := eventPool.Get().(*event)
	eve.event = e
	eve.status = status
	eve.info = info
	eve.time = time.Now()

	idx := e.Index()
	h.Lock()
	h.eventMap[idx] = eve
	h.Unlock()
}

// SendSize implements the HTTPStats interface.
func (h *httpStats) SendSize() int {
	return h.sendSize
}

// RecvSize implements the HTTPStats interface.
func (h *httpStats) RecvSize() int {
	return h.recvSize
}

// Error implements the HTTPStats interface.
func (h *httpStats) Error() error {
	return h.err
}

// Panicked implements the HTTPStats interface.
func (h *httpStats) Panicked() (bool, interface{}) {
	return h.panicErr != nil, h.panicErr
}

// GetEvent implements the HTTPStats interface.
func (h *httpStats) GetEvent(e stats.Event) Event {
	idx := e.Index()
	h.RLock()
	evt := h.eventMap[idx]
	h.RUnlock()
	if evt == nil || evt.IsNil() {
		return nil
	}
	return evt
}

// Level implements the HTTPStats interface.
func (h *httpStats) Level() stats.Level {
	return h.level
}

// SetSendSize sets send size.
func (h *httpStats) SetSendSize(size int) {
	h.sendSize = size
}

// SetRecvSize sets recv size.
func (h *httpStats) SetRecvSize(size int) {
	h.recvSize = size
}

// SetError sets error.
func (h *httpStats) SetError(err error) {
	h.err = err
}

// SetPanicked sets if panicked.
func (h *httpStats) SetPanicked(x interface{}) {
	h.panicErr = x
}

// SetLevel sets the level.
func (h *httpStats) SetLevel(level stats.Level) {
	h.level = level
}

// Reset resets the stats.
func (h *httpStats) Reset() {
	h.err = nil
	h.panicErr = nil
	h.recvSize = 0
	h.sendSize = 0
	for i := range h.eventMap {
		if h.eventMap[i] != nil {
			h.eventMap[i].(*event).Recycle()
			h.eventMap[i] = nil
		}
	}
}

// ImmutableView restricts the httpStats into a read-only traceinfo.HTTPStats.
func (h *httpStats) ImmutableView() HTTPStats {
	return h
}

// NewHTTPStats creates a new HTTPStats.
func NewHTTPStats() HTTPStats {
	once.Do(func() {
		stats.FinishInitialization()
		maxEventNum = stats.MaxEventNum()
	})
	return &httpStats{
		eventMap: make([]Event, maxEventNum),
	}
}
