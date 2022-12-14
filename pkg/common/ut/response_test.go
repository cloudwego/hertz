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

package ut

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestResult(t *testing.T) {
	r := new(ResponseRecorder)
	ret := r.Result()
	assert.DeepEqual(t, consts.StatusOK, ret.StatusCode())
}

func TestFlush(t *testing.T) {
	r := new(ResponseRecorder)
	r.Flush()
	ret := r.Result()
	assert.DeepEqual(t, consts.StatusOK, ret.StatusCode())
}

func TestWriterHeader(t *testing.T) {
	r := NewRecorder()
	r.WriteHeader(consts.StatusCreated)
	r.WriteHeader(consts.StatusOK)
	ret := r.Result()
	assert.DeepEqual(t, consts.StatusCreated, ret.StatusCode())
}

func TestWriteString(t *testing.T) {
	r := NewRecorder()
	r.WriteString("hello")
	ret := r.Result()
	assert.DeepEqual(t, "hello", string(ret.Body()))
}

func TestWrite(t *testing.T) {
	r := NewRecorder()
	r.Write([]byte("hello"))
	ret := r.Result()
	assert.DeepEqual(t, "hello", string(ret.Body()))
}
