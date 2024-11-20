# go-tagexpr [![report card](https://goreportcard.com/badge/github.com/bytedance/go-tagexpr?style=flat-square)](http://goreportcard.com/report/bytedance/go-tagexpr) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/bytedance/go-tagexpr)

An interesting go struct tag expression syntax for field validation, etc.

## Usage

- **[Validator](https://github.com/bytedance/go-tagexpr/tree/master/validator)**: A powerful validator that supports struct tag expression

- **[Binding](https://github.com/bytedance/go-tagexpr/tree/master/binding)**: A powerful HTTP request parameters binder that supports struct tag expression

## Feature

- Support for a variety of common operator
- Support for accessing arrays, slices, members of the dictionary
- Support access to any field in the current structure
- Support access to nested fields, non-exported fields, etc.
- Support variable
- Support registers function expression
- Built-in len, sprintf, regexp functions
- Support single mode and multiple mode to define expression
- Parameter check subpackage
- Use offset pointers to directly take values, better performance
- Required go version ≥1.9

## Example

```go
package tagexpr_test

import (
	"fmt"

	tagexpr "github.com/bytedance/go-tagexpr/v2"
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
		h  int 				`tagexpr:"$>minVal"`
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
```

## Syntax

Struct tag syntax spec:

```
type T struct {
	// Single model
    Field1 T1 `tagName:"expression"`
	// Multiple model
    Field2 T2 `tagName:"exprName:expression; [exprName2:expression2;]..."`
	// Omit it
    Field3 T3 `tagName:"-"`
    // Omit it when it is nil
    Field4 T4 `tagName:"?"`
    ...
}
```

NOTE: **The `exprName` under the same struct field cannot be the same！**

|Operator or Operand|Explain|
|-----|---------|
|`true` `false`|boolean|
|`0` `0.0`|float64 "0"|
|`''`|String|
|`\\'`| Escape `'` delims in string|
|`\"`| Escape `"` delims in string|
|`nil`|nil, undefined|
|`!`|not|
|`+`|Digital addition or string splicing|
|`-`|Digital subtraction or negative|
|`*`|Digital multiplication|
|`/`|Digital division|
|`%`|division remainder, as: `float64(int64(a)%int64(b))`|
|`==`|`eq`|
|`!=`|`ne`|
|`>`|`gt`|
|`>=`|`ge`|
|`<`|`lt`|
|`<=`|`le`|
|`&&`|Logic `and`|
|`\|\|`|Logic `or`|
|`()`|Expression group|
|`(X)$`|Struct field value named X|
|`(X.Y)$`|Struct field value named X.Y|
|`$`|Shorthand for `(X)$`, omit `(X)` to indicate current struct field value|
|`(X)$['A']`|Map value with key A or struct A sub-field in the struct field X|
|`(X)$[0]`|The 0th element or sub-field of the struct field X(type: map, slice, array, struct)|
|`len((X)$)`|Built-in function `len`, the length of struct field X|
|`mblen((X)$)`|the length of string field X (character number)|
|`regexp('^\\w*$', (X)$)`|Regular match the struct field X, return boolean|
|`regexp('^\\w*$')`|Regular match the current struct field, return boolean|
|`sprintf('X value: %v', (X)$)`|`fmt.Sprintf`, format the value of struct field X|
|`range(KvExpr, forEachExpr)`|Iterate over an array, slice, or dictionary <br> - `#k` is the element key var <br> - `#v` is the element value var <br> - `##` is the number of elements <br> - e.g. [example](spec_range_test.go)|
|`in((X)$, enum_1, ...enum_n)`|Check if the first parameter is one of the enumerated parameters|

<!-- |`(X)$k`|Traverse each element key of the struct field X(type: map, slice, array)|
|`(X)$v`|Traverse each element value of the struct field X(type: map, slice, array)| -->

<!-- |`&`|Integer bitwise `and`|
|`\|`|Integer bitwise `or`|
|`^`|Integer bitwise `not` or `xor`|
|`&^`|Integer bitwise `clean`|
|`<<`|Integer bitwise `shift left`|
|`>>`|Integer bitwise `shift right`| -->

Operator priority(high -> low):

* `()` `!` `bool` `float64` `string` `nil`
* `*` `/` `%`
* `+` `-`
* `<` `<=` `>` `>=`
* `==` `!=`
* `&&`
* `||`

## Field Selector

```
field_lv1.field_lv2...field_lvn
```

## Expression Selector

- If expression is **single model** or exprName is `@`:

```
field_lv1.field_lv2...field_lvn
```

- If expression is **multiple model** and exprName is not `@`:

```
field_lv1.field_lv2...field_lvn@exprName
```

## Benchmark

```
goos: darwin
goarch: amd64
pkg: github.com/bytedance/go-tagexpr
BenchmarkTagExpr-4   	10000000	       148 ns/op	      32 B/op	       3 allocs/op
BenchmarkReflect-4   	10000000	       182 ns/op	      16 B/op	       2 allocs/op
PASS
```

[Go to test code](https://github.com/bytedance/go-tagexpr/blob/master/tagexpr_test.go#L9-L56)
