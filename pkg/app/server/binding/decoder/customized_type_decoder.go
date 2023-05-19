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
 * MIT License
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
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package decoder

import (
	"fmt"
	"reflect"
	"time"

	path1 "github.com/cloudwego/hertz/pkg/app/server/binding/path"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func init() {
	MustRegTypeUnmarshal(reflect.TypeOf(time.Time{}), func(req *protocol.Request, params path1.PathParam, text string) (reflect.Value, error) {
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

type customizeDecodeFunc func(req *protocol.Request, params path1.PathParam, text string) (reflect.Value, error)

var typeUnmarshalFuncs = make(map[reflect.Type]customizeDecodeFunc)

// RegTypeUnmarshal registers customized type unmarshaler.
func RegTypeUnmarshal(t reflect.Type, fn customizeDecodeFunc) error {
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
	// test
	//vv, err := fn(&protocol.Request{}, nil)
	//if err != nil {
	//	return fmt.Errorf("test fail: %s", err)
	//}
	//if tt := vv.Type(); tt != t {
	//	return fmt.Errorf("test fail: expect return value type is %s, but got %s", t.String(), tt.String())
	//}

	typeUnmarshalFuncs[t] = fn
	return nil
}

// MustRegTypeUnmarshal registers customized type unmarshaler. It will panic if exist error.
func MustRegTypeUnmarshal(t reflect.Type, fn customizeDecodeFunc) {
	err := RegTypeUnmarshal(t, fn)
	if err != nil {
		panic(err)
	}
}

type customizedFieldTextDecoder struct {
	fieldInfo
	decodeFunc customizeDecodeFunc
}

func (d *customizedFieldTextDecoder) Decode(req *bindRequest, params path1.PathParam, reqValue reflect.Value) error {
	var text string
	var defaultValue string
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Skip || tagInfo.Key == jsonTag || tagInfo.Key == fileNameTag {
			defaultValue = tagInfo.Default
			continue
		}
		if tagInfo.Key == headerTag {
			tagInfo.Value = utils.GetNormalizeHeaderKey(tagInfo.Value, req.Req.Header.IsDisableNormalizing())
		}
		ret := tagInfo.Getter(req, params, tagInfo.Value)
		defaultValue = tagInfo.Default
		if len(ret) != 0 {
			text = ret[0]
			break
		}
	}
	if len(text) == 0 && len(defaultValue) != 0 {
		text = defaultValue
	}

	v, err := d.decodeFunc(req.Req, params, text)
	if err != nil {
		return err
	}

	reqValue = GetFieldValue(reqValue, d.parentIndex)
	field := reqValue.Field(d.index)
	if field.Kind() == reflect.Ptr {
		t := field.Type()
		var ptrDepth int
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
			ptrDepth++
		}
		field.Set(ReferenceValue(v, ptrDepth))
		return nil
	}

	field.Set(v)
	return nil
}

func getCustomizedFieldDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int, decodeFunc customizeDecodeFunc) ([]fieldDecoder, error) {
	for idx, tagInfo := range tagInfos {
		switch tagInfo.Key {
		case pathTag:
			tagInfos[idx].Getter = path
		case formTag:
			tagInfos[idx].Getter = postForm
		case queryTag:
			tagInfos[idx].Getter = query
		case cookieTag:
			tagInfos[idx].Getter = cookie
		case headerTag:
			tagInfos[idx].Getter = header
		case jsonTag:
			// do nothing
		case rawBodyTag:
			tagInfos[idx].Getter = rawBody
		case fileNameTag:
			// do nothing
		default:
		}
	}
	fieldType := field.Type
	for field.Type.Kind() == reflect.Ptr {
		fieldType = field.Type.Elem()
	}
	return []fieldDecoder{&customizedFieldTextDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
		},
		decodeFunc: decodeFunc,
	}}, nil
}
