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
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

// Binder provides a mock implementation of the Binder interface for testing.
type Binder struct {
	ValidateError error // Error to return from Validate method
}

// NewBinder creates a new mock binder.
func NewBinder() *Binder {
	return &Binder{}
}

// NewBinderWithValidateError creates a new mock binder that returns the specified error from Validate.
func NewBinderWithValidateError(err error) *Binder {
	return &Binder{ValidateError: err}
}

func (m *Binder) Name() string {
	return "test binder"
}

func (m *Binder) Bind(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *Binder) BindAndValidate(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *Binder) BindQuery(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *Binder) BindHeader(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *Binder) BindPath(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *Binder) BindForm(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *Binder) BindJSON(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *Binder) BindProtobuf(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *Binder) Validate(request *protocol.Request, i interface{}) error {
	if m.ValidateError != nil {
		return m.ValidateError
	}
	return nil
}
