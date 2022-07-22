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

package client

import (
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestClientOptions(t *testing.T) {
	opt := config.NewClientOptions([]config.ClientOption{
		WithDialTimeout(100 * time.Millisecond),
		WithMaxConnsPerHost(128),
		WithMaxIdleConnDuration(5 * time.Second),
		WithMaxConnDuration(10 * time.Second),
		WithMaxConnWaitTimeout(5 * time.Second),
		WithKeepAlive(false),
		WithClientReadTimeout(1 * time.Second),
		WithResponseBodyStream(true),
	})
	assert.DeepEqual(t, 100*time.Millisecond, opt.DialTimeout)
	assert.DeepEqual(t, 128, opt.MaxConnsPerHost)
	assert.DeepEqual(t, 5*time.Second, opt.MaxIdleConnDuration)
	assert.DeepEqual(t, 10*time.Second, opt.MaxConnDuration)
	assert.DeepEqual(t, 5*time.Second, opt.MaxConnWaitTimeout)
	assert.DeepEqual(t, false, opt.KeepAlive)
	assert.DeepEqual(t, 1*time.Second, opt.ReadTimeout)
	assert.DeepEqual(t, true, opt.ResponseBodyStream)
}
