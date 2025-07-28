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
	"math"
	"reflect"
	"testing"
)

func TestExpr(t *testing.T) {
	cases := []struct {
		expr string
		val  interface{}
	}{
		// Simple string
		{expr: "'a'", val: "a"},
		{expr: "('a')", val: "a"},
		// Simple digital
		{expr: " 10 ", val: 10.0},
		{expr: "(10)", val: 10.0},
		// Simple bool
		{expr: "true", val: true},
		{expr: "!true", val: false},
		{expr: "!!true", val: true},
		{expr: "false", val: false},
		{expr: "!false", val: true},
		{expr: "!!false", val: false},
		{expr: "(false)", val: false},
		{expr: "(!false)", val: true},
		{expr: "(!!false)", val: false},
		{expr: "!!(!false)", val: true},
		{expr: "!(!false)", val: false},
		// Join string
		{expr: "'true '+('a')", val: "true a"},
		{expr: "'a'+('b'+'c')+'d'", val: "abcd"},
		// Arithmetic operator
		{expr: "1+7+2", val: 10.0},
		{expr: "1+(7)+(2)", val: 10.0},
		{expr: "1.1+ 2", val: 3.1},
		{expr: "-1.1+4", val: 2.9},
		{expr: "10-7-2", val: 1.0},
		{expr: "20/2", val: 10.0},
		{expr: "1/0", val: math.NaN()},
		{expr: "20%2", val: 0.0},
		{expr: "6 % 5", val: 1.0},
		{expr: "20%7 %5", val: 1.0},
		{expr: "1*2+7+2.2", val: 11.2},
		{expr: "-20/2+1+2", val: -7.0},
		{expr: "20/2+1-2-1", val: 8.0},
		{expr: "30/(2+1)/5-2-1", val: -1.0},
		{expr: "100/(( 2+8)*5 )-(1 +1- 0)", val: 0.0},
		{expr: "(2*3)+(4*2)", val: 14.0},
		{expr: "1+(2*(3+4))", val: 15.0},
		{expr: "20%(7%5)", val: 0.0},
		// Relational operator
		{expr: "50 == 5", val: false},
		{expr: "'50'==50", val: true},
		{expr: "'50'=='50'", val: true},
		{expr: "'50' =='5' == true", val: false},
		{expr: "50== 50 == false", val: false},
		{expr: "50== 50 == true ==true==true", val: true},
		{expr: "50 != 5", val: true},
		{expr: "'50'!=50", val: false},
		{expr: "'50'!= '50'", val: false},
		{expr: "'50' !='5' != true", val: false},
		{expr: "50!= 50 == false", val: true},
		{expr: "50== 50 != true ==true!=true", val: true},
		{expr: "50 > 5", val: true},
		{expr: "50.1 > 50.1", val: false},
		{expr: "3.2 > 2.1", val: true},
		{expr: "'3.2' > '2.1'", val: true},
		{expr: "'13.2'>'2.1'", val: false},
		{expr: "3.2 >= 2.1", val: true},
		{expr: "2.1 >= 2.1", val: true},
		{expr: "2.05 >= 2.1", val: false},
		{expr: "'2.05'>='2.1'", val: false},
		{expr: "'12.05'>='2.1'", val: false},
		{expr: "50 < 5", val: false},
		{expr: "50.1 < 50.1", val: false},
		{expr: "3 <12.11", val: true},
		{expr: "3.2 < 2.1", val: false},
		{expr: "'3.2' < '2.1'", val: false},
		{expr: "'13.2' < '2.1'", val: true},
		{expr: "3.2 <= 2.1", val: false},
		{expr: "2.1 <= 2.1", val: true},
		{expr: "2.05 <= 2.1", val: true},
		{expr: "'2.05'<='2.1'", val: true},
		{expr: "'12.05'<='2.1'", val: true},
		// Logical operator
		{expr: "!('13.2' < '2.1')", val: false},
		{expr: "(3.2 <= 2.1) &&true", val: false},
		{expr: "true&&(2.1<=2.1)", val: true},
		{expr: "(2.05<=2.1)&&false", val: false},
		{expr: "true&&!true&&false", val: false},
		{expr: "true&&true&&true", val: true},
		{expr: "true&&true&&false", val: false},
		{expr: "false&&true&&true", val: false},
		{expr: "true && false && true", val: false},
		{expr: "true||false", val: true},
		{expr: "false ||true", val: true},
		{expr: "true&&true || false", val: true},
		{expr: "true&&false || false", val: false},
		{expr: "true && false || true ", val: true},
	}
	for _, c := range cases {
		t.Log(c.expr)
		vm, err := parseExpr(c.expr)
		if err != nil {
			t.Fatal(err)
		}
		val := vm.run("", nil)
		if !reflect.DeepEqual(val, c.val) {
			if f, ok := c.val.(float64); ok && math.IsNaN(f) && math.IsNaN(val.(float64)) {
				continue
			}
			t.Fatalf("expr: %q, got: %v, expect: %v", c.expr, val, c.val)
		}
	}
}

func TestExprWithEnv(t *testing.T) {
	cases := []struct {
		expr string
		val  interface{}
	}{
		// env: a = 10, b = "string value",
		{expr: "a", val: 10.0},
		{expr: "b", val: "string value"},
		{expr: "a>10", val: false},
		{expr: "a<11", val: true},
		{expr: "a+1", val: 11.0},
		{expr: "a==10", val: true},
	}

	for _, c := range cases {
		t.Log(c.expr)
		vm, err := parseExpr(c.expr)
		if err != nil {
			t.Fatal(err)
		}
		val := vm.runWithEnv("", nil, map[string]interface{}{"a": 10, "b": "string value"})
		if !reflect.DeepEqual(val, c.val) {
			if f, ok := c.val.(float64); ok && math.IsNaN(f) && math.IsNaN(val.(float64)) {
				continue
			}
			t.Fatalf("expr: %q, got: %v, expect: %v", c.expr, val, c.val)
		}
	}
}

func TestPriority(t *testing.T) {
	cases := []struct {
		expr string
		val  interface{}
	}{
		{expr: "false||true&&8==8", val: true},
		{expr: "1+2>5-4", val: true},
		{expr: "1+2*4/2", val: 5.0},
		{expr: "(true||false)&&false||false", val: false},
		{expr: "true||false&&false||false", val: true},
		{expr: "true||1<0&&'a'!='a'||0!=0", val: true},
	}
	for _, c := range cases {
		t.Log(c.expr)
		vm, err := parseExpr(c.expr)
		if err != nil {
			t.Fatal(err)
		}
		val := vm.run("", nil)
		if !reflect.DeepEqual(val, c.val) {
			if f, ok := c.val.(float64); ok && math.IsNaN(f) && math.IsNaN(val.(float64)) {
				continue
			}
			t.Fatalf("expr: %q, got: %v, expect: %v", c.expr, val, c.val)
		}
	}
}

func TestBuiltInFunc(t *testing.T) {
	cases := []struct {
		expr string
		val  interface{}
	}{
		{expr: "len('abc')", val: 3.0},
		{expr: "len('abc')+2*2/len('cd')", val: 5.0},
		{expr: "len(0)", val: 0.0},

		{expr: "regexp('a\\d','a0')", val: true},
		{expr: "regexp('^a\\d$','a0')", val: true},
		{expr: "regexp('a\\d','a')", val: false},
		{expr: "regexp('^a\\d$','a')", val: false},

		{expr: "sprintf('test string: %s','a')", val: "test string: a"},
		{expr: "sprintf('test string: %s','a'+'b')", val: "test string: ab"},
		{expr: "sprintf('test string: %s,%v','a',1)", val: "test string: a,1"},
		{expr: "sprintf('')+'a'", val: "a"},
		{expr: "sprintf('%v',10+2*2)", val: "14"},
	}
	for _, c := range cases {
		t.Log(c.expr)
		vm, err := parseExpr(c.expr)
		if err != nil {
			t.Fatal(err)
		}
		val := vm.run("", nil)
		if !reflect.DeepEqual(val, c.val) {
			if f, ok := c.val.(float64); ok && math.IsNaN(f) && math.IsNaN(val.(float64)) {
				continue
			}
			t.Fatalf("expr: %q, got: %v, expect: %v", c.expr, val, c.val)
		}
	}
}

func TestSyntaxIncorrect(t *testing.T) {
	cases := []struct {
		incorrectExpr string
	}{
		{incorrectExpr: "1 + + 'a'"},
		{incorrectExpr: "regexp()"},
		{incorrectExpr: "regexp('^'+'a','a')"},
		{incorrectExpr: "regexp('^a','a','b')"},
		{incorrectExpr: "sprintf()"},
		{incorrectExpr: "sprintf(0)"},
		{incorrectExpr: "sprintf('a'+'b')"},
	}
	for _, c := range cases {
		_, err := parseExpr(c.incorrectExpr)
		if err == nil {
			t.Fatalf("expect syntax incorrect: %s", c.incorrectExpr)
		} else {
			t.Log(err)
		}
	}
}
