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
	"unsafe"
)

// dereferencing a ptr value to left value for assignment
func dereference2lvalue(rv reflect.Value) reflect.Value {
	if rv.Kind() != reflect.Pointer {
		return rv
	}
	return dereference2lvalueSlow(rv)
}

func dereference2lvalueSlow(rv reflect.Value) reflect.Value {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}
	return rv
}

func dereferenceType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

type FieldSetter struct {
	f   reflect.Value
	old unsafe.Pointer
}

func newFieldSetter(f *fieldInfo, rv reflect.Value) FieldSetter {
	rv = dereference2lvalue(rv)
	if f.index >= 0 {
		rv = rv.Field(f.index)
	}
	p := FieldSetter{f: rv}
	if rv.Kind() == reflect.Pointer {
		// Field() may change the field value
		// save for Reset() if needed
		p.old = rv.UnsafePointer()
	}
	return p
}

// Value returns reflect.Value for assignment.
func (f *FieldSetter) Value() reflect.Value {
	// can not call dereference2lvalue directly,
	// then this method cost 81 which is too large for inline
	if f.f.Kind() != reflect.Pointer {
		return f.f
	}
	return dereference2lvalueSlow(f.f)
}

// Reset resets the underlying field to its original value. Currently only supports reflect.Pointer
func (f *FieldSetter) Reset() {
	if f.f.Kind() == reflect.Pointer {
		f.f.Set(reflect.NewAt(f.f.Type().Elem(), f.old))
	}
}

type reflectValue struct {
	typ_ uintptr
	ptr  unsafe.Pointer
}

func rvUnsafePointer(rv *reflect.Value) unsafe.Pointer {
	return (*reflectValue)(unsafe.Pointer(rv)).ptr
}
