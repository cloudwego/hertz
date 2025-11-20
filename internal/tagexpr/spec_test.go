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

package tagexpr

import (
	"context"
	"reflect"
	"testing"
)

func TestReadPairedSymbol(t *testing.T) {
	cases := []struct {
		left, right             rune
		expr, val, lastExprNode string
	}{
		{left: '\'', right: '\'', expr: "'true '+'a'", val: "true ", lastExprNode: "+'a'"},
		{left: '(', right: ')', expr: "((0+1)/(2-1)*9)%2", val: "(0+1)/(2-1)*9", lastExprNode: "%2"},
		{left: '(', right: ')', expr: `(\)\(\))`, val: `)()`},
		{left: '\'', right: '\'', expr: `'\\'`, val: `\\`},
		{left: '\'', right: '\'', expr: `'\'\''`, val: `''`},
	}
	for _, c := range cases {
		t.Log(c.expr)
		expr := c.expr
		got := readPairedSymbol(&expr, c.left, c.right)
		if got == nil {
			t.Fatalf("expr: %q, got: %v, %q, want: %q, %q", c.expr, got, expr, c.val, c.lastExprNode)
		} else if *got != c.val || expr != c.lastExprNode {
			t.Fatalf("expr: %q, got: %q, %q, want: %q, %q", c.expr, *got, expr, c.val, c.lastExprNode)
		}
	}
}

func TestReadBoolExprNode(t *testing.T) {
	cases := []struct {
		expr         string
		val          bool
		lastExprNode string
	}{
		{expr: "false", val: false, lastExprNode: ""},
		{expr: "true", val: true, lastExprNode: ""},
		{expr: "true ", val: true, lastExprNode: " "},
		{expr: "!true&", val: false, lastExprNode: "&"},
		{expr: "!false|", val: true, lastExprNode: "|"},
		{expr: "!!!!false =", val: !!!!false, lastExprNode: " ="}, //nolint:staticcheck // SA4013: negating a boolean twice has no effect
	}
	for _, c := range cases {
		t.Log(c.expr)
		expr := c.expr
		e := readBoolExprNode(&expr)
		got := e.Run(context.TODO(), "", nil).(bool)
		if got != c.val || expr != c.lastExprNode {
			t.Fatalf("expr: %s, got: %v, %s, want: %v, %s", c.expr, got, expr, c.val, c.lastExprNode)
		}
	}
}

func TestReadDigitalExprNode(t *testing.T) {
	cases := []struct {
		expr         string
		val          float64
		lastExprNode string
	}{
		{expr: "0.1 +1", val: 0.1, lastExprNode: " +1"},
		{expr: "-1\\1", val: -1, lastExprNode: "\\1"},
		{expr: "1a", val: 0, lastExprNode: ""},
		{expr: "1", val: 1, lastExprNode: ""},
		{expr: "1.1", val: 1.1, lastExprNode: ""},
		{expr: "1.1/", val: 1.1, lastExprNode: "/"},
	}
	for _, c := range cases {
		expr := c.expr
		e := readDigitalExprNode(&expr)
		if c.expr == "1a" {
			if e != nil {
				t.Fatalf("expr: %s, got:%v, want:%v", c.expr, e.Run(context.TODO(), "", nil), nil)
			}
			continue
		}
		got := e.Run(context.TODO(), "", nil).(float64)
		if got != c.val || expr != c.lastExprNode {
			t.Fatalf("expr: %s, got: %f, %s, want: %f, %s", c.expr, got, expr, c.val, c.lastExprNode)
		}
	}
}

func TestFindSelector(t *testing.T) {
	cases := []struct {
		expr         string
		field        string
		name         string
		subSelector  []string
		boolOpposite bool
		signOpposite bool
		found        bool
		last         string
	}{
		{expr: "$", name: "$", found: true},
		{expr: "!!$", name: "$", found: true},
		{expr: "!$", name: "$", boolOpposite: true, found: true},
		{expr: "+$", name: "$", found: true},
		{expr: "--$", name: "$", found: true},
		{expr: "-$", name: "$", signOpposite: true, found: true},
		{expr: "---$", name: "$", signOpposite: true, found: true},
		{expr: "()$", last: "()$"},
		{expr: "(0)$", last: "(0)$"},
		{expr: "(A)$", field: "A", name: "$", found: true},
		{expr: "+(A)$", field: "A", name: "$", found: true},
		{expr: "++(A)$", field: "A", name: "$", found: true},
		{expr: "!(A)$", field: "A", name: "$", boolOpposite: true, found: true},
		{expr: "-(A)$", field: "A", name: "$", signOpposite: true, found: true},
		{expr: "(A0)$", field: "A0", name: "$", found: true},
		{expr: "!!(A0)$", field: "A0", name: "$", found: true},
		{expr: "--(A0)$", field: "A0", name: "$", found: true},
		{expr: "(A0)$(A1)$", last: "(A0)$(A1)$"},
		{expr: "(A0)$ $(A1)$", field: "A0", name: "$", found: true, last: " $(A1)$"},
		{expr: "$a", last: "$a"},
		{expr: "$[1]['a']", name: "$", subSelector: []string{"1", "'a'"}, found: true},
		{expr: "$[1][]", last: "$[1][]"},
		{expr: "$[[]]", last: "$[[]]"},
		{expr: "$[[[]]]", last: "$[[[]]]"},
		{expr: "$[(A)$[1]]", name: "$", subSelector: []string{"(A)$[1]"}, found: true},
		{expr: "$>0&&$<10", name: "$", found: true, last: ">0&&$<10"},
	}
	for _, c := range cases {
		last := c.expr
		field, name, subSelector, boolOpposite, signOpposite, found := findSelector(&last)
		if found != c.found {
			t.Fatalf("%q found: got: %v, want: %v", c.expr, found, c.found)
		}
		if c.boolOpposite && (boolOpposite == nil || !*boolOpposite) {
			t.Fatalf("%q boolOpposite: got: %v, want: %v", c.expr, boolOpposite, c.boolOpposite)
		}
		if c.signOpposite && (signOpposite == nil || !*signOpposite) {
			t.Fatalf("%q signOpposite: got: %v, want: %v", c.expr, signOpposite, c.signOpposite)
		}
		if field != c.field {
			t.Fatalf("%q field: got: %q, want: %q", c.expr, field, c.field)
		}
		if name != c.name {
			t.Fatalf("%q name: got: %q, want: %q", c.expr, name, c.name)
		}
		if !reflect.DeepEqual(subSelector, c.subSelector) {
			t.Fatalf("%q subSelector: got: %v, want: %v", c.expr, subSelector, c.subSelector)
		}
		if last != c.last {
			t.Fatalf("%q last: got: %q, want: %q", c.expr, last, c.last)
		}
	}
}
