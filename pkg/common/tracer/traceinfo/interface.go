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
	"time"

	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
)

// HTTPStats is used to collect statistics about the HTTP.
type HTTPStats interface {
	Record(event stats.Event, status stats.Status, info string)
	GetEvent(event stats.Event) Event
	SendSize() int
	SetSendSize(size int)
	RecvSize() int
	SetRecvSize(size int)
	Error() error
	SetError(err error)
	Panicked() (bool, interface{})
	SetPanicked(x interface{})
	Level() stats.Level
	SetLevel(level stats.Level)
	Reset()
}

// Event is the abstraction of an event happened at a specific time.
type Event interface {
	Event() stats.Event
	Status() stats.Status
	Info() string
	Time() time.Time
	IsNil() bool
}

// TraceInfo contains the trace message in Hertz.
type TraceInfo interface {
	Stats() HTTPStats
	Reset()
}
