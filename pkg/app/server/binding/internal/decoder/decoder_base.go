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
	"fmt"
	"reflect"
	"strconv"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type baseDecoder struct {
	*fieldInfo

	decodeValue func(rv reflect.Value, s string) error
}

func newBaseDecoder(fi *fieldInfo) internalDecoder {
	fn := getBaseDecodeByKind(fi.fieldType.Kind())
	if fn == nil {
		fn = decodeJSON
	}
	return &baseDecoder{fieldInfo: fi, decodeValue: fn}
}

func (d *baseDecoder) Decode(req *protocol.Request, params param.Params, rv reflect.Value) (bool, error) {
	s, ok, err := d.FetchBindValue(req, params)
	if !ok || err != nil {
		return false, err
	}

	if s == "" && d.c.LooseZeroMode {
		d.FieldValue(rv).Set(reflect.Zero(d.fieldType)) // SetZero() >= go1.20
		return true, nil
	}

	f := d.FieldSetter(rv)
	rv = f.Value()
	if err := d.decodeValue(rv, s); err != nil {
		f.Reset()
		return false, fmt.Errorf("unable to decode '%s' as %s: %w", s, d.fieldType.Name(), err)
	}
	return true, nil
}

// use slice for better performance,
var type2decoder = [...]func(rv reflect.Value, s string) error{
	reflect.Bool:    decodeBool,
	reflect.Uint:    decodeUint,
	reflect.Uint8:   decodeUint8,
	reflect.Uint16:  decodeUint16,
	reflect.Uint32:  decodeUint32,
	reflect.Uint64:  decodeUint64,
	reflect.Int:     decodeInt,
	reflect.Int8:    decodeInt8,
	reflect.Int16:   decodeInt16,
	reflect.Int32:   decodeInt32,
	reflect.Int64:   decodeInt64,
	reflect.String:  decodeString,
	reflect.Float32: decodeFloat32,
	reflect.Float64: decodeFloat64,
}

func getBaseDecodeByKind(k reflect.Kind) (ret func(rv reflect.Value, s string) error) {
	if int(k) >= len(type2decoder) {
		return nil
	}
	return type2decoder[k]
}

func decodeJSON(rv reflect.Value, s string) error {
	return json.Unmarshal(bytesconv.S2b(s), rv.Addr().Interface())
}

func decodeBool(rv reflect.Value, s string) error {
	v, err := strconv.ParseBool(s)
	if err == nil {
		*(*bool)(rvUnsafePointer(&rv)) = v
	}
	return err

}

func decodeUint(rv reflect.Value, s string) error {
	v, err := strconv.ParseUint(s, 10, 0)
	if err == nil {
		*(*uint)(rvUnsafePointer(&rv)) = uint(v)
	}
	return err

}

func decodeUint8(rv reflect.Value, s string) error {
	v, err := strconv.ParseUint(s, 10, 8)
	if err == nil {
		*(*uint8)(rvUnsafePointer(&rv)) = uint8(v)
	}
	return err

}

func decodeUint16(rv reflect.Value, s string) error {
	v, err := strconv.ParseUint(s, 10, 16)
	if err == nil {
		*(*uint16)(rvUnsafePointer(&rv)) = uint16(v)
	}
	return err
}

func decodeUint32(rv reflect.Value, s string) error {
	v, err := strconv.ParseUint(s, 10, 32)
	if err == nil {
		*(*uint32)(rvUnsafePointer(&rv)) = uint32(v)
	}
	return err

}

func decodeUint64(rv reflect.Value, s string) error {
	v, err := strconv.ParseUint(s, 10, 64)
	if err == nil {
		*(*uint64)(rvUnsafePointer(&rv)) = v
	}
	return err

}

func decodeInt(rv reflect.Value, s string) error {
	v, err := strconv.Atoi(s)
	if err == nil {
		*(*int)(rvUnsafePointer(&rv)) = v
	}
	return err

}

func decodeInt8(rv reflect.Value, s string) error {
	v, err := strconv.ParseInt(s, 10, 8)
	if err == nil {
		*(*int8)(rvUnsafePointer(&rv)) = int8(v)
	}
	return err

}

func decodeInt16(rv reflect.Value, s string) error {
	v, err := strconv.ParseInt(s, 10, 16)
	if err == nil {
		*(*int16)(rvUnsafePointer(&rv)) = int16(v)
	}
	return err
}

func decodeInt32(rv reflect.Value, s string) error {
	v, err := strconv.ParseInt(s, 10, 32)
	if err == nil {
		*(*int32)(rvUnsafePointer(&rv)) = int32(v)
	}
	return err
}

func decodeInt64(rv reflect.Value, s string) error {
	v, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		*(*int64)(rvUnsafePointer(&rv)) = v
	}
	return err
}

func decodeString(rv reflect.Value, s string) error {
	*(*string)(rvUnsafePointer(&rv)) = s
	return nil
}

func decodeFloat32(rv reflect.Value, s string) error {
	v, err := strconv.ParseFloat(s, 32)
	if err == nil {
		*(*float32)(rvUnsafePointer(&rv)) = float32(v)
	}
	return err
}

func decodeFloat64(rv reflect.Value, s string) error {
	v, err := strconv.ParseFloat(s, 64)
	if err == nil {
		*(*float64)(rvUnsafePointer(&rv)) = v
	}
	return err
}
