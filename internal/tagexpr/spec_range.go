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

package tagexpr

import (
	"context"
	"reflect"
	"regexp"
)

type rangeCtxKey string

const (
	rangeKey   rangeCtxKey = "#k"
	rangeValue rangeCtxKey = "#v"
	rangeLen   rangeCtxKey = "##"
)

type rangeKvExprNode struct {
	exprBackground
	ctxKey       rangeCtxKey
	boolOpposite *bool
	signOpposite *bool
}

func (re *rangeKvExprNode) String() string {
	return string(re.ctxKey)
}

func (p *Expr) readRangeKvExprNode(expr *string) ExprNode {
	name, boolOpposite, signOpposite, found := findRangeKv(expr)
	if !found {
		return nil
	}
	operand := &rangeKvExprNode{
		ctxKey:       rangeCtxKey(name),
		boolOpposite: boolOpposite,
		signOpposite: signOpposite,
	}
	// fmt.Printf("operand: %#v\n", operand)
	return operand
}

var rangeKvRegexp = regexp.MustCompile(`^([\!\+\-]*)(#[kv#])([\)\[\],\+\-\*\/%><\|&!=\^ \t\\]|$)`)

func findRangeKv(expr *string) (name string, boolOpposite, signOpposite *bool, found bool) {
	raw := *expr
	a := rangeKvRegexp.FindAllStringSubmatch(raw, -1)
	if len(a) != 1 {
		return
	}
	r := a[0]
	name = r[2]
	*expr = (*expr)[len(a[0][0])-len(r[3]):]
	prefix := r[1]
	if len(prefix) == 0 {
		found = true
		return
	}
	_, boolOpposite, signOpposite = getBoolAndSignOpposite(&prefix)
	found = true
	return
}

func (re *rangeKvExprNode) Run(ctx context.Context, _ string, _ *TagExpr) interface{} {
	var v interface{}
	switch val := ctx.Value(re.ctxKey).(type) {
	case reflect.Value:
		if !val.IsValid() || !val.CanInterface() {
			return nil
		}
		v = val.Interface()
	default:
		v = val
	}
	return realValue(v, re.boolOpposite, re.signOpposite)
}

type rangeFuncExprNode struct {
	exprBackground
	object       ExprNode
	elemExprNode ExprNode
	boolOpposite *bool
	signOpposite *bool
}

func (e *rangeFuncExprNode) String() string {
	return "range()"
}

// range($, gt($v,10))
// range($, $v>10)
func readRangeFuncExprNode(p *Expr, expr *string) ExprNode {
	boolOpposite, signOpposite, args, found := p.parseFuncSign("range", expr)
	if !found {
		return nil
	}
	if len(args) != 2 {
		return nil
	}
	return &rangeFuncExprNode{
		boolOpposite: boolOpposite,
		signOpposite: signOpposite,
		object:       args[0],
		elemExprNode: args[1],
	}
}

func (e *rangeFuncExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	var r []interface{}
	obj := e.object.Run(ctx, currField, tagExpr)
	// fmt.Printf("%v\n", obj)
	objval := reflect.ValueOf(obj)
	switch objval.Kind() {
	case reflect.Array, reflect.Slice:
		count := objval.Len()
		r = make([]interface{}, count)
		ctx = context.WithValue(ctx, rangeLen, count)
		for i := 0; i < count; i++ {
			// fmt.Printf("%#v,  (%v)\n", e.elemExprNode, objval.Index(i))
			r[i] = realValue(e.elemExprNode.Run(
				context.WithValue(
					context.WithValue(
						ctx,
						rangeKey, i,
					),
					rangeValue, objval.Index(i),
				),
				currField, tagExpr,
			), e.boolOpposite, e.signOpposite)
		}
	case reflect.Map:
		keys := objval.MapKeys()
		count := len(keys)
		r = make([]interface{}, count)
		ctx = context.WithValue(ctx, rangeLen, count)
		for i, key := range keys {
			r[i] = realValue(e.elemExprNode.Run(
				context.WithValue(
					context.WithValue(
						ctx,
						rangeKey, key,
					),
					rangeValue, objval.MapIndex(key),
				),
				currField, tagExpr,
			), e.boolOpposite, e.signOpposite)
		}
	default:
	}
	return r
}
