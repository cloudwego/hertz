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
 * The MIT License (MIT)
 *
 * Copyright (c) 2014 Manuel Martínez-Almeida
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2023 CloudWeGo Authors
 */

package binding

import (
	"reflect"
	"sync"

	"github.com/cloudwego/hertz/pkg/protocol"
)

// ValidatorFunc defines a validation function that can access request context.
// It takes a request and the object to validate, returning an error if validation fails.
type ValidatorFunc func(*protocol.Request, interface{}) error

// StructValidator defines the interface for struct validation.
//
// Deprecated: Use ValidatorFunc in BindConfig instead. You can create a ValidatorFunc
// from a StructValidator using MakeValidatorFunc().
type StructValidator interface {
	ValidateStruct(interface{}) error
	Engine() interface{}
	ValidateTag() string
}

// hasValidateTagCache caches whether a type has validation tags to avoid
// redundant reflection-based tag analysis on repeated validations
var hasValidateTagCache sync.Map

// MakeValidatorFunc creates a validation function from a StructValidator.
// It optimizes validation by caching tag analysis results and skipping
// validation entirely for types that don't have validation tags.
func MakeValidatorFunc(s StructValidator) ValidatorFunc {
	if s == nil {
		return nil
	}
	return func(_ *protocol.Request, v any) error {
		rv, typeID := valueAndTypeID(v)
		c, ok := hasValidateTagCache.Load(typeID)
		if ok {
			if !c.(bool) {
				return nil
			}
			return s.ValidateStruct(rv)
		}
		tag := s.ValidateTag()
		if tag == "" {
			tag = defaultValidateTag
		}
		hasTag := containsStructTag(rv.Type(), tag, nil)
		hasValidateTagCache.Store(typeID, hasTag)
		if !hasTag {
			return nil
		}
		return s.ValidateStruct(rv)
	}
}

// containsStructTag recursively checks if a struct type contains any field with the specified tag.
// It uses a checking map to prevent infinite recursion in self-referential struct types.
func containsStructTag(rt reflect.Type, tag string, checking map[reflect.Type]bool) bool {
	rt = dereferenceType(rt)
	if rt.Kind() != reflect.Struct {
		return false
	}
	if checking == nil {
		checking = map[reflect.Type]bool{}
	}
	checking[rt] = true
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		_, ok := f.Tag.Lookup(tag)
		if ok {
			return true
		}
		ft := dereferenceType(f.Type)
		if checking[ft] {
			continue
		}
		if containsStructTag(ft, tag, checking) {
			return true
		}
	}
	return false
}
