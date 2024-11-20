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
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// --------------------------- Custom function ---------------------------

var funcList = map[string]func(p *Expr, expr *string) ExprNode{}

// MustRegFunc registers function expression.
// NOTE:
//
//	example: len($), regexp("\\d") or regexp("\\d",$);
//	If @force=true, allow to cover the existed same @funcName;
//	The go number types always are float64;
//	The go string types always are string;
//	Panic if there is an error.
func MustRegFunc(funcName string, fn func(...interface{}) interface{}, force ...bool) {
	err := RegFunc(funcName, fn, force...)
	if err != nil {
		panic(err)
	}
}

// RegFunc registers function expression.
// NOTE:
//
//	example: len($), regexp("\\d") or regexp("\\d",$);
//	If @force=true, allow to cover the existed same @funcName;
//	The go number types always are float64;
//	The go string types always are string.
func RegFunc(funcName string, fn func(...interface{}) interface{}, force ...bool) error {
	if len(force) == 0 || !force[0] {
		_, ok := funcList[funcName]
		if ok {
			return fmt.Errorf("duplicate registration expression function: %s", funcName)
		}
	}
	funcList[funcName] = newFunc(funcName, fn)
	return nil
}

func (p *Expr) parseFuncSign(funcName string, expr *string) (boolOpposite *bool, signOpposite *bool, args []ExprNode, found bool) {
	prefix := funcName + "("
	length := len(funcName)
	last, boolOpposite, signOpposite := getBoolAndSignOpposite(expr)
	if !strings.HasPrefix(last, prefix) {
		return
	}
	*expr = last[length:]
	lastStr := *expr
	subExprNode := readPairedSymbol(expr, '(', ')')
	if subExprNode == nil {
		return
	}
	*subExprNode = "," + *subExprNode
	for {
		if strings.HasPrefix(*subExprNode, ",") {
			*subExprNode = (*subExprNode)[1:]
			operand := newGroupExprNode()
			err := p.parseExprNode(trimLeftSpace(subExprNode), operand)
			if err != nil {
				*expr = lastStr
				return
			}
			sortPriority(operand)
			args = append(args, operand)
		} else {
			*expr = lastStr
			return
		}
		trimLeftSpace(subExprNode)
		if len(*subExprNode) == 0 {
			found = true
			return
		}
	}
}

func newFunc(funcName string, fn func(...interface{}) interface{}) func(*Expr, *string) ExprNode {
	return func(p *Expr, expr *string) ExprNode {
		boolOpposite, signOpposite, args, found := p.parseFuncSign(funcName, expr)
		if !found {
			return nil
		}
		return &funcExprNode{
			fn:           fn,
			boolOpposite: boolOpposite,
			signOpposite: signOpposite,
			args:         args,
		}
	}
}

type funcExprNode struct {
	exprBackground
	args         []ExprNode
	fn           func(...interface{}) interface{}
	boolOpposite *bool
	signOpposite *bool
}

func (f *funcExprNode) String() string {
	return "func()"
}

func (f *funcExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	var args []interface{}
	if n := len(f.args); n > 0 {
		args = make([]interface{}, n)
		for k, v := range f.args {
			args[k] = v.Run(ctx, currField, tagExpr)
		}
	}
	return realValue(f.fn(args...), f.boolOpposite, f.signOpposite)
}

// --------------------------- Built-in function ---------------------------
func init() {
	funcList["regexp"] = readRegexpFuncExprNode
	funcList["sprintf"] = readSprintfFuncExprNode
	funcList["range"] = readRangeFuncExprNode
	// len: Built-in function len, the length of struct field X
	MustRegFunc("len", func(args ...interface{}) (n interface{}) {
		if len(args) != 1 {
			return 0
		}
		v := args[0]
		switch e := v.(type) {
		case string:
			return float64(len(e))
		case float64, bool, nil:
			return 0
		}
		defer func() {
			if recover() != nil {
				n = 0
			}
		}()
		return float64(reflect.ValueOf(v).Len())
	}, true)
	// mblen: get the length of string field X (character number)
	MustRegFunc("mblen", func(args ...interface{}) (n interface{}) {
		if len(args) != 1 {
			return 0
		}
		v := args[0]
		switch e := v.(type) {
		case string:
			return float64(len([]rune(e)))
		case float64, bool, nil:
			return 0
		}
		defer func() {
			if recover() != nil {
				n = 0
			}
		}()
		return float64(reflect.ValueOf(v).Len())
	}, true)

	// in: Check if the first parameter is one of the enumerated parameters
	MustRegFunc("in", func(args ...interface{}) interface{} {
		switch len(args) {
		case 0:
			return true
		case 1:
			return false
		default:
			elem := args[0]
			set := args[1:]
			for _, e := range set {
				if elem == e {
					return true
				}
			}
			return false
		}
	}, true)
}

type regexpFuncExprNode struct {
	exprBackground
	re           *regexp.Regexp
	boolOpposite bool
}

func (re *regexpFuncExprNode) String() string {
	return "regexp()"
}

func readRegexpFuncExprNode(p *Expr, expr *string) ExprNode {
	last, boolOpposite, _ := getBoolAndSignOpposite(expr)
	if !strings.HasPrefix(last, "regexp(") {
		return nil
	}
	*expr = last[6:]
	lastStr := *expr
	subExprNode := readPairedSymbol(expr, '(', ')')
	if subExprNode == nil {
		return nil
	}
	s := readPairedSymbol(trimLeftSpace(subExprNode), '\'', '\'')
	if s == nil {
		*expr = lastStr
		return nil
	}
	rege, err := regexp.Compile(*s)
	if err != nil {
		*expr = lastStr
		return nil
	}
	operand := newGroupExprNode()
	trimLeftSpace(subExprNode)
	if strings.HasPrefix(*subExprNode, ",") {
		*subExprNode = (*subExprNode)[1:]
		err = p.parseExprNode(trimLeftSpace(subExprNode), operand)
		if err != nil {
			*expr = lastStr
			return nil
		}
	} else {
		currFieldVal := "$"
		p.parseExprNode(&currFieldVal, operand)
	}
	trimLeftSpace(subExprNode)
	if *subExprNode != "" {
		*expr = lastStr
		return nil
	}
	e := &regexpFuncExprNode{
		re: rege,
	}
	if boolOpposite != nil {
		e.boolOpposite = *boolOpposite
	}
	e.SetRightOperand(operand)
	return e
}

func (re *regexpFuncExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	param := re.rightOperand.Run(ctx, currField, tagExpr)
	switch v := param.(type) {
	case string:
		bol := re.re.MatchString(v)
		if re.boolOpposite {
			return !bol
		}
		return bol
	case float64, bool:
		return false
	}
	v := reflect.ValueOf(param)
	if v.Kind() == reflect.String {
		bol := re.re.MatchString(v.String())
		if re.boolOpposite {
			return !bol
		}
		return bol
	}
	return false
}

type sprintfFuncExprNode struct {
	exprBackground
	format string
	args   []ExprNode
}

func (se *sprintfFuncExprNode) String() string {
	return "sprintf()"
}

func readSprintfFuncExprNode(p *Expr, expr *string) ExprNode {
	if !strings.HasPrefix(*expr, "sprintf(") {
		return nil
	}
	*expr = (*expr)[7:]
	lastStr := *expr
	subExprNode := readPairedSymbol(expr, '(', ')')
	if subExprNode == nil {
		return nil
	}
	format := readPairedSymbol(trimLeftSpace(subExprNode), '\'', '\'')
	if format == nil {
		*expr = lastStr
		return nil
	}
	e := &sprintfFuncExprNode{
		format: *format,
	}
	for {
		trimLeftSpace(subExprNode)
		if len(*subExprNode) == 0 {
			return e
		}
		if strings.HasPrefix(*subExprNode, ",") {
			*subExprNode = (*subExprNode)[1:]
			operand := newGroupExprNode()
			err := p.parseExprNode(trimLeftSpace(subExprNode), operand)
			if err != nil {
				*expr = lastStr
				return nil
			}
			sortPriority(operand)
			e.args = append(e.args, operand)
		} else {
			*expr = lastStr
			return nil
		}
	}
}

func (se *sprintfFuncExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	var args []interface{}
	if n := len(se.args); n > 0 {
		args = make([]interface{}, n)
		for i, e := range se.args {
			args[i] = e.Run(ctx, currField, tagExpr)
		}
	}
	return fmt.Sprintf(se.format, args...)
}
