/*
 * Copyright 2024 CloudWeGo Authors
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

package tagexpr

import (
	"reflect"
	"unsafe"
)

func init() {
	testhack()
}

func dereferenceValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	return v
}

func dereferenceType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func dereferenceInterfaceValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	return v
}

type rvtype struct { // reflect.Value
	abiType uintptr
	ptr     unsafe.Pointer // data pointer
}

func rvPtr(rv reflect.Value) unsafe.Pointer {
	return (*rvtype)(unsafe.Pointer(&rv)).ptr
}

func rvType(rv reflect.Value) uintptr {
	return (*rvtype)(unsafe.Pointer(&rv)).abiType
}

func rtType(rt reflect.Type) uintptr {
	type iface struct {
		tab  uintptr
		data uintptr
	}
	return (*iface)(unsafe.Pointer(&rt)).data
}

// quick test make sure the hack above works
func testhack() {
	type T1 struct {
		a int
	}
	type T2 struct {
		a int
	}
	p0 := &T1{1}
	p1 := &T1{2}
	p2 := &T2{3}

	if rvPtr(reflect.ValueOf(p0)) != unsafe.Pointer(p0) ||
		rvPtr(reflect.ValueOf(p0).Elem()) != unsafe.Pointer(p0) ||
		rvPtr(reflect.ValueOf(p0)) == rvPtr(reflect.ValueOf(p1)) {
		panic("rvPtr() compatibility issue found")
	}

	if rvType(reflect.ValueOf(p0)) != rvType(reflect.ValueOf(p1)) ||
		rvType(reflect.ValueOf(p0)) == rvType(reflect.ValueOf(p2)) ||
		rvType(reflect.ValueOf(p0).Elem()) != rvType(reflect.ValueOf(p1).Elem()) ||
		rvType(reflect.ValueOf(p0).Elem()) == rvType(reflect.ValueOf(p2).Elem()) {
		panic("rvType() compatibility issue found")
	}

	if rtType(reflect.TypeOf(p0)) != rtType(reflect.TypeOf(p1)) ||
		rtType(reflect.TypeOf(p0)) == rtType(reflect.TypeOf(p2)) ||
		rtType(reflect.TypeOf(p0).Elem()) != rtType(reflect.TypeOf(p1).Elem()) ||
		rtType(reflect.TypeOf(p0).Elem()) == rtType(reflect.TypeOf(p2).Elem()) {
		panic("rtType() compatibility issue found")
	}
}
