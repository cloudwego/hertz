// Copyright 2022 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//go:build !tinygo
// +build !tinygo

package app

import "github.com/cloudwego/hertz/pkg/app/server/binding"

// BindAndValidate binds data from *RequestContext to obj and validates them if needed.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) BindAndValidate(obj interface{}) error {
	return binding.BindAndValidate(&ctx.Request, obj, ctx.Params)
}

// Bind binds data from *RequestContext to obj.
// NOTE: obj should be a pointer.
func (ctx *RequestContext) Bind(obj interface{}) error {
	return binding.Bind(&ctx.Request, obj, ctx.Params)
}

// Validate validates obj with "vd" tag
// NOTE: obj should be a pointer.
func (ctx *RequestContext) Validate(obj interface{}) error {
	return binding.Validate(obj)
}
