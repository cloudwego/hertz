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
	"io"
	"reflect"
	"sync"

	"github.com/bytedance/go-tagexpr/v2/validator"
	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/app/server/binding/decoder"
	hjson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
	"google.golang.org/protobuf/proto"
)

type decoderInfo struct {
	decoder      decoder.Decoder
	needValidate bool
}

type defaultBinder struct {
	decoderCache       sync.Map
	queryDecoderCache  sync.Map
	formDecoderCache   sync.Map
	headerDecoderCache sync.Map
	pathDecoderCache   sync.Map
}

func (b *defaultBinder) BindQuery(req *protocol.Request, v interface{}) error {
	rv, typeID := valueAndTypeID(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("receiver must be a non-nil pointer")
	}
	if rv.Elem().Kind() == reflect.Map {
		return nil
	}
	cached, ok := b.queryDecoderCache.Load(typeID)
	if ok {
		// cached fieldDecoder, fast path
		decoder := cached.(decoderInfo)
		return decoder.decoder(req, nil, rv.Elem())
	}

	decoder, needValidate, err := decoder.GetReqDecoder(rv.Type(), "query")
	if err != nil {
		return err
	}

	b.queryDecoderCache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	return decoder(req, nil, rv.Elem())
}

func (b *defaultBinder) BindHeader(req *protocol.Request, v interface{}) error {
	rv, typeID := valueAndTypeID(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("receiver must be a non-nil pointer")
	}
	if rv.Elem().Kind() == reflect.Map {
		return nil
	}
	cached, ok := b.headerDecoderCache.Load(typeID)
	if ok {
		// cached fieldDecoder, fast path
		decoder := cached.(decoderInfo)
		return decoder.decoder(req, nil, rv.Elem())
	}

	decoder, needValidate, err := decoder.GetReqDecoder(rv.Type(), "header")
	if err != nil {
		return err
	}

	b.headerDecoderCache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	return decoder(req, nil, rv.Elem())
}

func (b *defaultBinder) BindPath(req *protocol.Request, params param.Params, v interface{}) error {
	rv, typeID := valueAndTypeID(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("receiver must be a non-nil pointer")
	}
	if rv.Elem().Kind() == reflect.Map {
		return nil
	}
	cached, ok := b.pathDecoderCache.Load(typeID)
	if ok {
		// cached fieldDecoder, fast path
		decoder := cached.(decoderInfo)
		return decoder.decoder(req, params, rv.Elem())
	}

	decoder, needValidate, err := decoder.GetReqDecoder(rv.Type(), "path")
	if err != nil {
		return err
	}

	b.pathDecoderCache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	return decoder(req, params, rv.Elem())
}

func (b *defaultBinder) BindForm(req *protocol.Request, v interface{}) error {
	rv, typeID := valueAndTypeID(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("receiver must be a non-nil pointer")
	}
	if rv.Elem().Kind() == reflect.Map {
		return nil
	}
	cached, ok := b.formDecoderCache.Load(typeID)
	if ok {
		// cached fieldDecoder, fast path
		decoder := cached.(decoderInfo)
		return decoder.decoder(req, nil, rv.Elem())
	}

	decoder, needValidate, err := decoder.GetReqDecoder(rv.Type(), "form")
	if err != nil {
		return err
	}

	b.formDecoderCache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	return decoder(req, nil, rv.Elem())
}

func (b *defaultBinder) BindJSON(req *protocol.Request, v interface{}) error {
	return decodeJSON(req.BodyStream(), v)
}

func decodeJSON(r io.Reader, obj interface{}) error {
	decoder := hjson.NewDecoder(r)
	if enableDecoderUseNumber {
		decoder.UseNumber()
	}
	if enableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	return decoder.Decode(obj)
}

func (b *defaultBinder) BindProtobuf(req *protocol.Request, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("%s can not implement 'proto.Message'", v)
	}
	return proto.Unmarshal(req.Body(), msg)
}

func (b *defaultBinder) Name() string {
	return "hertz"
}

func (b *defaultBinder) BindAndValidate(req *protocol.Request, params param.Params, v interface{}) error {
	err := b.preBindBody(req, v)
	if err != nil {
		return fmt.Errorf("bind body failed, err=%v", err)
	}
	rv, typeID := valueAndTypeID(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("receiver must be a non-nil pointer")
	}
	if rv.Elem().Kind() == reflect.Map {
		return nil
	}
	cached, ok := b.decoderCache.Load(typeID)
	if ok {
		// cached fieldDecoder, fast path
		decoder := cached.(decoderInfo)
		err = decoder.decoder(req, params, rv.Elem())
		if err != nil {
			return err
		}
		if decoder.needValidate {
			err = DefaultValidator().ValidateStruct(rv.Elem())
		}
		return err
	}

	decoder, needValidate, err := decoder.GetReqDecoder(rv.Type(), "")
	if err != nil {
		return err
	}

	b.decoderCache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	err = decoder(req, params, rv.Elem())
	if err != nil {
		return err
	}
	if needValidate {
		err = DefaultValidator().ValidateStruct(rv.Elem())
	}
	return err
}

func (b *defaultBinder) Bind(req *protocol.Request, params param.Params, v interface{}) error {
	err := b.preBindBody(req, v)
	if err != nil {
		return fmt.Errorf("bind body failed, err=%v", err)
	}
	rv, typeID := valueAndTypeID(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("receiver must be a non-nil pointer")
	}
	if rv.Elem().Kind() == reflect.Map {
		return nil
	}
	cached, ok := b.decoderCache.Load(typeID)
	if ok {
		// cached fieldDecoder, fast path
		decoder := cached.(decoderInfo)
		return decoder.decoder(req, params, rv.Elem())
	}

	decoder, needValidate, err := decoder.GetReqDecoder(rv.Type(), "")
	if err != nil {
		return err
	}

	b.decoderCache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	return decoder(req, params, rv.Elem())
}

var (
	jsonContentType     = "application/json"
	protobufContentType = "application/x-protobuf"
)

// best effort binding
func (b *defaultBinder) preBindBody(req *protocol.Request, v interface{}) error {
	if req.Header.ContentLength() <= 0 {
		return nil
	}
	ct := bytesconv.B2s(req.Header.ContentType())
	switch utils.FilterContentType(ct) {
	case jsonContentType:
		return hjson.Unmarshal(req.Body(), v)
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
	validate *validator.Validator
}

// ValidateStruct receives any kind of type, but only performed struct or pointer to struct type.
func (v *defaultValidator) ValidateStruct(obj interface{}) error {
	if obj == nil {
		return nil
	}
	v.lazyinit()
	return v.validate.Validate(obj)
}

func (v *defaultValidator) lazyinit() {
	v.once.Do(func() {
		v.validate = validator.Default()
	})
}

// Engine returns the underlying validator
func (v *defaultValidator) Engine() interface{} {
	v.lazyinit()
	return v.validate
}
