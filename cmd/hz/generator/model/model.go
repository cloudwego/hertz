/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import (
	"errors"
	"fmt"
	"strings"
)

// Kind mirrors reflect.Kind to represent Go type kinds for code generation.
type Kind uint

const (
	KindInvalid Kind = iota
	KindBool
	KindInt
	KindInt8
	KindInt16
	KindInt32
	KindInt64
	KindUint
	KindUint8
	KindUint16
	KindUint32
	KindUint64
	KindUintptr
	KindFloat32
	KindFloat64
	KindComplex64
	KindComplex128
	KindArray
	KindChan
	KindFunc
	KindInterface
	KindMap
	KindPtr
	KindSlice
	KindString
	KindStruct
	KindUnsafePointer
)

// Category represents the IDL-level semantic type category (Thrift/Protobuf).
// Values align with Thrift type IDs where applicable.
type Category int64

const (
	CategoryConstant  Category = 1
	CategoryBinary    Category = 8
	CategoryMap       Category = 9
	CategoryList      Category = 10
	CategorySet       Category = 11
	CategoryEnum      Category = 12
	CategoryStruct    Category = 13
	CategoryUnion     Category = 14
	CategoryException Category = 15
	CategoryTypedef   Category = 16
	CategoryService   Category = 17
)

// Model represents a single IDL file translated into Go code generation data.
// Each Model corresponds to one output .go file.
type Model struct {
	FilePath string            // output file path (set during generation)
	Package  string            // Go import path for this model
	Imports  map[string]*Model // imported models, keyed by import path

	PackageName string // short package name (last segment of Package)
	Typedefs    []TypeDef
	Constants   []Constant
	Variables   []Variable
	Functions   []Function
	Enums       []Enum
	Structs     []Struct
	Methods     []Method
	Oneofs      []Oneof // protobuf oneof fields
}

func (m Model) IsEmpty() bool {
	return len(m.Typedefs) == 0 && len(m.Constants) == 0 && len(m.Variables) == 0 &&
		len(m.Functions) == 0 && len(m.Enums) == 0 && len(m.Structs) == 0 && len(m.Methods) == 0
}

type Models []*Model

// MergeMap appends models from b that are not already in a (by pointer identity).
func (a *Models) MergeMap(b map[string]*Model) {
	for _, v := range b {
		insert := true
		for _, p := range *a {
			if p == v {
				insert = false
			}
		}
		if insert {
			*a = append(*a, v)
		}
	}
	return
}

func (a *Models) MergeArray(b []*Model) {
	for _, v := range b {
		insert := true
		for _, p := range *a {
			if p == v {
				insert = false
			}
		}
		if insert {
			*a = append(*a, v)
		}
	}
	return
}

// RequiredNess maps to Thrift field requiredness semantics.
type RequiredNess int

const (
	RequiredNess_Default  RequiredNess = 0 // Thrift "default" requiredness
	RequiredNess_Required RequiredNess = 1
	RequiredNess_Optional RequiredNess = 2
)

// Type describes a Go type for code generation, bridging IDL types to Go types.
type Type struct {
	Name     string   // Go type name (e.g. "int64", "MyStruct")
	Scope    *Model   // owning model; nil means unresolved, &BaseModel means builtin
	Kind     Kind     // Go type kind
	Indirect bool     // whether this type is referenced indirectly (pointer)
	Category Category // IDL semantic category
	Extra    []*Type  // container element types: [elem] for list/set, [key, value] for map
	HasNew   bool     // whether this type needs a constructor (e.g. structs, enums)
}

func (rt *Type) ResolveDefaultValue() string {
	if rt == nil {
		return ""
	}
	switch rt.Kind {
	case KindInt, KindInt8, KindInt16, KindInt32, KindInt64, KindUint, KindUint16, KindUint32, KindUint64,
		KindFloat32, KindFloat64, KindComplex64, KindComplex128:
		return "0"
	case KindBool:
		return "false"
	case KindString:
		return "\"\""
	default:
		return "nil"
	}
}

// ResolveNameForTypedef returns the fully qualified Go type name for a typedef alias.
// Unlike ResolveName, it does not add pointer prefix for struct types since typedefs
// define new named types rather than references.
func (rt *Type) ResolveNameForTypedef(scope *Model) (string, error) {
	if rt == nil {
		return "", errors.New("type is nil")
	}
	name := rt.Name
	if rt.Scope == nil {
		return rt.Name, nil
	}

	switch rt.Kind {
	case KindArray, KindSlice:
		if len(rt.Extra) != 1 {
			return "", fmt.Errorf("the type: %s should have 1 extra type, but has %d", rt.Name, len(rt.Extra))
		}
		resolveName, err := rt.Extra[0].ResolveName(scope)
		if err != nil {
			return "", err
		}
		name = fmt.Sprintf("[]%s", resolveName)
	case KindMap:
		if len(rt.Extra) != 2 {
			return "", fmt.Errorf("the type: %s should have 2 extra types, but has %d", rt.Name, len(rt.Extra))
		}
		resolveKey, err := rt.Extra[0].ResolveName(scope)
		if err != nil {
			return "", err
		}
		resolveValue, err := rt.Extra[1].ResolveName(scope)
		if err != nil {
			return "", err
		}
		name = fmt.Sprintf("map[%s]%s", resolveKey, resolveValue)
	case KindChan:
		if len(rt.Extra) != 1 {
			return "", fmt.Errorf("the type: %s should have 1 extra type, but has %d", rt.Name, len(rt.Extra))
		}
		resolveName, err := rt.Extra[0].ResolveName(scope)
		if err != nil {
			return "", err
		}
		name = fmt.Sprintf("chan %s", resolveName)
	}

	if scope != nil && rt.Scope != &BaseModel && rt.Scope.Package != scope.Package {
		name = rt.Scope.PackageName + "." + name
	}
	return name, nil
}

// ResolveName returns the fully qualified Go type name relative to scope.
// For structs, it prepends "*" (pointer). For types from different packages,
// it prepends the package name qualifier.
func (rt *Type) ResolveName(scope *Model) (string, error) {
	if rt == nil {
		return "", fmt.Errorf("type is nil")
	}
	name := rt.Name
	if rt.Scope == nil {
		if rt.Kind == KindStruct {
			return "*" + rt.Name, nil
		}
		return rt.Name, nil
	}

	if rt.Category == CategoryTypedef {
		if scope != nil && rt.Scope != &BaseModel && rt.Scope.Package != scope.Package {
			name = rt.Scope.PackageName + "." + name
		}

		if rt.Kind == KindStruct {
			name = "*" + name
		}

		return name, nil
	}

	switch rt.Kind {
	case KindArray, KindSlice:
		if len(rt.Extra) != 1 {
			return "", fmt.Errorf("The type: %s should have 1 extra type, but has %d", rt.Name, len(rt.Extra))
		}
		resolveName, err := rt.Extra[0].ResolveName(scope)
		if err != nil {
			return "", err
		}
		name = fmt.Sprintf("[]%s", resolveName)
	case KindMap:
		if len(rt.Extra) != 2 {
			return "", fmt.Errorf("The type: %s should have 2 extra type, but has %d", rt.Name, len(rt.Extra))
		}
		resolveKey, err := rt.Extra[0].ResolveName(scope)
		if err != nil {
			return "", err
		}
		resolveValue, err := rt.Extra[1].ResolveName(scope)
		if err != nil {
			return "", err
		}
		name = fmt.Sprintf("map[%s]%s", resolveKey, resolveValue)
	case KindChan:
		if len(rt.Extra) != 1 {
			return "", fmt.Errorf("The type: %s should have 1 extra type, but has %d", rt.Name, len(rt.Extra))
		}
		resolveName, err := rt.Extra[0].ResolveName(scope)
		if err != nil {
			return "", err
		}
		name = fmt.Sprintf("chan %s", resolveName)
	}

	if scope != nil && rt.Scope != &BaseModel && rt.Scope.Package != scope.Package {
		name = rt.Scope.PackageName + "." + name
	}

	if rt.Kind == KindStruct {
		name = "*" + name
	}
	return name, nil
}

func (rt *Type) IsBinary() bool {
	return rt.Category == CategoryBinary && (rt.Kind == KindSlice || rt.Kind == KindArray)
}

func (rt *Type) IsBaseType() bool {
	return rt.Kind < KindComplex64
}

// IsSettable returns true if the type is a reference type that can be nil-checked.
func (rt *Type) IsSettable() bool {
	switch rt.Kind {
	case KindArray, KindChan, KindFunc, KindInterface, KindMap, KindPtr, KindSlice, KindUnsafePointer:
		return true
	}
	return false
}

type TypeDef struct {
	Scope *Model
	Alias string
	Type  *Type
}

type Constant struct {
	Scope *Model
	Name  string
	Type  *Type
	Value Literal
}

type Literal interface {
	Expression() string
}

type Variable struct {
	Scope *Model
	Name  string
	Type  *Type
	Value Literal
}

type Function struct {
	Scope *Model
	Name  string
	Args  []Variable
	Rets  []Variable
	Code  string
}

type Method struct {
	Scope        *Model
	ReceiverName string
	ReceiverType *Type
	ByPtr        bool
	Function
}

type Enum struct {
	Scope  *Model
	Name   string
	GoType string
	Values []Constant
}

type Struct struct {
	Scope           *Model
	Name            string
	Fields          []Field
	Category        Category
	LeadingComments string
}

type Field struct {
	Scope            *Struct
	Name             string
	Type             *Type
	IsSetDefault     bool         // whether a default value is explicitly set in IDL
	DefaultValue     Literal      // the default value expression
	Required         RequiredNess // Thrift requiredness
	Tags             Tags         // struct field tags (json, query, form, etc.)
	LeadingComments  string
	TrailingComments string
	IsPointer        bool // whether to generate as pointer type (for optional fields)
}

type Oneof struct {
	MessageName   string
	OneofName     string
	InterfaceName string
	Choices       []Choice
}

type Choice struct {
	MessageName string
	ChoiceName  string
	Type        *Type
}

type Tags []Tag

type Tag struct {
	Key       string
	Value     string
	IsDefault bool // true if auto-generated (not explicitly set in IDL annotation)
}

func (ts Tags) String() string {
	ret := make([]string, 0, len(ts))
	for _, t := range ts {
		ret = append(ret, fmt.Sprintf("%v:%q", t.Key, t.Value))
	}
	return strings.Join(ret, " ")
}

func (ts *Tags) Remove(name string) {
	ret := make([]Tag, 0, len(*ts))
	for _, t := range *ts {
		if t.Key != name {
			ret = append(ret, t)
		}
	}
	*ts = ret
}

func (ts Tags) Len() int { return len(ts) }

func (ts Tags) Less(i, j int) bool {
	return ts[i].Key < ts[j].Key
}

func (ts Tags) Swap(i, j int) { ts[i], ts[j] = ts[j], ts[i] }

func (f Field) GenGoTags() string {
	if len(f.Tags) == 0 {
		return ""
	}

	return fmt.Sprintf("`%s`", f.Tags.String())
}
