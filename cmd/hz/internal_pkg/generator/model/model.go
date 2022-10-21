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

type Model struct {
	FilePath string
	Package  string
	Imports  map[string]*Model //{{import}}:Model

	// rendering data
	PackageName string
	// Imports     map[string]string //{{alias}}:{{import}}
	Typedefs  []TypeDef
	Constants []Constant
	Variables []Variable
	Functions []Function
	Enums     []Enum
	Structs   []Struct
	Methods   []Method
	Oneofs    []Oneof
}

func (m Model) IsEmpty() bool {
	return len(m.Typedefs) == 0 && len(m.Constants) == 0 && len(m.Variables) == 0 &&
		len(m.Functions) == 0 && len(m.Enums) == 0 && len(m.Structs) == 0 && len(m.Methods) == 0
}

type Models []*Model

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

type RequiredNess int

const (
	RequiredNess_Default  RequiredNess = 0
	RequiredNess_Required RequiredNess = 1
	RequiredNess_Optional RequiredNess = 2
)

type Type struct {
	Name     string
	Scope    *Model
	Kind     Kind
	Indirect bool
	Category Category
	Extra    []*Type // [{key_type},{value_type}] for map, [{element_type}] for list or set
	HasNew   bool
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
	IsSetDefault     bool
	DefaultValue     Literal
	Required         RequiredNess
	Tags             Tags
	LeadingComments  string
	TrailingComments string
	IsPointer        bool
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
	Key   string
	Value string
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
