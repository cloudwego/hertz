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

package binder

import (
	"errors"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestNewBinder(t *testing.T) {
	binder := NewBinder()
	assert.DeepEqual(t, "test binder", binder.Name())
	assert.Nil(t, binder.ValidateError)

	// Test all binding methods return nil
	assert.Nil(t, binder.Bind(nil, nil, nil))
	assert.Nil(t, binder.BindAndValidate(nil, nil, nil))
	assert.Nil(t, binder.BindQuery(nil, nil))
	assert.Nil(t, binder.BindHeader(nil, nil))
	assert.Nil(t, binder.BindPath(nil, nil, nil))
	assert.Nil(t, binder.BindForm(nil, nil))
	assert.Nil(t, binder.BindJSON(nil, nil))
	assert.Nil(t, binder.BindProtobuf(nil, nil))
	assert.Nil(t, binder.Validate(nil, nil))
}

func TestNewBinderWithValidateError(t *testing.T) {
	testErr := errors.New("test error")
	binder := NewBinderWithValidateError(testErr)
	assert.DeepEqual(t, testErr, binder.ValidateError)
}

func TestBinderValidate(t *testing.T) {
	// Test no error
	binder1 := NewBinder()
	assert.Nil(t, binder1.Validate(nil, nil))

	// Test with error
	testErr := errors.New("validation failed")
	binder2 := NewBinderWithValidateError(testErr)
	assert.DeepEqual(t, testErr, binder2.Validate(nil, nil))
}
