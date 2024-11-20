package tagexpr

import "reflect"

// FieldHandler field handler
type FieldHandler struct {
	selector string
	field    *fieldVM
	expr     *TagExpr
}

func newFieldHandler(expr *TagExpr, fieldSelector string, field *fieldVM) *FieldHandler {
	return &FieldHandler{
		selector: fieldSelector,
		field:    field,
		expr:     expr,
	}
}

// StringSelector returns the field selector of string type.
func (f *FieldHandler) StringSelector() string {
	return f.selector
}

// FieldSelector returns the field selector of FieldSelector type.
func (f *FieldHandler) FieldSelector() FieldSelector {
	return FieldSelector(f.selector)
}

// Value returns the field value.
// NOTE:
//  If initZero==true, initialize nil pointer to zero value
func (f *FieldHandler) Value(initZero bool) reflect.Value {
	return f.field.reflectValueGetter(f.expr.ptr, initZero)
}

// EvalFuncs returns the tag expression eval functions.
func (f *FieldHandler) EvalFuncs() map[ExprSelector]func() interface{} {
	targetTagExpr, _ := f.expr.checkout(f.selector)
	evals := make(map[ExprSelector]func() interface{}, len(f.field.exprs))
	for k, v := range f.field.exprs {
		expr := v
		exprSelector := ExprSelector(k)
		evals[exprSelector] = func() interface{} {
			return expr.run(exprSelector.Name(), targetTagExpr)
		}
	}
	return evals
}

// StructField returns the field StructField object.
func (f *FieldHandler) StructField() reflect.StructField {
	return f.field.structField
}

// ExprHandler expr handler
type ExprHandler struct {
	base       string
	path       string
	selector   string
	expr       *TagExpr
	targetExpr *TagExpr
}

func newExprHandler(te, tte *TagExpr, base, es string) *ExprHandler {
	return &ExprHandler{
		base:       base,
		selector:   es,
		expr:       te,
		targetExpr: tte,
	}
}

// TagExpr returns the *TagExpr.
func (e *ExprHandler) TagExpr() *TagExpr {
	return e.expr
}

// StringSelector returns the expression selector of string type.
func (e *ExprHandler) StringSelector() string {
	return e.selector
}

// ExprSelector returns the expression selector of ExprSelector type.
func (e *ExprHandler) ExprSelector() ExprSelector {
	return ExprSelector(e.selector)
}

// Path returns the path description of the expression.
func (e *ExprHandler) Path() string {
	if e.path == "" {
		if e.targetExpr.path == "" {
			e.path = e.selector
		} else {
			e.path = e.targetExpr.path + FieldSeparator + e.selector
		}
	}
	return e.path
}

// Eval evaluate the value of the struct tag expression.
// NOTE:
//  result types: float64, string, bool, nil
func (e *ExprHandler) Eval() interface{} {
	return e.expr.s.exprs[e.selector].run(e.base, e.targetExpr)
}

// EvalFloat evaluates the value of the struct tag expression.
// NOTE:
//  If the expression value type is not float64, return 0.
func (e *ExprHandler) EvalFloat() float64 {
	r, _ := e.Eval().(float64)
	return r
}

// EvalString evaluates the value of the struct tag expression.
// NOTE:
//  If the expression value type is not string, return "".
func (e *ExprHandler) EvalString() string {
	r, _ := e.Eval().(string)
	return r
}

// EvalBool evaluates the value of the struct tag expression.
// NOTE:
//  If the expression value is not 0, '' or nil, return true.
func (e *ExprHandler) EvalBool() bool {
	return FakeBool(e.Eval())
}
