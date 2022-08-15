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

package dialer

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

func TestGetDialerName(t *testing.T) {
	dName := GetDialerName()
	assert.DeepEqual(t, "netpoll", dName)

	defaultDialer = standard.NewDialer()
	dName = GetDialerName()
	assert.DeepEqual(t, "standard", dName)

	defaultDialer = nil
	dName = GetDialerName()
	assert.DeepEqual(t, "unknown", dName)
}
