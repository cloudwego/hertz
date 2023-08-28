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

package binding

import (
	"fmt"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"testing"
)

func Test_ValidateStruct(t *testing.T) {
	type User struct {
		Age int `vd:"$>=0&&$<=130"`
	}

	user := &User{
		Age: 135,
	}
	err := DefaultValidator().ValidateStruct(user)
	if err == nil {
		t.Fatalf("expected an error, but got nil")
	}
}

type mockValidator struct{}

func (m *mockValidator) ValidateStruct(interface{}) error {
	return fmt.Errorf("test mock")

}

func (m *mockValidator) Engine() interface{} {
	return nil
}

func Test_ResetValidatorConfig(t *testing.T) {
	m := &mockValidator{}
	ResetValidator(m, "vt")
	type User struct {
		Age int `vt:"$>=0&&$<=130"`
	}

	user := &User{
		Age: 135,
	}
	err := DefaultValidator().ValidateStruct(user)
	if err == nil {
		t.Fatalf("expected an error, but got nil")
	}
	assert.DeepEqual(t, "test mock", err.Error())
}
