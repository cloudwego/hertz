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
	"fmt"

	"github.com/cloudwego/hertz/internal/tagexpr"
)

func Example() {
	type T struct {
		A  int             `tagexpr:"$<0||$>=100"`
		B  string          `tagexpr:"len($)>1 && regexp('^\\w*$')"`
		C  bool            `tagexpr:"expr1:(f.g)$>0 && $; expr2:'C must be true when T.f.g>0'"`
		d  []string        `tagexpr:"@:len($)>0 && $[0]=='D'; msg:sprintf('invalid d: %v',$)"`
		e  map[string]int  `tagexpr:"len($)==$['len']"`
		e2 map[string]*int `tagexpr:"len($)==$['len']"`
		f  struct {
			g int `tagexpr:"$"`
		}
		h int `tagexpr:"$>minVal"`
	}

	vm := tagexpr.New("tagexpr")
	t := &T{
		A:  107,
		B:  "abc",
		C:  true,
		d:  []string{"x", "y"},
		e:  map[string]int{"len": 1},
		e2: map[string]*int{"len": new(int)},
		f: struct {
			g int `tagexpr:"$"`
		}{1},
		h: 10,
	}

	tagExpr, err := vm.Run(t)
	if err != nil {
		panic(err)
	}

	fmt.Println(tagExpr.Eval("A"))
	fmt.Println(tagExpr.Eval("B"))
	fmt.Println(tagExpr.Eval("C@expr1"))
	fmt.Println(tagExpr.Eval("C@expr2"))
	if !tagExpr.Eval("d").(bool) {
		fmt.Println(tagExpr.Eval("d@msg"))
	}
	fmt.Println(tagExpr.Eval("e"))
	fmt.Println(tagExpr.Eval("e2"))
	fmt.Println(tagExpr.Eval("f.g"))
	fmt.Println(tagExpr.EvalWithEnv("h", map[string]interface{}{"minVal": 9}))
	fmt.Println(tagExpr.EvalWithEnv("h", map[string]interface{}{"minVal": 11}))

	// Output:
	// true
	// true
	// true
	// C must be true when T.f.g>0
	// invalid d: [x y]
	// true
	// false
	// 1
	// true
	// false
}
