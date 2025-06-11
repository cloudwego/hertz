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
	"testing"
)

func TestDereference(t *testing.T) {
	var p *int
	rv := dereference2lvalue(reflect.ValueOf(&p))
	rv.SetInt(7)
	if p == nil || *p != 7 {
		t.Fatal(p)
	}
	rt := dereferenceType(reflect.TypeOf(&p))
	if rt != reflect.TypeOf(int(0)) {
		t.Fatal(rt)
	}
}

func fIndex(v any, name string) int {
	rv := reflect.TypeOf(v)
	rv = dereferenceType(rv)
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if f.Name == name {
			return i
		}
	}
	panic("not found")
}

func TestFieldSetter(t *testing.T) {
	type Msg struct {
		S0 *string
		S1 string
	}
	p := &Msg{}

	{ // p.S0
		fi := &fieldInfo{
			index: fIndex(p, "S0"),
		}
		f := fi.FieldSetter(reflect.ValueOf(p))
		f.Value().Set(reflect.ValueOf("hello"))
		if p.S0 == nil || *p.S0 != "hello" {
			t.Fatal(p)
		}

		// Reset
		p.S0 = nil
		p.S1 = "hello1"
		f = fi.FieldSetter(reflect.ValueOf(p))
		f.Value().Set(reflect.ValueOf("hello0"))
		if *p.S0 != "hello0" || p.S1 != "hello1" {
			t.Fatal(p)
		}
		f.Reset()
		if p.S0 != nil {
			t.Fatal(p)
		}

	}
	{ // p.S1
		fi := &fieldInfo{
			index: fIndex(p, "S1"),
		}
		f := fi.FieldSetter(reflect.ValueOf(p))
		f.Value().Set(reflect.ValueOf("world"))
		if p.S1 != "world" {
			t.Fatal(p)
		}
		f.Reset() // noop
		if p.S1 != "world" {
			t.Fatal(p)
		}
		fi.FieldValue(reflect.ValueOf(p)).Set(reflect.ValueOf("test"))
		if p.S1 != "test" {
			t.Fatal(p)
		}
	}
}
