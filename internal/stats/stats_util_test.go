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
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	"github.com/cloudwego/hertz/pkg/common/tracer/traceinfo"
)

func TestUtil(t *testing.T) {
	assert.Assert(t, CalcEventCostUs(nil, nil) == 0)

	ti := traceinfo.NewTraceInfo()

	// nil context
	Record(ti, stats.HTTPStart, nil)
	Record(ti, stats.HTTPFinish, nil)

	st := ti.Stats()
	assert.Assert(t, st != nil)

	s, e := st.GetEvent(stats.HTTPStart), st.GetEvent(stats.HTTPFinish)
	assert.Assert(t, s == nil)
	assert.Assert(t, e == nil)

	// stats disabled
	Record(ti, stats.HTTPStart, nil)
	time.Sleep(time.Millisecond)
	Record(ti, stats.HTTPFinish, nil)

	st = ti.Stats()
	assert.Assert(t, st != nil)

	s, e = st.GetEvent(stats.HTTPStart), st.GetEvent(stats.HTTPFinish)
	assert.Assert(t, s == nil)
	assert.Assert(t, e == nil)

	// stats enabled
	st = ti.Stats()
	st.(interface{ SetLevel(stats.Level) }).SetLevel(stats.LevelBase)

	Record(ti, stats.HTTPStart, nil)
	time.Sleep(time.Millisecond)
	Record(ti, stats.HTTPFinish, nil)

	s, e = st.GetEvent(stats.HTTPStart), st.GetEvent(stats.HTTPFinish)
	assert.Assert(t, s != nil, s)
	assert.Assert(t, e != nil, e)
	assert.Assert(t, CalcEventCostUs(s, e) > 0)
}
