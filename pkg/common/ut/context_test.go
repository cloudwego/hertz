/*
 * Copyright 2023 CloudWeGo Authors
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

package ut

import (
	"bytes"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestCreateUtRequestContext(t *testing.T) {
	body := "1"
	method := "PUT"
	path := "/hey/dy"
	headerKey := "Connection"
	headerValue := "close"
	ctx := CreateUtRequestContext(method, path, &Body{bytes.NewBufferString(body), len(body)},
		Header{headerKey, headerValue})

	assert.DeepEqual(t, method, string(ctx.Method()))
	assert.DeepEqual(t, path, string(ctx.Path()))
	body1, err := ctx.Body()
	assert.DeepEqual(t, nil, err)
	assert.DeepEqual(t, body, string(body1))
	assert.DeepEqual(t, headerValue, string(ctx.GetHeader(headerKey)))
}
