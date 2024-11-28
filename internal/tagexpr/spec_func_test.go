// Copyright 2019 Bytedance Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tagexpr_test

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/cloudwego/hertz/internal/tagexpr"
)

func TestFunc(t *testing.T) {
	emailRegexp := regexp.MustCompile(
		"^([A-Za-z0-9_\\-\\.\u4e00-\u9fa5])+\\@([A-Za-z0-9_\\-\\.])+\\.([A-Za-z]{2,8})$",
	)
	tagexpr.RegFunc("email", func(args ...interface{}) interface{} {
		if len(args) == 0 {
			return false
		}
		s, ok := args[0].(string)
		if !ok {
			return false
		}
		t.Log(s)
		return emailRegexp.MatchString(s)
	})

	vm := tagexpr.New("te")

	type T struct {
		Email string `te:"email($)"`
	}
	cases := []struct {
		email  string
		expect bool
	}{
		{"", false},
		{"henrylee2cn@gmail.com", true},
	}

	obj := new(T)
	for _, c := range cases {
		obj.Email = c.email
		te := vm.MustRun(obj)
		got := te.EvalBool("Email")
		if got != c.expect {
			t.Fatalf("email: %s, expect: %v, but got: %v", c.email, c.expect, got)
		}
	}

	// test len
	type R struct {
		Str string `vd:"mblen($)<6"`
	}
	lenCases := []struct {
		str    string
		expect bool
	}{
		{"123", true},
		{"一二三四五六七", false},
		{"一二三四五", true},
	}

	lenObj := new(R)
	vm = tagexpr.New("vd")
	for _, lenCase := range lenCases {
		lenObj.Str = lenCase.str
		te := vm.MustRun(lenObj)
		got := te.EvalBool("Str")
		if got != lenCase.expect {
			t.Fatalf("string: %v, expect: %v, but got: %v", lenCase.str, lenCase.expect, got)
		}
	}
}

func TestRangeIn(t *testing.T) {
	vm := tagexpr.New("te")
	type S struct {
		F []string `te:"range($, in(#v, '', 'ttp', 'euttp'))"`
	}
	a := []string{"ttp", "", "euttp"}
	r := vm.MustRun(S{
		F: a,
		// F: b,
	})
	expect := []interface{}{true, true, true}
	actual := r.Eval("F")
	if !reflect.DeepEqual(expect, actual) {
		t.Fatal("not equal", expect, actual)
	}
}
