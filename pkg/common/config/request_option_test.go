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
)

// TestRequestOptions test request options with custom values
func TestRequestOptions(t *testing.T) {
	opt := NewRequestOptions([]RequestOption{
		WithTag("a", "b"),
		WithTag("c", "d"),
		WithTag("e", "f"),
		WithSD(true),
		WithDialTimeout(time.Second),
		WithReadTimeout(time.Second),
		WithWriteTimeout(time.Second),
	})
	assert.DeepEqual(t, "b", opt.Tag("a"))
	assert.DeepEqual(t, "d", opt.Tag("c"))
	assert.DeepEqual(t, "f", opt.Tag("e"))
	assert.DeepEqual(t, time.Second, opt.DialTimeout())
	assert.DeepEqual(t, time.Second, opt.ReadTimeout())
	assert.DeepEqual(t, time.Second, opt.WriteTimeout())
	assert.True(t, opt.IsSD())
}

// TestRequestOptionsWithDefaultOpts test request options with default values
func TestRequestOptionsWithDefaultOpts(t *testing.T) {
	SetPreDefinedOpts(WithTag("pre-defined", "blablabla"), WithTag("a", "default-value"), WithSD(true))
	opt := NewRequestOptions([]RequestOption{
		WithTag("a", "b"),
		WithSD(false),
	})
	assert.DeepEqual(t, "b", opt.Tag("a"))
	assert.DeepEqual(t, "blablabla", opt.Tag("pre-defined"))
	assert.DeepEqual(t, map[string]string{
		"a":           "b",
		"pre-defined": "blablabla",
	}, opt.Tags())
	assert.False(t, opt.IsSD())
	SetPreDefinedOpts()
	assert.Nil(t, preDefinedOpts)
	assert.DeepEqual(t, time.Duration(0), opt.WriteTimeout())
	assert.DeepEqual(t, time.Duration(0), opt.ReadTimeout())
	assert.DeepEqual(t, time.Duration(0), opt.DialTimeout())
}

// TestRequestOptions_CopyTo test request options copy to another one
func TestRequestOptions_CopyTo(t *testing.T) {
	opt := NewRequestOptions([]RequestOption{
		WithTag("a", "b"),
		WithSD(false),
	})
	var copyOpt RequestOptions
	opt.CopyTo(&copyOpt)
	assert.DeepEqual(t, opt.Tags(), copyOpt.Tags())
	assert.DeepEqual(t, opt.IsSD(), copyOpt.IsSD())
}
