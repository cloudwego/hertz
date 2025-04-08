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

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/common/json"
	hJson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type sliceDecoder struct {
	*fieldInfo
}

func newSliceDecoder(fi *fieldInfo) internalDecoder {
	switch fi.fieldType.Kind() {
	case reflect.Slice, reflect.Array:
		elemType := dereferenceType(fi.fieldType.Elem())
		if elemType == fileBindingType {
			return &fileTypeDecoder{fieldInfo: fi}
		}
		return &sliceDecoder{fieldInfo: fi}
	}
	panic(fmt.Sprintf("unexpected type %s", fi.fieldType))
}

func (d *sliceDecoder) Decode(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error) {
	jsonExist, requiredErr := d.checkJSON(req)

	ss := []string{}
	for _, ti := range d.tagInfos {
		if ti.Tag == rawBodyTag && d.fieldType == byteSliceType {
			d.FieldValue(rv).Set(reflect.ValueOf(req.Body()))
			return true, nil // raw_body must be the last tag in d.tagInfos
		}
		if ti.SliceGetter == nil {
			continue
		}
		ss = ti.SliceGetter(req, params, ti.Name)
		if len(ss) > 0 {
			requiredErr = nil
			break
		}
		if ti.Required {
			requiredErr = errFieldNotSet
		}
	}

	if requiredErr != nil && !jsonExist {
		return false, errFieldNotSet
	}

	var err error
	if len(ss) == 0 {
		if !jsonExist && len(d.defaultv) > 0 {
			// try use default value to decode
			f := d.FieldSetter(rv)
			err = hJson.Unmarshal(bytesconv.S2b(d.defaultv), f.Value().Addr().Interface())
			if err != nil {
				f.Reset()
				return false, fmt.Errorf("json unmarshal %q err: %v", d.defaultv, err)
			}
			return true, nil
		}
		// if json exists and len(ss) == 0
		return false, nil
	}

	if f := d.fieldType; f.Kind() == reflect.Array && len(ss) != f.Len() {
		return false, fmt.Errorf("%q is not valid value for %s", ss, f)
	}

	f := d.FieldSetter(rv)
	v := f.Value()
	if v.Kind() == reflect.Slice {
		v.Set(reflect.MakeSlice(v.Type(), len(ss), len(ss)))
	}
	for i, s := range ss {
		if err = d.decodeElem(v.Index(i), s, req, params); err != nil {
			break
		}
	}
	if err != nil {
		if !v.CanAddr() {
			f.Reset()
			return false, err
		}
		// XXX: ??? another undocumented impl??? use ss[0] for the whole slice?
		// see: https://github.com/cloudwego/hertz/blob/v0.9.6/pkg/app/server/binding/internal/decoder/slice_type_decoder.go#L164
		err1 := hJson.Unmarshal(bytesconv.S2b(ss[0]), v.Addr().Interface())
		if err1 != nil {
			f.Reset()
			return false, fmt.Errorf("err: %w, fallback json unmarshal err: %v", err, err1)
		}
	}
	return true, nil
}

var errUnsupportedType = errors.New("unsupported")

func (d *sliceDecoder) decodeElem(rv reflect.Value, s string, req *protocol.Request, params param.Params) error {
	rv = dereference2lvalue(rv)
	rt := rv.Type()

	if d.elemDecodeFunc != nil {
		return d.elemDecodeFunc.Call(req, params, s, rv)
	}

	if s == "" && d.c.LooseZeroMode {
		rv.Set(reflect.Zero(rt)) // SetZero() >= go1.20
		return nil
	}

	kd := rt.Kind()
	if fn := getBaseDecodeByKind(kd); fn != nil {
		return fn(rv, s)
	}

	switch kd {
	case reflect.Struct, reflect.Map, reflect.Interface:
		return json.Unmarshal(bytesconv.S2b(s), rv.Addr().Interface())

	case reflect.Array, reflect.Slice:
		rv.Set(reflect.Zero(rt)) // any case?
		return nil

	default:
		return errUnsupportedType

	}
}
