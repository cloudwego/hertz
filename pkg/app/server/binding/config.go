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

package binding

import (
	standardJson "encoding/json"
	"reflect"

	"github.com/bytedance/go-tagexpr/v2/validator"
	"github.com/cloudwego/hertz/pkg/app/server/binding/internal/decoder"
	hjson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

// UseThirdPartyJSONUnmarshaler uses third-party json library for binding
// NOTE:
//
//	UseThirdPartyJSONUnmarshaler will remain in effect once it has been called.
func UseThirdPartyJSONUnmarshaler(fn func(data []byte, v interface{}) error) {
	hjson.Unmarshal = fn
}

// UseStdJSONUnmarshaler uses encoding/json as json library
// NOTE:
//
//	The current version uses encoding/json by default.
//	UseStdJSONUnmarshaler will remain in effect once it has been called.
func UseStdJSONUnmarshaler() {
	UseThirdPartyJSONUnmarshaler(standardJson.Unmarshal)
}

// EnableDefaultTag is used to enable or disable adding default tags to a field when it has no tag, it is true by default.
// If is true, the field with no tag will be added default tags, for more automated parameter binding. But there may be additional overhead
func EnableDefaultTag(b bool) {
	decoder.EnableDefaultTag = b
}

// EnableStructFieldResolve to enable or disable the generation of a separate decoder for a struct, it is false by default.
// If is true, the 'struct' field will get a single decoder.structTypeFieldTextDecoder, and use json.Unmarshal for decode it.
func EnableStructFieldResolve(b bool) {
	decoder.EnableStructFieldResolve = b
}

// RegTypeUnmarshal registers customized type unmarshaler.
func RegTypeUnmarshal(t reflect.Type, fn func(req *protocol.Request, params param.Params, text string) (reflect.Value, error)) error {
	return decoder.RegTypeUnmarshal(t, fn)
}

// MustRegTypeUnmarshal registers customized type unmarshaler. It will panic if exist error.
func MustRegTypeUnmarshal(t reflect.Type, fn func(req *protocol.Request, params param.Params, text string) (reflect.Value, error)) {
	decoder.MustRegTypeUnmarshal(t, fn)
}

// ResetValidator reset a customized
func ResetValidator(v StructValidator, validatorTag string) {
	defaultValidate = v
	decoder.DefaultValidatorTag = validatorTag
}

// MustRegValidateFunc registers validator function expression.
// NOTE:
//
//	If force=true, allow to cover the existed same funcName.
//	MustRegValidateFunc will remain in effect once it has been called.
func MustRegValidateFunc(funcName string, fn func(args ...interface{}) error, force ...bool) {
	validator.MustRegFunc(funcName, fn, force...)
}

// SetValidatorErrorFactory customizes the factory of validation error.
func SetValidatorErrorFactory(validatingErrFactory func(failField, msg string) error) {
	if val, ok := DefaultValidator().(*defaultValidator); ok {
		val.validate.SetErrorFactory(validatingErrFactory)
	} else {
		panic("customized validator can not use 'SetValidatorErrorFactory'")
	}
}

var enableDecoderUseNumber = false

var enableDecoderDisallowUnknownFields = false

// EnableDecoderUseNumber is used to call the UseNumber method on the JSON
// Decoder instance. UseNumber causes the Decoder to unmarshal a number into an
// interface{} as a Number instead of as a float64.
// NOTE: it is used for BindJSON().
func EnableDecoderUseNumber(b bool) {
	enableDecoderUseNumber = b
}

// EnableDecoderDisallowUnknownFields is used to call the DisallowUnknownFields method
// on the JSON Decoder instance. DisallowUnknownFields causes the Decoder to
// return an error when the destination is a struct and the input contains object
// keys which do not match any non-ignored, exported fields in the destination.
// NOTE: it is used for BindJSON().
func EnableDecoderDisallowUnknownFields(b bool) {
	enableDecoderDisallowUnknownFields = b
}
