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
	"reflect"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type mapDeocoder struct {
	*fieldInfo
}

func newMapDecoder(fi *fieldInfo) internalDecoder {
	if fi.fieldType.Kind() != reflect.Map {
		panic("not reflect.Map")
	}
	return &mapDeocoder{fieldInfo: fi}
}

func (d *mapDeocoder) Decode(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error) {
	s, ok, err := d.FetchBindValue(req, params)
	if !ok || err != nil {
		return false, err
	}
	f := d.FieldSetter(rv)
	rv = f.Value()
	if s == "" && d.c.LooseZeroMode {
		rv.Set(reflect.MakeMapWithSize(d.fieldType, 0))
		return true, nil
	}
	if err := json.Unmarshal(bytesconv.S2b(s), rv.Addr().Interface()); err != nil {
		f.Reset()
		return false, newJSONDecodeErr(d.fieldType, err)
	}
	return true, nil
}
