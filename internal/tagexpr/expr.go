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
)

type variableKeyType string

const variableKey variableKeyType = "__ENV_KEY__"

// Expr expression
type Expr struct {
	expr ExprNode
}

// parseExpr parses the expression.
func parseExpr(expr string) (*Expr, error) {
	e := newGroupExprNode()
	p := &Expr{
		expr: e,
	}
	s := expr
	err := p.parseExprNode(&s, e)
	if err != nil {
		return nil, err
	}
	sortPriority(e)
	return p, nil
}

func (p *Expr) parseExprNode(expr *string, e ExprNode) error {
	trimLeftSpace(expr)
	if *expr == "" {
		return nil
	}
	operand := p.readSelectorExprNode(expr)
	if operand == nil {
		operand = p.readRangeKvExprNode(expr)
		if operand == nil {
			var subExprNode *string
			operand, subExprNode = readGroupExprNode(expr)
			if operand != nil {
				err := p.parseExprNode(subExprNode, operand)
				if err != nil {
					return err
				}
			} else {
				operand = p.parseOperand(expr)
			}
		}
	}
	if operand == nil {
		return fmt.Errorf("syntax error: %q", *expr)
	}
	trimLeftSpace(expr)
	operator := p.parseOperator(expr)
	if operator == nil {
		e.SetRightOperand(operand)
		operand.SetParent(e)
		return nil
	}
	if _, ok := e.(*groupExprNode); ok {
		operator.SetLeftOperand(operand)
		operand.SetParent(operator)
		e.SetRightOperand(operator)
		operator.SetParent(e)
	} else {
		operator.SetParent(e.Parent())
		operator.Parent().SetRightOperand(operator)
		operator.SetLeftOperand(e)
		e.SetParent(operator)
		e.SetRightOperand(operand)
		operand.SetParent(e)
	}
	return p.parseExprNode(expr, operator)
}

func (p *Expr) parseOperand(expr *string) (e ExprNode) {
	for _, fn := range funcList {
		if e = fn(p, expr); e != nil {
			return e
		}
	}
	if e = readStringExprNode(expr); e != nil {
		return e
	}
	if e = readDigitalExprNode(expr); e != nil {
		return e
	}
	if e = readBoolExprNode(expr); e != nil {
		return e
	}
	if e = readNilExprNode(expr); e != nil {
		return e
	}
	if e = readVariableExprNode(expr); e != nil {
		return e
	}
	return nil
}

func (*Expr) parseOperator(expr *string) (e ExprNode) {
	s := *expr
	if len(s) < 2 {
		return nil
	}
	defer func() {
		if e != nil && *expr == s {
			*expr = (*expr)[2:]
		}
	}()
	a := s[:2]
	switch a {
	// case "<<":
	// case ">>":
	// case "&^":
	case "||":
		return newOrExprNode()
	case "&&":
		return newAndExprNode()
	case "==":
		return newEqualExprNode()
	case ">=":
		return newGreaterEqualExprNode()
	case "<=":
		return newLessEqualExprNode()
	case "!=":
		return newNotEqualExprNode()
	}
	defer func() {
		if e != nil {
			*expr = (*expr)[1:]
		}
	}()
	switch a[0] {
	// case '&':
	// case '|':
	// case '^':
	case '+':
		return newAdditionExprNode()
	case '-':
		return newSubtractionExprNode()
	case '*':
		return newMultiplicationExprNode()
	case '/':
		return newDivisionExprNode()
	case '%':
		return newRemainderExprNode()
	case '<':
		return newLessExprNode()
	case '>':
		return newGreaterExprNode()
	}
	return nil
}

// run calculates the value of expression.
func (p *Expr) run(field string, tagExpr *TagExpr) interface{} {
	return p.expr.Run(context.Background(), field, tagExpr)
}

func (p *Expr) runWithEnv(field string, tagExpr *TagExpr, env map[string]interface{}) interface{} {
	ctx := context.WithValue(context.Background(), variableKey, env)
	return p.expr.Run(ctx, field, tagExpr)
}

/**
 * Priority:
 * () ! bool float64 string nil
 * * / %
 * + -
 * < <= > >=
 * == !=
 * &&
 * ||
**/

func sortPriority(e ExprNode) {
	for subSortPriority(e.RightOperand(), false) {
	}
}

func subSortPriority(e ExprNode, isLeft bool) bool {
	if e == nil {
		return false
	}
	leftChanged := subSortPriority(e.LeftOperand(), true)
	rightChanged := subSortPriority(e.RightOperand(), false)
	if getPriority(e) > getPriority(e.LeftOperand()) {
		leftOperandToParent(e, isLeft)
		return true
	}
	return leftChanged || rightChanged
}

func leftOperandToParent(e ExprNode, isLeft bool) {
	le := e.LeftOperand()
	if le == nil {
		return
	}
	p := e.Parent()
	le.SetParent(p)
	if p != nil {
		if isLeft {
			p.SetLeftOperand(le)
		} else {
			p.SetRightOperand(le)
		}
	}
	e.SetParent(le)
	e.SetLeftOperand(le.RightOperand())
	le.RightOperand().SetParent(e)
	le.SetRightOperand(e)
}

func getPriority(e ExprNode) (i int) {
	// defer func() {
	// 	printf("expr:%T %d\n", e, i)
	// }()
	switch e.(type) {
	default: // () ! bool float64 string nil
		return 7
	case *multiplicationExprNode, *divisionExprNode, *remainderExprNode: // * / %
		return 6
	case *additionExprNode, *subtractionExprNode: // + -
		return 5
	case *lessExprNode, *lessEqualExprNode, *greaterExprNode, *greaterEqualExprNode: // < <= > >=
		return 4
	case *equalExprNode, *notEqualExprNode: // == !=
		return 3
	case *andExprNode: // &&
		return 2
	case *orExprNode: // ||
		return 1
	}
}

// ExprNode expression interface
type ExprNode interface {
	SetParent(ExprNode)
	Parent() ExprNode
	LeftOperand() ExprNode
	RightOperand() ExprNode
	SetLeftOperand(ExprNode)
	SetRightOperand(ExprNode)
	String() string
	Run(context.Context, string, *TagExpr) interface{}
}

// var _ ExprNode = new(exprBackground)

type exprBackground struct {
	parent       ExprNode
	leftOperand  ExprNode
	rightOperand ExprNode
}

func (eb *exprBackground) SetParent(e ExprNode) {
	eb.parent = e
}

func (eb *exprBackground) Parent() ExprNode {
	return eb.parent
}

func (eb *exprBackground) LeftOperand() ExprNode {
	return eb.leftOperand
}

func (eb *exprBackground) RightOperand() ExprNode {
	return eb.rightOperand
}

func (eb *exprBackground) SetLeftOperand(left ExprNode) {
	eb.leftOperand = left
}

func (eb *exprBackground) SetRightOperand(right ExprNode) {
	eb.rightOperand = right
}

func (*exprBackground) Run(context.Context, string, *TagExpr) interface{} { return nil }
