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
 * Modifications are Copyright 2023 CloudWeGo Authors
 */

package decoder

import (
	"fmt"
	"mime/multipart"
	"reflect"

	"github.com/cloudwego/hertz/internal/bytesconv"
	hJson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type sliceTypeFieldTextDecoder struct {
	fieldInfo
	isArray bool
}

func (d *sliceTypeFieldTextDecoder) Decode(req *protocol.Request, params param.Params, reqValue reflect.Value) error {
	var err error
	var texts []string
	var defaultValue string
	var bindRawBody bool
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Skip || tagInfo.Key == jsonTag || tagInfo.Key == fileNameTag {
			defaultValue = tagInfo.Default
			if tagInfo.Key == jsonTag {
				found := checkRequireJSON(req, tagInfo)
				if found {
					err = nil
				} else {
					err = fmt.Errorf("'%s' field is a 'required' parameter, but the request does not have this parameter", d.fieldName)
				}
			}
			continue
		}
		if tagInfo.Key == rawBodyTag {
			bindRawBody = true
		}
		texts = tagInfo.SliceGetter(req, params, tagInfo.Value)
		defaultValue = tagInfo.Default
		if len(texts) != 0 {
			err = nil
			break
		}
		if tagInfo.Required {
			err = fmt.Errorf("'%s' field is a 'required' parameter, but the request does not have this parameter", d.fieldName)
		}
	}
	if err != nil {
		return err
	}
	if len(texts) == 0 && len(defaultValue) != 0 {
		texts = append(texts, defaultValue)
	}
	if len(texts) == 0 {
		return nil
	}

	reqValue = GetFieldValue(reqValue, d.parentIndex)
	field := reqValue.Field(d.index)
	// **[]**int
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			nonNilVal, ptrDepth := GetNonNilReferenceValue(field)
			field.Set(ReferenceValue(nonNilVal, ptrDepth))
		}
	}
	var parentPtrDepth int
	for field.Kind() == reflect.Ptr {
		field = field.Elem()
		parentPtrDepth++
	}

	if d.isArray {
		if len(texts) != field.Len() {
			return fmt.Errorf("%q is not valid value for %s", texts, field.Type().String())
		}
	} else {
		// slice need creating enough capacity
		field = reflect.MakeSlice(field.Type(), len(texts), len(texts))
	}
	// raw_body && []byte binding
	if bindRawBody && field.Type().Elem().Kind() == reflect.Uint8 {
		reqValue.Field(d.index).Set(reflect.ValueOf(req.Body()))
		return nil
	}

	// handle internal multiple pointer, []**int
	var ptrDepth int
	t := d.fieldType.Elem() // d.fieldType is non-pointer type for the field
	elemKind := t.Kind()
	for elemKind == reflect.Ptr {
		t = t.Elem()
		elemKind = t.Kind()
		ptrDepth++
	}

	for idx, text := range texts {
		var vv reflect.Value
		vv, err = stringToValue(t, text, req, params, d.config)
		if err != nil {
			break
		}
		field.Index(idx).Set(ReferenceValue(vv, ptrDepth))
	}
	if err != nil {
		if !reqValue.Field(d.index).CanAddr() {
			return err
		}
		// text[0] can be a complete json content for []Type.
		err = hJson.Unmarshal(bytesconv.S2b(texts[0]), reqValue.Field(d.index).Addr().Interface())
		if err != nil {
			return fmt.Errorf("using '%s' to unmarshal field '%s: %s' failed, %v", texts[0], d.fieldName, d.fieldType.String(), err)
		}
	} else {
		reqValue.Field(d.index).Set(ReferenceValue(field, parentPtrDepth))
	}

	return nil
}

func getSliceFieldDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int, config *DecodeConfig) ([]fieldDecoder, error) {
	if !(field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) {
		return nil, fmt.Errorf("unexpected type %s, expected slice or array", field.Type.String())
	}
	isArray := false
	if field.Type.Kind() == reflect.Array {
		isArray = true
	}
	for idx, tagInfo := range tagInfos {
		switch tagInfo.Key {
		case pathTag:
			tagInfos[idx].SliceGetter = pathSlice
			tagInfos[idx].Getter = path
		case formTag:
			tagInfos[idx].SliceGetter = postFormSlice
			tagInfos[idx].Getter = postForm
		case queryTag:
			tagInfos[idx].SliceGetter = querySlice
			tagInfos[idx].Getter = query
		case cookieTag:
			tagInfos[idx].SliceGetter = cookieSlice
			tagInfos[idx].Getter = cookie
		case headerTag:
			tagInfos[idx].SliceGetter = headerSlice
			tagInfos[idx].Getter = header
		case jsonTag:
			// do nothing
		case rawBodyTag:
			tagInfos[idx].SliceGetter = rawBodySlice
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
	// fieldType.Elem() is the type for array/slice elem
	t := getElemType(fieldType.Elem())
	if t == reflect.TypeOf(multipart.FileHeader{}) {
		return getMultipartFileDecoder(field, index, tagInfos, parentIdx, config)
	}

	return []fieldDecoder{&sliceTypeFieldTextDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
			config:      config,
		},
		isArray: isArray,
	}}, nil
}

func stringToValue(elemType reflect.Type, text string, req *protocol.Request, params param.Params, config *DecodeConfig) (v reflect.Value, err error) {
	v = reflect.New(elemType).Elem()
	if customizedFunc, exist := config.TypeUnmarshalFuncs[elemType]; exist {
		val, err := customizedFunc(req, params, text)
		if err != nil {
			return reflect.Value{}, err
		}
		return val, nil
	}
	switch elemType.Kind() {
	case reflect.Struct:
		err = hJson.Unmarshal(bytesconv.S2b(text), v.Addr().Interface())
	case reflect.Map:
		err = hJson.Unmarshal(bytesconv.S2b(text), v.Addr().Interface())
	case reflect.Array, reflect.Slice:
		// do nothing
	default:
		decoder, err := SelectTextDecoder(elemType)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("unsupported type %s for slice/array", elemType.String())
		}
		err = decoder.UnmarshalString(text, v, config.LooseZeroMode)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("unable to decode '%s' as %s: %w", text, elemType.String(), err)
		}
	}

	return v, err
}
