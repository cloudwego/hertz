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

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

var errInvalidValue = errors.New("invalid value")

func newCustomDecodeFuncErr(typ reflect.Type, err error) error {
	return fmt.Errorf("call TypeUnmarshalFunc for type %s err: %w", typ, err)
}

type CustomDecodeFunc func(req *protocol.Request, params param.Params, s string) (reflect.Value, error)

func (f CustomDecodeFunc) Call(req *protocol.Request, params param.Params, s string, rv reflect.Value) error {
	v, err := f(req, params, s)
	if err != nil {
		return newCustomDecodeFuncErr(rv.Type(), err)
	}
	if !v.IsValid() {
		return newCustomDecodeFuncErr(rv.Type(), errInvalidValue)
	}
	if rv.Type() != v.Type() {
		return newCustomDecodeFuncErr(rv.Type(), fmt.Errorf("returned %s", v.Type()))
	}
	rv.Set(v)
	return nil
}

type customFuncDecoder struct {
	*fieldInfo
}

func (d *customFuncDecoder) Decode(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error) {
	s, ok, err := d.FetchBindValue(req, params)
	if !ok || err != nil {
		return false, err
	}
	f := d.FieldSetter(rv)
	rv = f.Value()
	if err := d.decodeFunc.Call(req, params, s, rv); err != nil {
		f.Reset()
		return false, err
	}
	return true, nil
}

func newCustomFuncDecoder(fi *fieldInfo) internalDecoder {
	if fi.decodeFunc == nil {
		panic("has no decodeFunc")
	}
	return &customFuncDecoder{
		fieldInfo: fi,
	}
}
