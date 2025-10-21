/*
 * Copyright 2024 CloudWeGo Authors
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

package decoder

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"

	"github.com/cloudwego/hertz/internal/bytesconv"
	hJson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

const (
	specialChar = "\x07"
)

// toDefaultValue will preprocess the default value and transfer it to be standard format
func toDefaultValue(typ reflect.Type, defaultValue string) string {
	switch typ.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		// escape single quote and double quote, replace single quote with double quote
		defaultValue = strings.Replace(defaultValue, `"`, `\"`, -1)
		defaultValue = strings.Replace(defaultValue, `\'`, specialChar, -1)
		defaultValue = strings.Replace(defaultValue, `'`, `"`, -1)
		defaultValue = strings.Replace(defaultValue, specialChar, `'`, -1)
	}
	return defaultValue
}

// stringToValue is used to dynamically create reflect.Value for 'text'
func stringToValue(elemType reflect.Type, text string, req *protocol.Request, params param.Params, config *DecodeConfig) (reflect.Value, error) {
	if customizedFunc, exist := config.TypeUnmarshalFuncs[elemType]; exist {
		val, err := customizedFunc(req, params, text)
		if err != nil {
			return reflect.Value{}, err
		}
		return val, nil
	}
	v := reflect.New(elemType)
	if tryTextUnmarshaler(v, text) {
		return v.Elem(), nil
	}
	switch elemType.Kind() {
	case reflect.Struct, reflect.Map:
		if err := hJson.Unmarshal(bytesconv.S2b(text), v.Interface()); err != nil {
			return reflect.Value{}, err
		}
		return v.Elem(), nil

	case reflect.Array, reflect.Slice:
		// do nothing
		return v.Elem(), nil

	default:
		decoder, err := SelectTextDecoder(elemType)
		if err != nil {
			return reflect.Value{}, err
		}
		v = v.Elem()
		err = decoder.UnmarshalString(text, v, config.LooseZeroMode)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("unable to decode '%s' as %s: %w", text, elemType.String(), err)
		}
		return v, nil
	}
}

func tryTextUnmarshaler(v reflect.Value, s string) bool {
	enc, ok := v.Interface().(encoding.TextUnmarshaler)
	if ok {
		if err := enc.UnmarshalText(bytesconv.S2b(s)); err == nil {
			return true
		}
	}
	return false
}
