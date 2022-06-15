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

package bytesconv

import (
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestAppendDate(t *testing.T) {
	t.Parallel()
	// GMT+8
	shanghaiTimeZone := time.FixedZone("Asia/Shanghai", 8*60*60)

	for _, c := range []struct {
		name    string
		date    time.Time
		dateStr string
	}{
		{
			name:    "UTC",
			date:    time.Date(2022, 6, 15, 11, 12, 13, 123, time.UTC),
			dateStr: "Wed, 15 Jun 2022 11:12:13 GMT",
		},
		{
			name:    "Asia/Shanghai",
			date:    time.Date(2022, 6, 15, 3, 12, 45, 999, shanghaiTimeZone),
			dateStr: "Tue, 14 Jun 2022 19:12:45 GMT",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := AppendHTTPDate(nil, c.date)
			assert.DeepEqual(t, c.dateStr, string(s))
		})
	}
}
