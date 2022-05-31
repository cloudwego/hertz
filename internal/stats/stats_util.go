/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stats

import (
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	"github.com/cloudwego/hertz/pkg/common/tracer/traceinfo"
)

// Record records the event to HTTPStats.
func Record(ti traceinfo.TraceInfo, event stats.Event, err error) {
	if ti == nil {
		return
	}
	if err != nil {
		ti.Stats().Record(event, stats.StatusError, err.Error())
	} else {
		ti.Stats().Record(event, stats.StatusInfo, "")
	}
}

// CalcEventCostUs calculates the duration between start and end and returns in microsecond.
func CalcEventCostUs(start, end traceinfo.Event) uint64 {
	if start == nil || end == nil || start.IsNil() || end.IsNil() {
		return 0
	}
	return uint64(end.Time().Sub(start.Time()).Microseconds())
}
