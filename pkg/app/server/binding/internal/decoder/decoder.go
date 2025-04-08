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

package decoder

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

var byteSliceType = reflect.TypeOf([]byte(nil))

func logger() hlog.FullLogger {
	return hlog.SystemLogger()
}

type Decoder interface {
	Decode(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error)
}

type internalDecoder interface {
	Decoder

	GetFieldInfo() *fieldInfo
}

type DecodeConfig struct {
	LooseZeroMode             bool
	DisableDefaultTag         bool
	DisableStructFieldResolve bool
	TypeUnmarshalFuncs        map[reflect.Type]CustomDecodeFunc

	DecodeTag string
}

func NewDecoder(rt reflect.Type, config *DecodeConfig) (Decoder, error) {
	if rt.Kind() != reflect.Pointer {
		return nil, errors.New("not pointer type")
	}
	rt = rt.Elem()
	if rt.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported %s type binding", rt)
	}
	return newStructDecoder(newRootFieldInfo(rt, config))
}

func getFieldDecoder(fi *fieldInfo) (internalDecoder, error) {
	ft := fi.fieldType
	// customized type decoder has the highest priority
	if fi.decodeFunc != nil {
		return newCustomFuncDecoder(fi), nil
	}
	switch ft.Kind() {
	case reflect.Slice, reflect.Array:
		return newSliceDecoder(fi), nil

	case reflect.Map:
		return newMapDecoder(fi), nil

	case reflect.Struct:
		return newStructDecoder(fi)

	default:
		return newBaseDecoder(fi), nil
	}
}
