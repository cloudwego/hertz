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
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"

	path1 "github.com/cloudwego/hertz/pkg/app/server/binding/path"
	"github.com/cloudwego/hertz/pkg/protocol"
)

var (
	EnableDefaultTag         = true
	EnableStructFieldResolve = false
)

type bindRequest struct {
	Req           *protocol.Request
	Query         url.Values
	Form          url.Values
	MultipartForm url.Values
	Header        http.Header
	Cookie        []*http.Cookie
}

type fieldDecoder interface {
	Decode(req *bindRequest, params path1.PathParam, reqValue reflect.Value) error
}

type Decoder func(req *protocol.Request, params path1.PathParam, rv reflect.Value) error

func GetReqDecoder(rt reflect.Type) (Decoder, error) {
	var decoders []fieldDecoder

	el := rt.Elem()
	if el.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported \"%s\" type binding", el.String())
	}

	for i := 0; i < el.NumField(); i++ {
		if el.Field(i).PkgPath != "" && !el.Field(i).Anonymous {
			// ignore unexported field
			continue
		}

		dec, err := getFieldDecoder(el.Field(i), i, []int{}, "")
		if err != nil {
			return nil, err
		}

		if dec != nil {
			decoders = append(decoders, dec...)
		}
	}

	return func(req *protocol.Request, params path1.PathParam, rv reflect.Value) error {
		bindReq := &bindRequest{
			Req: req,
		}
		for _, decoder := range decoders {
			err := decoder.Decode(bindReq, params, rv)
			if err != nil {
				return err
			}
		}

		return nil
	}, nil
}

func getFieldDecoder(field reflect.StructField, index int, parentIdx []int, parentJSONName string) ([]fieldDecoder, error) {
	for field.Type.Kind() == reflect.Ptr {
		field.Type = field.Type.Elem()
	}
	if field.Type.Kind() != reflect.Struct && field.Anonymous {
		return nil, nil
	}

	fieldTagInfos, newParentJSONName := lookupFieldTags(field, parentJSONName)
	if len(fieldTagInfos) == 0 && EnableDefaultTag {
		fieldTagInfos = getDefaultFieldTags(field)
	}

	// customized type decoder has the highest priority
	if customizedFunc, exist := typeUnmarshalFuncs[field.Type]; exist {
		return getCustomizedFieldDecoder(field, index, fieldTagInfos, parentIdx, customizedFunc)
	}

	if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
		return getSliceFieldDecoder(field, index, fieldTagInfos, parentIdx)
	}

	if field.Type.Kind() == reflect.Map {
		return getMapTypeTextDecoder(field, index, fieldTagInfos, parentIdx)
	}

	if field.Type.Kind() == reflect.Struct {
		var decoders []fieldDecoder
		el := field.Type
		// todo: more built-in common struct binding, ex. time...
		switch el {
		case reflect.TypeOf(multipart.FileHeader{}):
			return getMultipartFileDecoder(field, index, fieldTagInfos, parentIdx)
		}
		if EnableStructFieldResolve {
			structFieldDecoder, err := getStructTypeFieldDecoder(field, index, fieldTagInfos, parentIdx)
			if err != nil {
				return nil, err
			}
			if structFieldDecoder != nil {
				decoders = append(decoders, structFieldDecoder...)
			}
		}

		for i := 0; i < el.NumField(); i++ {
			if el.Field(i).PkgPath != "" && !el.Field(i).Anonymous {
				// ignore unexported field
				continue
			}
			var idxes []int
			if len(parentIdx) > 0 {
				idxes = append(idxes, parentIdx...)
			}
			idxes = append(idxes, index)
			dec, err := getFieldDecoder(el.Field(i), i, idxes, newParentJSONName)
			if err != nil {
				return nil, err
			}
			if dec != nil {
				decoders = append(decoders, dec...)
			}
		}

		return decoders, nil
	}

	return getBaseTypeTextDecoder(field, index, fieldTagInfos, parentIdx)
}
