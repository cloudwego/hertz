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

package tagexpr_test

import (
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/internal/tagexpr"
)

func TestIssue12(t *testing.T) {
	vm := tagexpr.New("te")
	type I int
	type S struct {
		F    []I              `te:"range($, '>'+sprintf('%v:%v', #k, #v+2+len($)))"`
		Fs   [][]I            `te:"range($, range(#v, '>'+sprintf('%v:%v', #k, #v+2+##)))"`
		M    map[string]I     `te:"range($, '>'+sprintf('%s:%v', #k, #v+2+##))"`
		MFs  []map[string][]I `te:"range($, range(#v, range(#v, '>'+sprintf('%v:%v', #k, #v+2+##))))"`
		MFs2 []map[string][]I `te:"range($, range(#v, range(#v, '>'+sprintf('%v:%v', #k, #v+2+##))))"`
	}
	a := []I{2, 3}
	r := vm.MustRun(S{
		F:    a,
		Fs:   [][]I{a},
		M:    map[string]I{"m0": 2, "m1": 3},
		MFs:  []map[string][]I{{"m": a}},
		MFs2: []map[string][]I{},
	})
	assertEqual(t, []interface{}{">0:6", ">1:7"}, r.Eval("F"))
	assertEqual(t, []interface{}{[]interface{}{">0:6", ">1:7"}}, r.Eval("Fs"))
	assertEqual(t, []interface{}{[]interface{}{[]interface{}{">0:6", ">1:7"}}}, r.Eval("MFs"))
	assertEqual(t, []interface{}{}, r.Eval("MFs2"))
	assertEqual(t, true, r.EvalBool("MFs2"))

	// result may not stable for map
	got := r.Eval("M")
	if !reflect.DeepEqual([]interface{}{">m0:6", ">m1:7"}, got) &&
		!reflect.DeepEqual([]interface{}{">m1:7", ">m0:6"}, got) {
		t.Fatal(got)
	}
}
