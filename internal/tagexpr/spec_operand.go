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
	"strconv"
	"strings"
)

// --------------------------- Operand ---------------------------

type groupExprNode struct {
	exprBackground
	boolOpposite *bool
	signOpposite *bool
}

func newGroupExprNode() ExprNode { return &groupExprNode{} }

func readGroupExprNode(expr *string) (grp ExprNode, subExprNode *string) {
	last, boolOpposite, signOpposite := getBoolAndSignOpposite(expr)
	sptr := readPairedSymbol(&last, '(', ')')
	if sptr == nil {
		return nil, nil
	}
	*expr = last
	e := &groupExprNode{boolOpposite: boolOpposite, signOpposite: signOpposite}
	return e, sptr
}

func (ge *groupExprNode) String() string {
	return "()"
}

func (ge *groupExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	if ge.rightOperand == nil {
		return nil
	}
	return realValue(ge.rightOperand.Run(ctx, currField, tagExpr), ge.boolOpposite, ge.signOpposite)
}

type boolExprNode struct {
	exprBackground
	val bool
}

func (be *boolExprNode) String() string {
	return fmt.Sprintf("%v", be.val)
}

var boolRegexp = regexp.MustCompile(`^!*(true|false)([\)\],\|&!= \t]{1}|$)`)

func readBoolExprNode(expr *string) ExprNode {
	s := boolRegexp.FindString(*expr)
	if s == "" {
		return nil
	}
	last := s[len(s)-1]
	if last != 'e' {
		s = s[:len(s)-1]
	}
	*expr = (*expr)[len(s):]
	e := &boolExprNode{}
	if strings.Contains(s, "t") {
		e.val = (len(s)-4)&1 == 0
	} else {
		e.val = (len(s)-5)&1 == 1
	}
	return e
}

func (be *boolExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	return be.val
}

type stringExprNode struct {
	exprBackground
	val interface{}
}

func (se *stringExprNode) String() string {
	return fmt.Sprintf("%v", se.val)
}

func readStringExprNode(expr *string) ExprNode {
	last, boolOpposite, _ := getBoolAndSignOpposite(expr)
	sptr := readPairedSymbol(&last, '\'', '\'')
	if sptr == nil {
		return nil
	}
	*expr = last
	e := &stringExprNode{val: realValue(*sptr, boolOpposite, nil)}
	return e
}

func (se *stringExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	return se.val
}

type digitalExprNode struct {
	exprBackground
	val interface{}
}

func (de *digitalExprNode) String() string {
	return fmt.Sprintf("%v", de.val)
}

var digitalRegexp = regexp.MustCompile(`^[\+\-]?\d+(\.\d+)?([\)\],\+\-\*\/%><\|&!=\^ \t\\]|$)`)

func readDigitalExprNode(expr *string) ExprNode {
	last, boolOpposite := getOpposite(expr, "!")
	s := digitalRegexp.FindString(last)
	if s == "" {
		return nil
	}
	if r := s[len(s)-1]; r < '0' || r > '9' {
		s = s[:len(s)-1]
	}
	*expr = last[len(s):]
	f64, _ := strconv.ParseFloat(s, 64)
	return &digitalExprNode{val: realValue(f64, boolOpposite, nil)}
}

func (de *digitalExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	return de.val
}

type nilExprNode struct {
	exprBackground
	val interface{}
}

func (ne *nilExprNode) String() string {
	return "<nil>"
}

var nilRegexp = regexp.MustCompile(`^nil([\)\],\|&!= \t]{1}|$)`)

func readNilExprNode(expr *string) ExprNode {
	last, boolOpposite := getOpposite(expr, "!")
	s := nilRegexp.FindString(last)
	if s == "" {
		return nil
	}
	*expr = last[3:]
	return &nilExprNode{val: realValue(nil, boolOpposite, nil)}
}

func (ne *nilExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	return ne.val
}

type variableExprNode struct {
	exprBackground
	boolOpposite *bool
	val          string
}

func (ve *variableExprNode) String() string {
	return fmt.Sprintf("%v", ve.val)
}

func (ve *variableExprNode) Run(ctx context.Context, variableName string, _ *TagExpr) interface{} {
	envObj := ctx.Value(variableKey)
	if envObj == nil {
		return nil
	}

	env := envObj.(map[string]interface{})
	if len(env) == 0 {
		return nil
	}

	if value, ok := env[ve.val]; ok && value != nil {
		return realValue(value, ve.boolOpposite, nil)
	} else {
		return nil
	}
}

var variableRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*`)

func readVariableExprNode(expr *string) ExprNode {
	last, boolOpposite := getOpposite(expr, "!")
	variable := variableRegex.FindString(last)
	if variable == "" {
		return nil
	}

	*expr = (*expr)[len(*expr)-len(last)+len(variable):]

	return &variableExprNode{
		val:          variable,
		boolOpposite: boolOpposite,
	}
}

func getBoolAndSignOpposite(expr *string) (last string, boolOpposite *bool, signOpposite *bool) {
	last, boolOpposite = getOpposite(expr, "!")
	last = strings.TrimLeft(last, "+")
	last, signOpposite = getOpposite(&last, "-")
	last = strings.TrimLeft(last, "+")
	return
}

func getOpposite(expr *string, cutset string) (string, *bool) {
	last := strings.TrimLeft(*expr, cutset)
	n := len(*expr) - len(last)
	if n == 0 {
		return last, nil
	}
	bol := n&1 == 1
	return last, &bol
}

func toString(i interface{}, enforce bool) (string, bool) {
	switch vv := i.(type) {
	case string:
		return vv, true
	case nil:
		return "", false
	default:
		rv := dereferenceValue(reflect.ValueOf(i))
		if rv.Kind() == reflect.String {
			return rv.String(), true
		}
		if enforce {
			if rv.IsValid() && rv.CanInterface() {
				return fmt.Sprint(rv.Interface()), true
			} else {
				return fmt.Sprint(i), true
			}
		}
	}
	return "", false
}

func toFloat64(i interface{}, tryParse bool) (float64, bool) {
	var v float64
	ok := true
	switch t := i.(type) {
	case float64:
		v = t
	case float32:
		v = float64(t)
	case int:
		v = float64(t)
	case int8:
		v = float64(t)
	case int16:
		v = float64(t)
	case int32:
		v = float64(t)
	case int64:
		v = float64(t)
	case uint:
		v = float64(t)
	case uint8:
		v = float64(t)
	case uint16:
		v = float64(t)
	case uint32:
		v = float64(t)
	case uint64:
		v = float64(t)
	case nil:
		ok = false
	default:
		rv := dereferenceValue(reflect.ValueOf(t))
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v = float64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v = float64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			v = rv.Float()
		default:
			if tryParse {
				if s, ok := toString(i, false); ok {
					var err error
					v, err = strconv.ParseFloat(s, 64)
					return v, err == nil
				}
			}
			ok = false
		}
	}
	return v, ok
}

func realValue(v interface{}, boolOpposite *bool, signOpposite *bool) interface{} {
	if boolOpposite != nil {
		bol := FakeBool(v)
		if *boolOpposite {
			return !bol
		}
		return bol
	}
	switch t := v.(type) {
	case float64, string:
	case float32:
		v = float64(t)
	case int:
		v = float64(t)
	case int8:
		v = float64(t)
	case int16:
		v = float64(t)
	case int32:
		v = float64(t)
	case int64:
		v = float64(t)
	case uint:
		v = float64(t)
	case uint8:
		v = float64(t)
	case uint16:
		v = float64(t)
	case uint32:
		v = float64(t)
	case uint64:
		v = float64(t)
	case []interface{}:
		for k, v := range t {
			t[k] = realValue(v, boolOpposite, signOpposite)
		}
	default:
		rv := dereferenceValue(reflect.ValueOf(v))
		switch rv.Kind() {
		case reflect.String:
			v = rv.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v = float64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v = float64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			v = rv.Float()
		}
	}
	if signOpposite != nil && *signOpposite {
		if f, ok := v.(float64); ok {
			v = -f
		}
	}
	return v
}
