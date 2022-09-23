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

package config

import (
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// TestDefaultClientOptions test client options with default values
func TestDefaultClientOptions(t *testing.T) {
	options := NewClientOptions([]ClientOption{})

	assert.DeepEqual(t, consts.DefaultDialTimeout, options.DialTimeout)
	assert.DeepEqual(t, consts.DefaultMaxConnsPerHost, options.MaxConnsPerHost)
	assert.DeepEqual(t, consts.DefaultMaxIdleConnDuration, options.MaxIdleConnDuration)
	assert.DeepEqual(t, true, options.KeepAlive)
}

// TestCustomClientOptions test client options with custom values
func TestCustomClientOptions(t *testing.T) {
	options := NewClientOptions([]ClientOption{})

	options.Apply([]ClientOption{
		{
			F: func(o *ClientOptions) {
				o.DialTimeout = 2 * time.Second
			},
		},
	})
	assert.DeepEqual(t, 2*time.Second, options.DialTimeout)
}
