// Copyright 2019 Bytedance Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validator_test

import (
	"fmt"

	vd "github.com/cloudwego/hertz/internal/tagexpr/validator"
)

func Example() {
	type InfoRequest struct {
		Name         string   `vd:"($!='Alice'||(Age)$==18) && regexp('\\w')"`
		Age          int      `vd:"$>0"`
		Email        string   `vd:"email($)"`
		Phone1       string   `vd:"phone($)"`
		OtherPhones  []string `vd:"range($, phone(#v,'CN'))"`
		*InfoRequest `vd:"?"`
		Info1        *InfoRequest `vd:"?"`
		Info2        *InfoRequest `vd:"-"`
	}
	info := &InfoRequest{
		Name:        "Alice",
		Age:         18,
		Email:       "henrylee2cn@gmail.com",
		Phone1:      "+8618812345678",
		OtherPhones: []string{"18812345679", "18812345680"},
	}
	fmt.Println(vd.Validate(info))

	type A struct {
		A    int `vd:"$<0||$>=100"`
		Info interface{}
	}
	info.Email = "xxx"
	a := &A{A: 107, Info: info}
	fmt.Println(vd.Validate(a))
	type B struct {
		B string `vd:"len($)>1 && regexp('^\\w*$')"`
	}
	b := &B{"abc"}
	fmt.Println(vd.Validate(b) == nil)

	type C struct {
		C bool `vd:"@:(S.A)$>0 && !$; msg:'C must be false when S.A>0'"`
		S *A
	}
	c := &C{C: true, S: a}
	fmt.Println(vd.Validate(c))

	type D struct {
		d []string `vd:"@:len($)>0 && $[0]=='D'; msg:sprintf('invalid d: %v',$)"`
	}
	d := &D{d: []string{"x", "y"}}
	fmt.Println(vd.Validate(d))

	type E struct {
		e map[string]int `vd:"len($)==$['len']"`
	}
	e := &E{map[string]int{"len": 2}}
	fmt.Println(vd.Validate(e))

	// Customizes the factory of validation error.
	vd.SetErrorFactory(func(failPath, msg string) error {
		return fmt.Errorf(`{"succ":false, "error":"validation failed: %s"}`, failPath)
	})

	type F struct {
		f struct {
			g int `vd:"$%3==0"`
		}
	}
	f := &F{}
	f.f.g = 10
	fmt.Println(vd.Validate(f))

	fmt.Println(vd.Validate(map[string]*F{"a": f}))
	fmt.Println(vd.Validate(map[string]map[string]*F{"a": {"b": f}}))
	fmt.Println(vd.Validate([]map[string]*F{{"a": f}}))
	fmt.Println(vd.Validate(struct {
		A []map[string]*F
	}{A: []map[string]*F{{"x": f}}}))
	fmt.Println(vd.Validate(map[*F]int{f: 1}))
	fmt.Println(vd.Validate([][1]*F{{f}}))
	fmt.Println(vd.Validate((*F)(nil)))
	fmt.Println(vd.Validate(map[string]*F{}))
	fmt.Println(vd.Validate(map[string]map[string]*F{}))
	fmt.Println(vd.Validate([]map[string]*F{}))
	fmt.Println(vd.Validate([]*F{}))

	// Output:
	// <nil>
	// email format is incorrect
	// true
	// C must be false when S.A>0
	// invalid d: [x y]
	// invalid parameter: e
	// {"succ":false, "error":"validation failed: f.g"}
	// {"succ":false, "error":"validation failed: {v for k=a}.f.g"}
	// {"succ":false, "error":"validation failed: {v for k=a}{v for k=b}.f.g"}
	// {"succ":false, "error":"validation failed: [0]{v for k=a}.f.g"}
	// {"succ":false, "error":"validation failed: A[0]{v for k=x}.f.g"}
	// {"succ":false, "error":"validation failed: {k}.f.g"}
	// {"succ":false, "error":"validation failed: [0][0].f.g"}
	// unsupported data: nil
	// <nil>
	// <nil>
	// <nil>
	// <nil>
}
