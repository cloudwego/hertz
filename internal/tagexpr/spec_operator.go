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
	"math"
)

// --------------------------- Operator ---------------------------

type additionExprNode struct{ exprBackground }

func (ae *additionExprNode) String() string {
	return "+"
}

func newAdditionExprNode() ExprNode { return &additionExprNode{} }

func (ae *additionExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	// positive number or Addition
	v0 := ae.leftOperand.Run(ctx, currField, tagExpr)
	v1 := ae.rightOperand.Run(ctx, currField, tagExpr)
	if s0, ok := toFloat64(v0, false); ok {
		s1, _ := toFloat64(v1, true)
		return s0 + s1
	}
	if s0, ok := toString(v0, false); ok {
		s1, _ := toString(v1, true)
		return s0 + s1
	}
	return v0
}

type multiplicationExprNode struct{ exprBackground }

func (ae *multiplicationExprNode) String() string {
	return "*"
}

func newMultiplicationExprNode() ExprNode { return &multiplicationExprNode{} }

func (ae *multiplicationExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v0, _ := toFloat64(ae.leftOperand.Run(ctx, currField, tagExpr), true)
	v1, _ := toFloat64(ae.rightOperand.Run(ctx, currField, tagExpr), true)
	return v0 * v1
}

type divisionExprNode struct{ exprBackground }

func (de *divisionExprNode) String() string {
	return "/"
}

func newDivisionExprNode() ExprNode { return &divisionExprNode{} }

func (de *divisionExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v1, _ := toFloat64(de.rightOperand.Run(ctx, currField, tagExpr), true)
	if v1 == 0 {
		return math.NaN()
	}
	v0, _ := toFloat64(de.leftOperand.Run(ctx, currField, tagExpr), true)
	return v0 / v1
}

type subtractionExprNode struct{ exprBackground }

func (de *subtractionExprNode) String() string {
	return "-"
}

func newSubtractionExprNode() ExprNode { return &subtractionExprNode{} }

func (de *subtractionExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v0, _ := toFloat64(de.leftOperand.Run(ctx, currField, tagExpr), true)
	v1, _ := toFloat64(de.rightOperand.Run(ctx, currField, tagExpr), true)
	return v0 - v1
}

type remainderExprNode struct{ exprBackground }

func (re *remainderExprNode) String() string {
	return "%"
}

func newRemainderExprNode() ExprNode { return &remainderExprNode{} }

func (re *remainderExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v1, _ := toFloat64(re.rightOperand.Run(ctx, currField, tagExpr), true)
	if v1 == 0 {
		return math.NaN()
	}
	v0, _ := toFloat64(re.leftOperand.Run(ctx, currField, tagExpr), true)
	return float64(int64(v0) % int64(v1))
}

type equalExprNode struct{ exprBackground }

func (ee *equalExprNode) String() string {
	return "=="
}

func newEqualExprNode() ExprNode { return &equalExprNode{} }

func (ee *equalExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v0 := ee.leftOperand.Run(ctx, currField, tagExpr)
	v1 := ee.rightOperand.Run(ctx, currField, tagExpr)
	if v0 == v1 {
		return true
	}
	if s0, ok := toFloat64(v0, false); ok {
		if s1, ok := toFloat64(v1, true); ok {
			return s0 == s1
		}
	}
	if s0, ok := toString(v0, false); ok {
		if s1, ok := toString(v1, true); ok {
			return s0 == s1
		}
		return false
	}
	switch r := v0.(type) {
	case bool:
		r1, ok := v1.(bool)
		if ok {
			return r == r1
		}
	case nil:
		return v1 == nil
	}
	return false
}

type notEqualExprNode struct{ equalExprNode }

func (ne *notEqualExprNode) String() string {
	return "!="
}

func newNotEqualExprNode() ExprNode { return &notEqualExprNode{} }

func (ne *notEqualExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	return !ne.equalExprNode.Run(ctx, currField, tagExpr).(bool)
}

type greaterExprNode struct{ exprBackground }

func (ge *greaterExprNode) String() string {
	return ">"
}

func newGreaterExprNode() ExprNode { return &greaterExprNode{} }

func (ge *greaterExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v0 := ge.leftOperand.Run(ctx, currField, tagExpr)
	v1 := ge.rightOperand.Run(ctx, currField, tagExpr)
	if s0, ok := toFloat64(v0, false); ok {
		if s1, ok := toFloat64(v1, true); ok {
			return s0 > s1
		}
	}
	if s0, ok := toString(v0, false); ok {
		if s1, ok := toString(v1, true); ok {
			return s0 > s1
		}
		return false
	}
	return false
}

type greaterEqualExprNode struct{ exprBackground }

func (ge *greaterEqualExprNode) String() string {
	return ">="
}

func newGreaterEqualExprNode() ExprNode { return &greaterEqualExprNode{} }

func (ge *greaterEqualExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v0 := ge.leftOperand.Run(ctx, currField, tagExpr)
	v1 := ge.rightOperand.Run(ctx, currField, tagExpr)
	if s0, ok := toFloat64(v0, false); ok {
		if s1, ok := toFloat64(v1, true); ok {
			return s0 >= s1
		}
	}
	if s0, ok := toString(v0, false); ok {
		if s1, ok := toString(v1, true); ok {
			return s0 >= s1
		}
		return false
	}
	return false
}

type lessExprNode struct{ exprBackground }

func (le *lessExprNode) String() string {
	return "<"
}

func newLessExprNode() ExprNode { return &lessExprNode{} }

func (le *lessExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v0 := le.leftOperand.Run(ctx, currField, tagExpr)
	v1 := le.rightOperand.Run(ctx, currField, tagExpr)
	if s0, ok := toFloat64(v0, false); ok {
		if s1, ok := toFloat64(v1, true); ok {
			return s0 < s1
		}
	}
	if s0, ok := toString(v0, false); ok {
		if s1, ok := toString(v1, true); ok {
			return s0 < s1
		}
		return false
	}
	return false
}

type lessEqualExprNode struct{ exprBackground }

func (le *lessEqualExprNode) String() string {
	return "<="
}

func newLessEqualExprNode() ExprNode { return &lessEqualExprNode{} }

func (le *lessEqualExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	v0 := le.leftOperand.Run(ctx, currField, tagExpr)
	v1 := le.rightOperand.Run(ctx, currField, tagExpr)
	if s0, ok := toFloat64(v0, false); ok {
		if s1, ok := toFloat64(v1, true); ok {
			return s0 <= s1
		}
	}
	if s0, ok := toString(v0, false); ok {
		if s1, ok := toString(v1, true); ok {
			return s0 <= s1
		}
		return false
	}
	return false
}

type andExprNode struct{ exprBackground }

func (ae *andExprNode) String() string {
	return "&&"
}

func newAndExprNode() ExprNode { return &andExprNode{} }

func (ae *andExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	for _, e := range [2]ExprNode{ae.leftOperand, ae.rightOperand} {
		if !FakeBool(e.Run(ctx, currField, tagExpr)) {
			return false
		}
	}
	return true
}

type orExprNode struct{ exprBackground }

func (oe *orExprNode) String() string {
	return "||"
}

func newOrExprNode() ExprNode { return &orExprNode{} }

func (oe *orExprNode) Run(ctx context.Context, currField string, tagExpr *TagExpr) interface{} {
	for _, e := range [2]ExprNode{oe.leftOperand, oe.rightOperand} {
		if FakeBool(e.Run(ctx, currField, tagExpr)) {
			return true
		}
	}
	return false
}
