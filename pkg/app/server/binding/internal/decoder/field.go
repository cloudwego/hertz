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
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type fieldInfo struct {
	parent *fieldInfo

	index     int
	fieldName string
	fieldType reflect.Type

	jsonName string
	jsonKeys []any // for optimizing `sonic.Get`
	jsonTag  int

	defaultv string
	tagInfos []*TagInfo // ordered by precedence

	decodeFunc     CustomDecodeFunc
	elemDecodeFunc CustomDecodeFunc // for slice

	c *DecodeConfig

	// true if getter convert []byte to string without copy
	// it only used for basic types like int/float etc which is safe
	// NOTE: use []byte for all funcs in the future?
	unsafestr bool
}

func newRootFieldInfo(rt reflect.Type, c *DecodeConfig) *fieldInfo {
	rt = dereferenceType(rt)
	fi := &fieldInfo{index: -1, fieldType: rt, jsonTag: -1, c: c}
	fi.init()
	return fi
}

func newFieldInfo(parent *fieldInfo, f reflect.StructField) *fieldInfo {
	c := parent.c
	rt := dereferenceType(f.Type)
	fi := &fieldInfo{
		parent:    parent,
		index:     f.Index[len(f.Index)-1],
		fieldName: f.Name,
		jsonName:  f.Name,
		fieldType: rt,
		jsonTag:   -1,
		c:         c,
	}
	defaultv, _ := f.Tag.Lookup(defaultTag)
	fi.defaultv = toDefaultValue(rt, defaultv)
	fi.init()
	return fi
}

func (f *fieldInfo) init() {
	c := f.c
	rt := f.fieldType

	f.decodeFunc = c.TypeUnmarshalFuncs[rt]
	if f.IsRepeated() {
		f.elemDecodeFunc = c.TypeUnmarshalFuncs[dereferenceType(rt.Elem())]
	}

	// for base types we use unsafestr
	f.unsafestr = getBaseDecodeByKind(rt.Kind()) != nil
	if rt.Kind() == reflect.String {
		f.unsafestr = false // for string, it keeps ref to the value, must be a copy
	}
	if f.decodeFunc != nil || f.elemDecodeFunc != nil {
		f.unsafestr = false
	}
}

func (f *fieldInfo) IsRepeated() bool {
	switch f.fieldType.Kind() {
	case reflect.Slice, reflect.Array:
		return true
	}
	return false
}

func (f *fieldInfo) FieldSetter(rv reflect.Value) FieldSetter {
	return newFieldSetter(f, rv)
}

// FieldValue is shortcut of FieldSetter(rv).Value()
func (f *fieldInfo) FieldValue(rv reflect.Value) reflect.Value {
	rv = dereference2lvalue(rv)
	if f.index >= 0 {
		rv = rv.Field(f.index)
	}
	return dereference2lvalue(rv)
}

func jsonKeyExist(req *protocol.Request, key []any) bool {
	node, _ := sonic.Get(req.Body(), key...)
	return node.Exists()
}

func (f *fieldInfo) FullJSONPath() string {
	x := &strings.Builder{}
	for i, k := range f.jsonKeys {
		if i > 0 {
			_ = x.WriteByte('.')
		}
		_, _ = x.WriteString(k.(string))
	}
	return x.String()
}

func (f *fieldInfo) checkJSON(req *protocol.Request) (ok bool, err error) {
	if f.jsonTag < 0 {
		return false, nil
	}
	return f.checkJSONSlow(req)
}

func (f *fieldInfo) checkJSONSlow(req *protocol.Request) (ok bool, err error) {
	ti := f.tagInfos[f.jsonTag]
	if !ti.Required && f.defaultv == "" {
		// fast path if not ti.Required && has no default value
		// need to check jsonKeys for ti.Required
		// also need to check if we should use defaultv if we have json body
		return false, nil
	}

	ct := bytesconv.B2s(req.Header.ContentType())
	if !strings.EqualFold(utils.FilterContentType(ct), consts.MIMEApplicationJSON) {
		if ti.Required {
			return false, fmt.Errorf("JSON field %q is required, request body is not JSON", f.FullJSONPath())
		}
		return false, nil
	}

	if jsonKeyExist(req, f.jsonKeys) {
		return true, nil
	}

	if ti.Required {
		// it's ok if parent is empty for a required field
		if len(f.jsonKeys) <= 1 || jsonKeyExist(req, f.jsonKeys[:len(f.jsonKeys)-1]) {
			return false, fmt.Errorf("JSON field %q is required, but it's not set", f.FullJSONPath())
		}
	}
	return false, nil
}

var errFieldNotSet = errors.New("field is required, but not set")

func (f *fieldInfo) FetchBindValue(req *protocol.Request, params param.Params) (v string, ok bool, err error) {
	jsonExist, requiredErr := f.checkJSON(req)

	exist := false
	s := ""
	for _, ti := range f.tagInfos {
		if ti.Getter == nil {
			continue
		}
		s, exist = ti.Getter(f, req, params, ti.Name)
		if exist {
			requiredErr = nil
			break
		}
		if ti.Required && !jsonExist {
			requiredErr = errFieldNotSet
		}
	}
	if requiredErr != nil {
		return "", false, requiredErr
	}
	if len(s) == 0 && !jsonExist {
		s = f.defaultv
	}
	if !exist && len(s) == 0 {
		return "", false, nil
	}
	return s, true, nil
}

// GetFieldInfo ... used by struct decoder to fetch info of a field
func (f *fieldInfo) GetFieldInfo() *fieldInfo {
	return f
}
