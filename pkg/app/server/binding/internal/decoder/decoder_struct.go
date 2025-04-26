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
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type structDecoder struct {
	*fieldInfo

	dd []internalDecoder
}

func newStructDecoder(fi *fieldInfo) (internalDecoder, error) {
	c := fi.c
	rt := fi.fieldType
	if rt.Kind() != reflect.Struct {
		return nil, errors.New("structDecoder: not struct type")
	}
	if rt == fileBindingType {
		return &fileTypeDecoder{fieldInfo: fi}, nil
	}

	ret := &structDecoder{fieldInfo: fi}

	// check if any closed cycle
	parent := fi.parent
	for parent != nil {
		if parent.fieldType == rt {
			return ret, nil
		}
		parent = parent.parent
	}

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		ffi := newFieldInfo(fi, f)

		if !f.IsExported() && !f.Anonymous {
			// ignore unexported field
			continue
		}
		if f.Anonymous && f.Type.Kind() != reflect.Struct {
			// only anonymous struct is allowed
			continue
		}

		if c.DecodeTag != "" { // for BindXXX, if BindForm, c.DecodeTag = "form"
			ffi.tagInfos = []*TagInfo{getFieldTagInfoByTag(f, c.DecodeTag)}
		} else {
			ffi.tagInfos = lookupFieldTags(f, c)
			if len(ffi.tagInfos) == 0 && !c.DisableDefaultTag {
				ffi.tagInfos = getDefaultFieldTags(f)
			}
		}
		tt := ffi.tagInfos[:0]
		for _, ti := range ffi.tagInfos {
			if ti.Skip() {
				continue
			}
			tt = append(tt, ti)
		}
		ffi.tagInfos = tt

		// update jsonName
		for i, ti := range ffi.tagInfos {
			if ti.IsJSON() {
				ffi.jsonName = ti.Name
				ffi.jsonTag = i
				break
			}
		}
		ffi.jsonKeys = append(make([]any, 0, len(fi.jsonKeys)+1), fi.jsonKeys...)
		ffi.jsonKeys = append(ffi.jsonKeys, ffi.jsonName)

		dec, err := getFieldDecoder(ffi)
		if err != nil {
			return nil, err
		}
		ret.dd = append(ret.dd, dec)

		if ffi.jsonTag < 0 {
			// GC unused data
			ffi.jsonName = ""
			ffi.jsonKeys = nil
		}
	}

	return ret, nil
}

func (d *structDecoder) decodeWholeStruct(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error) {
	s, ok, err := d.FetchBindValue(req, params)
	if !ok || err != nil {
		return false, err
	}
	if err := json.Unmarshal(bytesconv.S2b(s), rv.Addr().Interface()); err != nil {
		logger().Infof("json marshal struct %T err: %s, but it may not affect correctness, so skip it", d.fieldType.Name(), err)
		return false, nil
	}
	return true, nil
}

func (d *structDecoder) Decode(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error) {
	f := d.FieldSetter(rv)
	rv = f.Value()

	changed := false
	if !d.c.DisableStructFieldResolve {
		if ok, err := d.decodeWholeStruct(req, params, rv); err != nil {
			f.Reset()
			return false, err
		} else {
			changed = changed || ok
		}
	}

	for _, d := range d.dd {
		fieldChanged, err := d.Decode(req, params, rv)
		if err != nil {
			f.Reset()
			return changed, fmt.Errorf("decode field %q err: %w", d.GetFieldInfo().fieldName, err)
		}
		changed = changed || fieldChanged
	}
	if !changed {
		f.Reset()
	}
	return changed, nil
}
