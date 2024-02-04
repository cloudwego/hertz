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
 * Modifications are Copyright 2023 CloudWeGo Authors
 */

package binding

import (
	"bytes"
	stdJson "encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strings"
	"sync"

	exprValidator "github.com/bytedance/go-tagexpr/v2/validator"
	"github.com/cloudwego/hertz/internal/bytesconv"
	inDecoder "github.com/cloudwego/hertz/pkg/app/server/binding/internal/decoder"
	hJson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route/param"
	"google.golang.org/protobuf/proto"
)

const (
	queryTag           = "query"
	headerTag          = "header"
	formTag            = "form"
	pathTag            = "path"
	defaultValidateTag = "vd"
)

type decoderInfo struct {
	decoder      inDecoder.Decoder
	needValidate bool
}

var defaultBind = NewDefaultBinder(nil)

func DefaultBinder() Binder {
	return defaultBind
}

type defaultBinder struct {
	config             *BindConfig
	decoderCache       sync.Map
	queryDecoderCache  sync.Map
	formDecoderCache   sync.Map
	headerDecoderCache sync.Map
	pathDecoderCache   sync.Map
}

func NewDefaultBinder(config *BindConfig) Binder {
	if config == nil {
		bindConfig := NewBindConfig()
		bindConfig.initTypeUnmarshal()
		return &defaultBinder{
			config: bindConfig,
		}
	}
	config.initTypeUnmarshal()
	if config.Validator == nil {
		config.Validator = DefaultValidator()
	}
	return &defaultBinder{
		config: config,
	}
}

// BindAndValidate binds data from *protocol.Request to obj and validates them if needed.
// NOTE:
//
//	obj should be a pointer.
func BindAndValidate(req *protocol.Request, obj interface{}, pathParams param.Params) error {
	return DefaultBinder().BindAndValidate(req, obj, pathParams)
}

// Bind binds data from *protocol.Request to obj.
// NOTE:
//
//	obj should be a pointer.
func Bind(req *protocol.Request, obj interface{}, pathParams param.Params) error {
	return DefaultBinder().Bind(req, obj, pathParams)
}

// Validate validates obj with "vd" tag
// NOTE:
//
//	obj should be a pointer.
//	Validate should be called after Bind.
func Validate(obj interface{}) error {
	return DefaultValidator().ValidateStruct(obj)
}

func (b *defaultBinder) tagCache(tag string) *sync.Map {
	switch tag {
	case queryTag:
		return &b.queryDecoderCache
	case headerTag:
		return &b.headerDecoderCache
	case formTag:
		return &b.formDecoderCache
	case pathTag:
		return &b.pathDecoderCache
	default:
		return &b.decoderCache
	}
}

func (b *defaultBinder) bindTag(req *protocol.Request, v interface{}, params param.Params, tag string) error {
	rv, typeID := valueAndTypeID(v)
	if err := checkPointer(rv); err != nil {
		return err
	}
	rt := dereferPointer(rv)
	if rt.Kind() != reflect.Struct {
		return b.bindNonStruct(req, v)
	}

	if len(tag) == 0 {
		err := b.preBindBody(req, v)
		if err != nil {
			return fmt.Errorf("bind body failed, err=%v", err)
		}
	}
	cache := b.tagCache(tag)
	cached, ok := cache.Load(typeID)
	if ok {
		// cached fieldDecoder, fast path
		decoder := cached.(decoderInfo)
		return decoder.decoder(req, params, rv.Elem())
	}
	validateTag := defaultValidateTag
	if len(b.config.Validator.ValidateTag()) != 0 {
		validateTag = b.config.Validator.ValidateTag()
	}
	decodeConfig := &inDecoder.DecodeConfig{
		LooseZeroMode:                      b.config.LooseZeroMode,
		DisableDefaultTag:                  b.config.DisableDefaultTag,
		DisableStructFieldResolve:          b.config.DisableStructFieldResolve,
		EnableDecoderUseNumber:             b.config.EnableDecoderUseNumber,
		EnableDecoderDisallowUnknownFields: b.config.EnableDecoderDisallowUnknownFields,
		ValidateTag:                        validateTag,
		TypeUnmarshalFuncs:                 b.config.TypeUnmarshalFuncs,
	}
	decoder, needValidate, err := inDecoder.GetReqDecoder(rv.Type(), tag, decodeConfig)
	if err != nil {
		return err
	}

	cache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	return decoder(req, params, rv.Elem())
}

func (b *defaultBinder) bindTagWithValidate(req *protocol.Request, v interface{}, params param.Params, tag string) error {
	rv, typeID := valueAndTypeID(v)
	if err := checkPointer(rv); err != nil {
		return err
	}
	rt := dereferPointer(rv)
	if rt.Kind() != reflect.Struct {
		return b.bindNonStruct(req, v)
	}

	err := b.preBindBody(req, v)
	if err != nil {
		return fmt.Errorf("bind body failed, err=%v", err)
	}
	cache := b.tagCache(tag)
	cached, ok := cache.Load(typeID)
	if ok {
		// cached fieldDecoder, fast path
		decoder := cached.(decoderInfo)
		err = decoder.decoder(req, params, rv.Elem())
		if err != nil {
			return err
		}
		if decoder.needValidate {
			err = b.config.Validator.ValidateStruct(rv.Elem())
		}
		return err
	}
	validateTag := defaultValidateTag
	if len(b.config.Validator.ValidateTag()) != 0 {
		validateTag = b.config.Validator.ValidateTag()
	}
	decodeConfig := &inDecoder.DecodeConfig{
		LooseZeroMode:                      b.config.LooseZeroMode,
		DisableDefaultTag:                  b.config.DisableDefaultTag,
		DisableStructFieldResolve:          b.config.DisableStructFieldResolve,
		EnableDecoderUseNumber:             b.config.EnableDecoderUseNumber,
		EnableDecoderDisallowUnknownFields: b.config.EnableDecoderDisallowUnknownFields,
		ValidateTag:                        validateTag,
		TypeUnmarshalFuncs:                 b.config.TypeUnmarshalFuncs,
	}
	decoder, needValidate, err := inDecoder.GetReqDecoder(rv.Type(), tag, decodeConfig)
	if err != nil {
		return err
	}

	cache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	err = decoder(req, params, rv.Elem())
	if err != nil {
		return err
	}
	if needValidate {
		err = b.config.Validator.ValidateStruct(rv.Elem())
	}
	return err
}

func (b *defaultBinder) BindQuery(req *protocol.Request, v interface{}) error {
	return b.bindTag(req, v, nil, queryTag)
}

func (b *defaultBinder) BindHeader(req *protocol.Request, v interface{}) error {
	return b.bindTag(req, v, nil, headerTag)
}

func (b *defaultBinder) BindPath(req *protocol.Request, v interface{}, params param.Params) error {
	return b.bindTag(req, v, params, pathTag)
}

func (b *defaultBinder) BindForm(req *protocol.Request, v interface{}) error {
	return b.bindTag(req, v, nil, formTag)
}

func (b *defaultBinder) BindJSON(req *protocol.Request, v interface{}) error {
	return b.decodeJSON(bytes.NewReader(req.Body()), v)
}

func (b *defaultBinder) decodeJSON(r io.Reader, obj interface{}) error {
	decoder := hJson.NewDecoder(r)
	if b.config.EnableDecoderUseNumber {
		decoder.UseNumber()
	}
	if b.config.EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	return decoder.Decode(obj)
}

func (b *defaultBinder) BindProtobuf(req *protocol.Request, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("%s does not implement 'proto.Message'", v)
	}
	return proto.Unmarshal(req.Body(), msg)
}

func (b *defaultBinder) Name() string {
	return "hertz"
}

func (b *defaultBinder) BindAndValidate(req *protocol.Request, v interface{}, params param.Params) error {
	return b.bindTagWithValidate(req, v, params, "")
}

func (b *defaultBinder) Bind(req *protocol.Request, v interface{}, params param.Params) error {
	return b.bindTag(req, v, params, "")
}

// best effort binding
func (b *defaultBinder) preBindBody(req *protocol.Request, v interface{}) error {
	if req.Header.ContentLength() <= 0 {
		return nil
	}
	ct := bytesconv.B2s(req.Header.ContentType())
	switch strings.ToLower(utils.FilterContentType(ct)) {
	case consts.MIMEApplicationJSON:
		return hJson.Unmarshal(req.Body(), v)
	case consts.MIMEPROTOBUF:
		msg, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("%s can not implement 'proto.Message'", v)
		}
		return proto.Unmarshal(req.Body(), msg)
	default:
		return nil
	}
}

func (b *defaultBinder) bindNonStruct(req *protocol.Request, v interface{}) (err error) {
	ct := bytesconv.B2s(req.Header.ContentType())
	switch strings.ToLower(utils.FilterContentType(ct)) {
	case consts.MIMEApplicationJSON:
		err = hJson.Unmarshal(req.Body(), v)
	case consts.MIMEPROTOBUF:
		msg, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("%s can not implement 'proto.Message'", v)
		}
		err = proto.Unmarshal(req.Body(), msg)
	case consts.MIMEMultipartPOSTForm:
		form := make(url.Values)
		mf, err1 := req.MultipartForm()
		if err1 == nil && mf.Value != nil {
			for k, v := range mf.Value {
				for _, vv := range v {
					form.Add(k, vv)
				}
			}
		}
		b, _ := stdJson.Marshal(form)
		err = hJson.Unmarshal(b, v)
	case consts.MIMEApplicationHTMLForm:
		form := make(url.Values)
		req.PostArgs().VisitAll(func(formKey, value []byte) {
			form.Add(string(formKey), string(value))
		})
		b, _ := stdJson.Marshal(form)
		err = hJson.Unmarshal(b, v)
	default:
		// using query to decode
		query := make(url.Values)
		req.URI().QueryArgs().VisitAll(func(queryKey, value []byte) {
			query.Add(string(queryKey), string(value))
		})
		b, _ := stdJson.Marshal(query)
		err = hJson.Unmarshal(b, v)
	}
	return
}

var _ StructValidator = (*validator)(nil)

type validator struct {
	validateTag string
	validate    *exprValidator.Validator
}

func NewValidator(config *ValidateConfig) StructValidator {
	validateTag := defaultValidateTag
	if config != nil && len(config.ValidateTag) != 0 {
		validateTag = config.ValidateTag
	}
	vd := exprValidator.New(validateTag).SetErrorFactory(defaultValidateErrorFactory)
	if config != nil && config.ErrFactory != nil {
		vd.SetErrorFactory(config.ErrFactory)
	}
	return &validator{
		validateTag: validateTag,
		validate:    vd,
	}
}

// Error validate error
type validateError struct {
	FailPath, Msg string
}

// Error implements error interface.
func (e *validateError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "invalid parameter: " + e.FailPath
}

func defaultValidateErrorFactory(failPath, msg string) error {
	return &validateError{
		FailPath: failPath,
		Msg:      msg,
	}
}

// ValidateStruct receives any kind of type, but only performed struct or pointer to struct type.
func (v *validator) ValidateStruct(obj interface{}) error {
	if obj == nil {
		return nil
	}
	return v.validate.Validate(obj)
}

// Engine returns the underlying validator
func (v *validator) Engine() interface{} {
	return v.validate
}

func (v *validator) ValidateTag() string {
	return v.validateTag
}

var defaultValidate = NewValidator(NewValidateConfig())

func DefaultValidator() StructValidator {
	return defaultValidate
}
