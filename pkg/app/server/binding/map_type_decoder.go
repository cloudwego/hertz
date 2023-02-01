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

package binding

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
)

type mapTypeFieldTextDecoder struct {
	fieldInfo
}

func (d *mapTypeFieldTextDecoder) Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error {
	var text string
	var defaultValue string
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Key == jsonTag {
			continue
		}
		if tagInfo.Key == headerTag {
			tagInfo.Value = utils.GetNormalizeHeaderKey(tagInfo.Value, req.Header.IsDisableNormalizing())
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
	if text == "" {
		return nil
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
		var vv reflect.Value
		vv, err := stringToValue(t, text)
		if err != nil {
			return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.fieldType.Name(), err)
		}
		field.Set(ReferenceValue(vv, ptrDepth))
		return nil
	}

	err := jsonUnmarshalFunc(bytesconv.S2b(text), field.Addr().Interface())
	if err != nil {
		return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.fieldType.Name(), err)
	}

	return nil
}

func getMapTypeTextDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int) ([]decoder, error) {
	for idx, tagInfo := range tagInfos {
		switch tagInfo.Key {
		case pathTag:
			tagInfos[idx].Getter = PathParam
		case formTag:
			tagInfos[idx].Getter = Form
		case queryTag:
			tagInfos[idx].Getter = Query
		case cookieTag:
			tagInfos[idx].Getter = Cookie
		case headerTag:
			tagInfos[idx].Getter = Header
		case jsonTag:
			// do nothing
		case rawBodyTag:
			tagInfo.Getter = RawBody
		default:
		}
	}

	fieldType := field.Type
	for field.Type.Kind() == reflect.Ptr {
		fieldType = field.Type.Elem()
	}
	fieldDecoder := &mapTypeFieldTextDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
		},
	}

	return []decoder{fieldDecoder}, nil
}
