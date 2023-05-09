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
 * The MIT License
 *
 * Copyright (c) 2019-present Fenny and Contributors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
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
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package binding

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/go-playground/validator/v10"
	"google.golang.org/protobuf/proto"
)

type defaultBinder struct {
	decoderCache sync.Map
}

func (b *defaultBinder) Name() string {
	return "hertz"
}

func (b *defaultBinder) Bind(req *protocol.Request, params PathParam, v interface{}) error {
	err := b.preBindBody(req, v)
	if err != nil {
		return err
	}
	rv, typeID := valueAndTypeID(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("receiver must be a non-nil pointer")
	}
	if rv.Elem().Kind() == reflect.Map {
		return nil
	}
	cached, ok := b.decoderCache.Load(typeID)
	if ok {
		// cached decoder, fast path
		decoder := cached.(Decoder)
		return decoder(req, params, rv.Elem())
	}

	decoder, err := getReqDecoder(rv.Type())
	if err != nil {
		return err
	}

	b.decoderCache.Store(typeID, decoder)
	return decoder(req, params, rv.Elem())
}

var (
	jsonContentTypeBytes = "application/json; charset=utf-8"
	protobufContentType  = "application/x-protobuf"
)

// best effort binding
func (b *defaultBinder) preBindBody(req *protocol.Request, v interface{}) error {
	if req.Header.ContentLength() <= 0 {
		return nil
	}
	switch bytesconv.B2s(req.Header.ContentType()) {
	case jsonContentTypeBytes:
		// todo: Aligning the gin, add "EnableDecoderUseNumber"/"EnableDecoderDisallowUnknownFields" interface
		return jsonUnmarshalFunc(req.Body(), v)
	case protobufContentType:
		msg, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("%s can not implement 'proto.Message'", v)
		}
		return proto.Unmarshal(req.Body(), msg)
	default:
		return nil
	}
}

var _ StructValidator = (*defaultValidator)(nil)

type defaultValidator struct {
	once     sync.Once
	validate *validator.Validate
}

// ValidateStruct receives any kind of type, but only performed struct or pointer to struct type.
func (v *defaultValidator) ValidateStruct(obj interface{}) error {
	if obj == nil {
		return nil
	}

	value := reflect.ValueOf(obj)
	switch value.Kind() {
	case reflect.Ptr:
		return v.ValidateStruct(value.Elem().Interface())
	case reflect.Struct:
		return v.validateStruct(obj)
	default:
		return nil
	}
}

// validateStruct receives struct type
func (v *defaultValidator) validateStruct(obj interface{}) error {
	v.lazyinit()
	return v.validate.Struct(obj)
}

func (v *defaultValidator) lazyinit() {
	v.once.Do(func() {
		v.validate = validator.New()
		v.validate.SetTagName("validate")
	})
}

// Engine returns the underlying validator engine which powers the default
// Validator instance. This is useful if you want to register custom validations
// or struct level validations. See validator GoDoc for more info -
// https://pkg.go.dev/github.com/go-playground/validator/v10
func (v *defaultValidator) Engine() interface{} {
	v.lazyinit()
	return v.validate
}
