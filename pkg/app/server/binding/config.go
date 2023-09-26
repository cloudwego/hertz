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
	stdJson "encoding/json"
	"fmt"
	"reflect"
	"time"

	exprValidator "github.com/bytedance/go-tagexpr/v2/validator"
	inDecoder "github.com/cloudwego/hertz/pkg/app/server/binding/internal/decoder"
	hJson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

// BindConfig contains options for default bind behavior.
type BindConfig struct {
	// LooseZeroMode if set to true,
	// the empty string request parameter is bound to the zero value of parameter.
	// NOTE:
	//	The default is false.
	//	Suitable for these parameter types: query/header/cookie/form .
	LooseZeroMode bool
	// DisableDefaultTag is used to add default tags to a field when it has no tag
	// If is false, the field with no tag will be added default tags, for more automated binding. But there may be additional overhead.
	// NOTE:
	// The default is false.
	DisableDefaultTag bool
	// DisableStructFieldResolve is used to generate a separate decoder for a struct.
	// If is false, the 'struct' field will get a single inDecoder.structTypeFieldTextDecoder, and use json.Unmarshal for decode it.
	// It usually used to add json string to query parameter.
	// NOTE:
	// The default is false.
	DisableStructFieldResolve bool
	// EnableDecoderUseNumber is used to call the UseNumber method on the JSON
	// Decoder instance. UseNumber causes the Decoder to unmarshal a number into an
	// interface{} as a Number instead of as a float64.
	// NOTE:
	// The default is false.
	// It is used for BindJSON().
	EnableDecoderUseNumber bool
	// EnableDecoderDisallowUnknownFields is used to call the DisallowUnknownFields method
	// on the JSON Decoder instance. DisallowUnknownFields causes the Decoder to
	// return an error when the destination is a struct and the input contains object
	// keys which do not match any non-ignored, exported fields in the destination.
	// NOTE:
	// The default is false.
	// It is used for BindJSON().
	EnableDecoderDisallowUnknownFields bool
	// TypeUnmarshalFuncs registers customized type unmarshaler.
	// NOTE:
	// time.Time is registered by default
	TypeUnmarshalFuncs map[reflect.Type]inDecoder.CustomizeDecodeFunc
	// Validator is used to validate for BindAndValidate()
	Validator StructValidator
}

func NewBindConfig() *BindConfig {
	return &BindConfig{
		LooseZeroMode:                      false,
		DisableDefaultTag:                  false,
		DisableStructFieldResolve:          false,
		EnableDecoderUseNumber:             false,
		EnableDecoderDisallowUnknownFields: false,
		TypeUnmarshalFuncs:                 make(map[reflect.Type]inDecoder.CustomizeDecodeFunc),
		Validator:                          defaultValidate,
	}
}

// RegTypeUnmarshal registers customized type unmarshaler.
func (config *BindConfig) RegTypeUnmarshal(t reflect.Type, fn inDecoder.CustomizeDecodeFunc) error {
	// check
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8,
		reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		return fmt.Errorf("registration type cannot be a basic type")
	case reflect.Ptr:
		return fmt.Errorf("registration type cannot be a pointer type")
	}
	if config.TypeUnmarshalFuncs == nil {
		config.TypeUnmarshalFuncs = make(map[reflect.Type]inDecoder.CustomizeDecodeFunc)
	}
	config.TypeUnmarshalFuncs[t] = fn
	return nil
}

// MustRegTypeUnmarshal registers customized type unmarshaler. It will panic if exist error.
func (config *BindConfig) MustRegTypeUnmarshal(t reflect.Type, fn func(req *protocol.Request, params param.Params, text string) (reflect.Value, error)) {
	err := config.RegTypeUnmarshal(t, fn)
	if err != nil {
		panic(err)
	}
}

func (config *BindConfig) initTypeUnmarshal() {
	config.MustRegTypeUnmarshal(reflect.TypeOf(time.Time{}), func(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
		if text == "" {
			return reflect.ValueOf(time.Time{}), nil
		}
		t, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(t), nil
	})
}

// UseThirdPartyJSONUnmarshaler uses third-party json library for binding
// NOTE:
//
//	UseThirdPartyJSONUnmarshaler will remain in effect once it has been called.
func (config *BindConfig) UseThirdPartyJSONUnmarshaler(fn func(data []byte, v interface{}) error) {
	hJson.Unmarshal = fn
}

// UseStdJSONUnmarshaler uses encoding/json as json library
// NOTE:
//
//	The current version uses encoding/json by default.
//	UseStdJSONUnmarshaler will remain in effect once it has been called.
func (config *BindConfig) UseStdJSONUnmarshaler() {
	config.UseThirdPartyJSONUnmarshaler(stdJson.Unmarshal)
}

type ValidateErrFactory func(fieldSelector, msg string) error

type ValidateConfig struct {
	ValidateTag string
	ErrFactory  ValidateErrFactory
}

func NewValidateConfig() *ValidateConfig {
	return &ValidateConfig{}
}

// MustRegValidateFunc registers validator function expression.
// NOTE:
//
//	If force=true, allow to cover the existed same funcName.
//	MustRegValidateFunc will remain in effect once it has been called.
func (config *ValidateConfig) MustRegValidateFunc(funcName string, fn func(args ...interface{}) error, force ...bool) {
	exprValidator.MustRegFunc(funcName, fn, force...)
}

// SetValidatorErrorFactory customizes the factory of validation error.
func (config *ValidateConfig) SetValidatorErrorFactory(errFactory ValidateErrFactory) {
	config.ErrFactory = errFactory
}

// SetValidatorTag customizes the factory of validation error.
func (config *ValidateConfig) SetValidatorTag(tag string) {
	config.ValidateTag = tag
}
