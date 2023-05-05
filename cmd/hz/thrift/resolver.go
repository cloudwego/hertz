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

package thrift

import (
	"fmt"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/thriftgo/parser"
)

var (
	ConstTrue = Symbol{
		IsValue: true,
		Type:    model.TypeBool,
		Value:   true,
		Scope:   &BaseThrift,
	}
	ConstFalse = Symbol{
		IsValue: true,
		Type:    model.TypeBool,
		Value:   false,
		Scope:   &BaseThrift,
	}
	ConstEmptyString = Symbol{
		IsValue: true,
		Type:    model.TypeString,
		Value:   "",
		Scope:   &BaseThrift,
	}
)

type PackageReference struct {
	IncludeBase string
	IncludePath string
	Model       *model.Model
	Ast         *parser.Thrift
	Referred    bool
}

func getReferPkgMap(pkgMap map[string]string, incs []*parser.Include, mainModel *model.Model) (map[string]*PackageReference, error) {
	var err error
	out := make(map[string]*PackageReference, len(pkgMap))
	pkgAliasMap := make(map[string]string, len(incs))
	// bugfix: add main package to avoid namespace conflict
	mainPkg := mainModel.Package
	mainPkgName := mainModel.PackageName
	mainPkgName, err = util.GetPackageUniqueName(mainPkgName)
	if err != nil {
		return nil, err
	}
	pkgAliasMap[mainPkg] = mainPkgName
	for _, inc := range incs {
		pkg := getGoPackage(inc.Reference, pkgMap)
		impt := inc.GetPath()
		base := util.BaseNameAndTrim(impt)
		pkgName := util.SplitPackageName(pkg, "")
		if pn, exist := pkgAliasMap[pkg]; exist {
			pkgName = pn
		} else {
			pkgName, err = util.GetPackageUniqueName(pkgName)
			pkgAliasMap[pkg] = pkgName
			if err != nil {
				return nil, fmt.Errorf("get package unique name failed, err: %v", err)
			}
		}
		out[base] = &PackageReference{base, impt, &model.Model{
			FilePath:    inc.Path,
			Package:     pkg,
			PackageName: pkgName,
		}, inc.Reference, false}
	}

	return out, nil
}

type Symbol struct {
	IsValue bool
	Type    *model.Type
	Value   interface{}
	Scope   *parser.Thrift
}

type NameSpace map[string]*Symbol

type Resolver struct {
	// idl symbols
	root NameSpace
	deps map[string]NameSpace

	// exported models
	mainPkg PackageReference
	refPkgs map[string]*PackageReference
}

func NewResolver(ast *parser.Thrift, model *model.Model, pkgMap map[string]string) (*Resolver, error) {
	pm, err := getReferPkgMap(pkgMap, ast.GetIncludes(), model)
	if err != nil {
		return nil, fmt.Errorf("get package map failed, err: %v", err)
	}
	file := ast.GetFilename()
	return &Resolver{
		root:    make(NameSpace),
		deps:    make(map[string]NameSpace),
		refPkgs: pm,
		mainPkg: PackageReference{
			IncludeBase: util.BaseNameAndTrim(file),
			IncludePath: ast.GetFilename(),
			Model:       model,
			Ast:         ast,
			Referred:    false,
		},
	}, nil
}

func (resolver *Resolver) GetRefModel(includeBase string) (*model.Model, error) {
	if includeBase == "" {
		return resolver.mainPkg.Model, nil
	}
	ref, ok := resolver.refPkgs[includeBase]
	if !ok {
		return nil, fmt.Errorf("not found include %s", includeBase)
	}
	return ref.Model, nil
}

func (resolver *Resolver) getBaseType(typ *parser.Type) (*model.Type, bool) {
	tt := switchBaseType(typ)
	if tt != nil {
		return tt, true
	}
	if typ.Name == "map" {
		t := *model.TypeBaseMap
		return &t, false
	}
	if typ.Name == "list" {
		t := *model.TypeBaseList
		return &t, false
	}
	if typ.Name == "set" {
		t := *model.TypeBaseList
		return &t, false
	}
	return nil, false
}

func (resolver *Resolver) ResolveType(typ *parser.Type) (*model.Type, error) {
	bt, base := resolver.getBaseType(typ)
	if bt != nil {
		if base {
			return bt, nil
		} else {
			if typ.Name == model.TypeBaseMap.Name {
				resolveKey, err := resolver.ResolveType(typ.KeyType)
				if err != nil {
					return nil, err
				}
				resolveValue, err := resolver.ResolveType(typ.ValueType)
				if err != nil {
					return nil, err
				}
				bt.Extra = append(bt.Extra, resolveKey, resolveValue)
			} else if typ.Name == model.TypeBaseList.Name || typ.Name == model.TypeBaseSet.Name {
				resolveValue, err := resolver.ResolveType(typ.ValueType)
				if err != nil {
					return nil, err
				}
				bt.Extra = append(bt.Extra, resolveValue)
			} else {
				return nil, fmt.Errorf("invalid DefinitionType(%+v)", bt)
			}
			return bt, nil
		}
	}

	id := typ.GetName()
	rs, err := resolver.ResolveIdentifier(id)
	if err != nil {
		return nil, err
	}
	sb := rs.Symbol
	if sb == nil {
		return nil, fmt.Errorf("not found identifier %s", id)
	}
	return sb.Type, nil
}

func (resolver *Resolver) ResolveConstantValue(constant *parser.ConstValue) (model.Literal, error) {
	switch constant.Type {
	case parser.ConstType_ConstInt:
		return model.IntExpression{Src: int(constant.TypedValue.GetInt())}, nil
	case parser.ConstType_ConstDouble:
		return model.DoubleExpression{Src: constant.TypedValue.GetDouble()}, nil
	case parser.ConstType_ConstLiteral:
		return model.StringExpression{Src: constant.TypedValue.GetLiteral()}, nil
	case parser.ConstType_ConstList:
		eleType, err := switchConstantType(constant.Type)
		if err != nil {
			return nil, err
		}
		ret := model.ListExpression{
			ElementType: eleType,
		}
		for _, i := range constant.TypedValue.List {
			elem, err := resolver.ResolveConstantValue(i)
			if err != nil {
				return nil, err
			}
			ret.Elements = append(ret.Elements, elem)
		}
		return ret, nil
	case parser.ConstType_ConstMap:
		keyType, err := switchConstantType(constant.TypedValue.Map[0].Key.Type)
		if err != nil {
			return nil, err
		}
		valueType, err := switchConstantType(constant.TypedValue.Map[0].Value.Type)
		if err != nil {
			return nil, err
		}
		ret := model.MapExpression{
			KeyType:   keyType,
			ValueType: valueType,
			Elements:  make(map[string]model.Literal, len(constant.TypedValue.Map)),
		}
		for _, v := range constant.TypedValue.Map {
			value, err := resolver.ResolveConstantValue(v.Value)
			if err != nil {
				return nil, err
			}
			ret.Elements[v.Key.String()] = value
		}
		return ret, nil
	case parser.ConstType_ConstIdentifier:
		return resolver.ResolveIdentifier(*constant.TypedValue.Identifier)
	}
	return model.StringExpression{Src: constant.String()}, nil
}

func (resolver *Resolver) ResolveIdentifier(id string) (ret ResolvedSymbol, err error) {
	sb := resolver.Get(id)
	if sb == nil {
		return ResolvedSymbol{}, fmt.Errorf("identifier '%s' not found", id)
	}
	ret.Symbol = sb
	ret.Base = id
	if sb.Scope == &BaseThrift {
		return
	}
	if sb.Scope == resolver.mainPkg.Ast {
		resolver.mainPkg.Referred = true
		ret.Src = resolver.mainPkg.Model.PackageName
		return
	}

	sp := strings.SplitN(id, ".", 2)
	if ref, ok := resolver.refPkgs[sp[0]]; ok {
		ref.Referred = true
		ret.Base = sp[1]
		ret.Src = ref.Model.PackageName
		ret.Type.Scope = ref.Model
	} else {
		return ResolvedSymbol{}, fmt.Errorf("can't resolve identifier '%s'", id)
	}

	return
}

func (resolver *Resolver) ResolveTypeName(typ *parser.Type) (string, error) {
	if typ.GetIsTypedef() {
		rt, err := resolver.ResolveIdentifier(typ.GetName())
		if err != nil {
			return "", err
		}

		return rt.Expression(), nil
	}
	switch typ.GetCategory() {
	case parser.Category_Map:
		keyType, err := resolver.ResolveTypeName(typ.GetKeyType())
		if err != nil {
			return "", err
		}
		if typ.GetKeyType().GetCategory().IsStruct() {
			keyType = "*" + keyType
		}
		valueType, err := resolver.ResolveTypeName(typ.GetValueType())
		if err != nil {
			return "", err
		}
		if typ.GetValueType().GetCategory().IsStruct() {
			valueType = "*" + valueType
		}
		return fmt.Sprintf("map[%s]%s", keyType, valueType), nil
	case parser.Category_List, parser.Category_Set:
		// list/set -> []element for thriftgo
		// valueType refers the element type for list/set
		elemType, err := resolver.ResolveTypeName(typ.GetValueType())
		if err != nil {
			return "", err
		}
		if typ.GetValueType().GetCategory().IsStruct() {
			elemType = "*" + elemType
		}
		return fmt.Sprintf("[]%s", elemType), err
	}
	rt, err := resolver.ResolveIdentifier(typ.GetName())
	if err != nil {
		return "", err
	}

	return rt.Expression(), nil
}

func (resolver *Resolver) Get(name string) *Symbol {
	s, ok := resolver.root[name]
	if ok {
		return s
	}
	if strings.Contains(name, ".") {
		sp := strings.SplitN(name, ".", 2)
		if ref, ok := resolver.deps[sp[0]]; ok {
			if ss, ok := ref[sp[1]]; ok {
				return ss
			}
		}
	}
	return nil
}

func (resolver *Resolver) ExportReferred(all, needMain bool) (ret []*PackageReference) {
	for _, v := range resolver.refPkgs {
		if all {
			ret = append(ret, v)
			v.Referred = false
		} else if v.Referred {
			ret = append(ret, v)
			v.Referred = false
		}
	}
	if needMain && (all || resolver.mainPkg.Referred) {
		ret = append(ret, &resolver.mainPkg)
	}
	resolver.mainPkg.Referred = false
	return
}

func (resolver *Resolver) LoadAll(ast *parser.Thrift) error {
	var err error
	resolver.root, err = resolver.LoadOne(ast)
	if err != nil {
		return fmt.Errorf("load root package: %s", err)
	}

	includes := ast.GetIncludes()
	astMap := make(map[string]NameSpace, len(includes))
	for _, dep := range includes {
		bName := util.BaseName(dep.Path, ".thrift")
		astMap[bName], err = resolver.LoadOne(dep.Reference)
		if err != nil {
			return fmt.Errorf("load idl %s: %s", dep.Path, err)
		}
	}
	resolver.deps = astMap
	for _, td := range ast.Typedefs {
		name := td.GetAlias()
		if _, ex := resolver.root[name]; ex {
			if resolver.root[name].Type != nil {
				typ := newTypedefType(resolver.root[name].Type, name)
				resolver.root[name].Type = &typ
				continue
			}
		}
		sym := resolver.Get(td.Type.GetName())
		typ := newTypedefType(sym.Type, name)
		resolver.root[name].Type = &typ
	}
	return nil
}

func LoadBaseIdentifier() NameSpace {
	ret := make(NameSpace, 16)

	ret["true"] = &ConstTrue
	ret["false"] = &ConstFalse
	ret[`""`] = &ConstEmptyString
	ret["bool"] = &Symbol{
		Type:  model.TypeBool,
		Scope: &BaseThrift,
	}
	ret["byte"] = &Symbol{
		Type:  model.TypeByte,
		Scope: &BaseThrift,
	}
	ret["i8"] = &Symbol{
		Type:  model.TypeInt8,
		Scope: &BaseThrift,
	}
	ret["i16"] = &Symbol{
		Type:  model.TypeInt16,
		Scope: &BaseThrift,
	}
	ret["i32"] = &Symbol{
		Type:  model.TypeInt32,
		Scope: &BaseThrift,
	}
	ret["i64"] = &Symbol{
		Type:  model.TypeInt64,
		Scope: &BaseThrift,
	}
	ret["int"] = &Symbol{
		Type:  model.TypeInt,
		Scope: &BaseThrift,
	}
	ret["double"] = &Symbol{
		Type:  model.TypeFloat64,
		Scope: &BaseThrift,
	}
	ret["string"] = &Symbol{
		Type:  model.TypeString,
		Scope: &BaseThrift,
	}
	ret["binary"] = &Symbol{
		Type:  model.TypeBinary,
		Scope: &BaseThrift,
	}
	ret["list"] = &Symbol{
		Type:  model.TypeBaseList,
		Scope: &BaseThrift,
	}
	ret["set"] = &Symbol{
		Type:  model.TypeBaseSet,
		Scope: &BaseThrift,
	}
	ret["map"] = &Symbol{
		Type:  model.TypeBaseMap,
		Scope: &BaseThrift,
	}
	return ret
}

func (resolver *Resolver) LoadOne(ast *parser.Thrift) (NameSpace, error) {
	ret := LoadBaseIdentifier()

	for _, e := range ast.Enums {
		prefix := e.GetName()
		ret[prefix] = &Symbol{
			IsValue: false,
			Value:   e,
			Scope:   ast,
			Type:    newEnumType(prefix, model.CategoryEnum),
		}
		for _, ee := range e.Values {
			name := prefix + "." + ee.GetName()
			if _, exist := ret[name]; exist {
				return nil, fmt.Errorf("duplicated identifier '%s' in %s", name, ast.Filename)
			}

			ret[name] = &Symbol{
				IsValue: true,
				Value:   ee,
				Scope:   ast,
				Type:    newBaseType(model.TypeInt, model.CategoryEnum),
			}
		}
	}

	for _, e := range ast.Constants {
		name := e.GetName()
		if _, exist := ret[name]; exist {
			return nil, fmt.Errorf("duplicated identifier '%s' in %s", name, ast.Filename)
		}
		gt, _ := resolver.getBaseType(e.Type)
		ret[name] = &Symbol{
			IsValue: true,
			Value:   e,
			Scope:   ast,
			Type:    gt,
		}
	}

	for _, e := range ast.Structs {
		name := e.GetName()
		if _, exist := ret[name]; exist {
			return nil, fmt.Errorf("duplicated identifier '%s' in %s", name, ast.Filename)
		}
		ret[name] = &Symbol{
			IsValue: false,
			Value:   e,
			Scope:   ast,
			Type:    newStructType(name, model.CategoryStruct),
		}
	}

	for _, e := range ast.Unions {
		name := e.GetName()
		if _, exist := ret[name]; exist {
			return nil, fmt.Errorf("duplicated identifier '%s' in %s", name, ast.Filename)
		}
		ret[name] = &Symbol{
			IsValue: false,
			Value:   e,
			Scope:   ast,
			Type:    newStructType(name, model.CategoryStruct),
		}
	}

	for _, e := range ast.Exceptions {
		name := e.GetName()
		if _, exist := ret[name]; exist {
			return nil, fmt.Errorf("duplicated identifier '%s' in %s", name, ast.Filename)
		}
		ret[name] = &Symbol{
			IsValue: false,
			Value:   e,
			Scope:   ast,
			Type:    newStructType(name, model.CategoryStruct),
		}
	}

	for _, e := range ast.Services {
		name := e.GetName()
		if _, exist := ret[name]; exist {
			return nil, fmt.Errorf("duplicated identifier '%s' in %s", name, ast.Filename)
		}
		ret[name] = &Symbol{
			IsValue: false,
			Value:   e,
			Scope:   ast,
			Type:    newFuncType(name, model.CategoryService),
		}
	}

	for _, td := range ast.Typedefs {
		name := td.GetAlias()
		if _, exist := ret[name]; exist {
			return nil, fmt.Errorf("duplicated identifier '%s' in %s", name, ast.Filename)
		}
		gt, _ := resolver.getBaseType(td.Type)
		if gt == nil {
			sym := ret[td.Type.Name]
			if sym != nil {
				gt = sym.Type
			}
		}
		ret[name] = &Symbol{
			IsValue: false,
			Value:   td,
			Scope:   ast,
			Type:    gt,
		}
	}

	return ret, nil
}

func switchConstantType(constant parser.ConstType) (*model.Type, error) {
	switch constant {
	case parser.ConstType_ConstInt:
		return model.TypeInt, nil
	case parser.ConstType_ConstDouble:
		return model.TypeFloat64, nil
	case parser.ConstType_ConstLiteral:
		return model.TypeString, nil
	default:
		return nil, fmt.Errorf("unknown constant type %d", constant)
	}
}

func newTypedefType(t *model.Type, name string) model.Type {
	tmp := t
	typ := *tmp
	typ.Name = name
	typ.Category = model.CategoryTypedef
	return typ
}
