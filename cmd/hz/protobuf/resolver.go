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

package protobuf

import (
	"fmt"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/jhump/protoreflect/desc"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Symbol struct {
	Space   string
	Name    string
	IsValue bool
	Type    *model.Type
	Value   interface{}
	Scope   *descriptorpb.FileDescriptorProto
}

type NameSpace map[string]*Symbol

var (
	ConstTrue = Symbol{
		IsValue: true,
		Type:    model.TypeBool,
		Value:   true,
		Scope:   &BaseProto,
	}
	ConstFalse = Symbol{
		IsValue: true,
		Type:    model.TypeBool,
		Value:   false,
		Scope:   &BaseProto,
	}
	ConstEmptyString = Symbol{
		IsValue: true,
		Type:    model.TypeString,
		Value:   "",
		Scope:   &BaseProto,
	}
)

type PackageReference struct {
	IncludeBase string
	IncludePath string
	Model       *model.Model
	Ast         *descriptorpb.FileDescriptorProto
	Referred    bool
}

func getReferPkgMap(pkgMap map[string]string, incs []*descriptorpb.FileDescriptorProto, mainModel *model.Model) (map[string]*PackageReference, error) {
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
		pkg := getGoPackage(inc, pkgMap)
		path := inc.GetName()
		base := util.BaseName(path, ".proto")
		fileName := inc.GetName()
		pkgName := util.BaseName(pkg, "")
		if pn, exist := pkgAliasMap[pkg]; exist {
			pkgName = pn
		} else {
			pkgName, err = util.GetPackageUniqueName(pkgName)
			pkgAliasMap[pkg] = pkgName
			if err != nil {
				return nil, fmt.Errorf("get package unique name failed, err: %v", err)
			}
		}
		out[fileName] = &PackageReference{base, path, &model.Model{
			FilePath:    path,
			Package:     pkg,
			PackageName: pkgName,
		}, inc, false}
	}

	return out, nil
}

type FileInfos struct {
	Official  map[string]*descriptorpb.FileDescriptorProto
	PbReflect map[string]*desc.FileDescriptor
}

type Resolver struct {
	// idl symbols
	rootName string
	root     NameSpace
	deps     map[string]NameSpace

	// exported models
	mainPkg PackageReference
	refPkgs map[string]*PackageReference

	files FileInfos
}

func updateFiles(fileName string, files FileInfos) (FileInfos, error) {
	file, _ := files.PbReflect[fileName]
	if file == nil {
		return FileInfos{}, fmt.Errorf("%s not found", fileName)
	}
	fileDep := file.GetDependencies()

	maps := make(map[string]*descriptorpb.FileDescriptorProto, len(fileDep)+1)
	sourceInfoMap := make(map[string]*desc.FileDescriptor, len(fileDep)+1)
	for _, dep := range fileDep {
		ast := dep.AsFileDescriptorProto()
		maps[dep.GetName()] = ast
		sourceInfoMap[dep.GetName()] = dep
	}
	ast := file.AsFileDescriptorProto()
	maps[file.GetName()] = ast
	sourceInfoMap[file.GetName()] = file

	newFileInfo := FileInfos{
		Official:  maps,
		PbReflect: sourceInfoMap,
	}

	return newFileInfo, nil
}

func NewResolver(ast *descriptorpb.FileDescriptorProto, files FileInfos, model *model.Model, pkgMap map[string]string) (*Resolver, error) {
	file := ast.GetName()
	deps := ast.GetDependency()
	var err error
	if files.PbReflect != nil {
		files, err = updateFiles(file, files)
		if err != nil {
			return nil, err
		}
	}
	incs := make([]*descriptorpb.FileDescriptorProto, 0, len(deps))
	for _, dep := range deps {
		if v, ok := files.Official[dep]; ok {
			incs = append(incs, v)
		} else {
			return nil, fmt.Errorf("%s not found", dep)
		}
	}
	pm, err := getReferPkgMap(pkgMap, incs, model)
	if err != nil {
		return nil, fmt.Errorf("get package map failed, err: %v", err)
	}
	return &Resolver{
		root:    make(NameSpace),
		deps:    make(map[string]NameSpace),
		refPkgs: pm,
		files:   files,
		mainPkg: PackageReference{
			IncludeBase: util.BaseName(file, ".proto"),
			IncludePath: file,
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
		return nil, fmt.Errorf("%s not found", includeBase)
	}
	return ref.Model, nil
}

func (resolver *Resolver) getBaseType(f *descriptorpb.FieldDescriptorProto, nested []*descriptorpb.DescriptorProto) (*model.Type, error) {
	bt := switchBaseType(f.GetType())
	if bt != nil {
		return checkListType(bt, f.GetLabel()), nil
	}

	nt := getNestedType(f, nested)
	if nt != nil {
		fields := nt.GetField()
		if IsMapEntry(nt) {
			t := *model.TypeBaseMap
			tk, err := resolver.ResolveType(fields[0], nt.GetNestedType())
			if err != nil {
				return nil, err
			}
			tv, err := resolver.ResolveType(fields[1], nt.GetNestedType())
			if err != nil {
				return nil, err
			}
			t.Extra = []*model.Type{tk, tv}
			return &t, nil
		}
	}
	return nil, nil
}

func IsMapEntry(nt *descriptorpb.DescriptorProto) bool {
	fields := nt.GetField()
	return len(fields) == 2 && fields[0].GetName() == "key" && fields[1].GetName() == "value"
}

func checkListType(typ *model.Type, label descriptorpb.FieldDescriptorProto_Label) *model.Type {
	if label == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		t := *model.TypeBaseList
		t.Extra = []*model.Type{typ}
		return &t
	}
	return typ
}

func getNestedType(f *descriptorpb.FieldDescriptorProto, nested []*descriptorpb.DescriptorProto) *descriptorpb.DescriptorProto {
	tName := f.GetTypeName()
	entry := util.SplitPackageName(tName, "")
	for _, nt := range nested {
		if nt.GetName() == entry {
			return nt
		}
	}
	return nil
}

func (resolver *Resolver) ResolveType(f *descriptorpb.FieldDescriptorProto, nested []*descriptorpb.DescriptorProto) (*model.Type, error) {
	bt, err := resolver.getBaseType(f, nested)
	if err != nil {
		return nil, err
	}
	if bt != nil {
		return bt, nil
	}

	tName := f.GetTypeName()
	symbol, err := resolver.ResolveIdentifier(tName)
	if err != nil {
		return nil, err
	}
	deepType := checkListType(symbol.Type, f.GetLabel())
	return deepType, nil
}

func (resolver *Resolver) ResolveIdentifier(id string) (ret *Symbol, err error) {
	ret = resolver.Get(id)
	if ret == nil {
		return nil, fmt.Errorf("not found identifier %s", id)
	}

	var ref *PackageReference
	if _, ok := resolver.deps[ret.Space]; ok {
		ref = resolver.refPkgs[ret.Scope.GetName()]
		if ref != nil {
			ref.Referred = true
			ret.Type.Scope = ref.Model
		}
	}
	// bugfix: root & dep file has the same package(namespace), the 'ret' will miss the namespace match for root.
	// This results in a lack of dependencies in the generated handlers.
	if ref == nil && ret.Scope == resolver.mainPkg.Ast {
		resolver.mainPkg.Referred = true
		ret.Type.Scope = resolver.mainPkg.Model
	}
	return
}

func (resolver *Resolver) getFieldType(f *descriptorpb.FieldDescriptorProto, nested []*descriptorpb.DescriptorProto) (*model.Type, error) {
	dt, err := resolver.getBaseType(f, nested)
	if err != nil {
		return nil, err
	}
	if dt != nil {
		return dt, nil
	}
	sb := resolver.Get(f.GetTypeName())
	if sb != nil {
		return sb.Type, nil
	}
	return nil, fmt.Errorf("not found type %s", f.GetTypeName())
}

func (resolver *Resolver) Get(name string) *Symbol {
	if strings.HasPrefix(name, "."+resolver.rootName) {
		id := strings.TrimPrefix(name, "."+resolver.rootName+".")
		if v, ok := resolver.root[id]; ok {
			return v
		}
	}

	// directly map first
	var space string
	if idx := strings.LastIndex(name, "."); idx >= 0 && idx < len(name)-1 {
		space = strings.TrimLeft(name[:idx], ".")
	}
	if ns, ok := resolver.deps[space]; ok {
		id := strings.TrimPrefix(name, "."+space+".")
		if s, ok := ns[id]; ok {
			return s
		}
	}

	// iterate check nested type in dependencies
	for s, m := range resolver.deps {
		if strings.HasPrefix(name, "."+s) {
			id := strings.TrimPrefix(name, "."+s+".")
			if s, ok := m[id]; ok {
				return s
			}
		}
	}
	return nil
}

func (resolver *Resolver) ExportReferred(all, needMain bool) (ret []*PackageReference) {
	for _, v := range resolver.refPkgs {
		if all {
			ret = append(ret, v)
		} else if v.Referred {
			ret = append(ret, v)
		}
		v.Referred = false
	}

	if needMain && (all || resolver.mainPkg.Referred) {
		ret = append(ret, &resolver.mainPkg)
	}
	resolver.mainPkg.Referred = false
	return
}

func (resolver *Resolver) LoadAll(ast *descriptorpb.FileDescriptorProto) error {
	var err error
	resolver.root, err = resolver.LoadOne(ast)
	if err != nil {
		return fmt.Errorf("load main idl failed: %s", err)
	}
	resolver.rootName = ast.GetPackage()

	includes := ast.GetDependency()
	astMap := make(map[string]NameSpace, len(includes))
	for _, dep := range includes {
		file, ok := resolver.files.Official[dep]
		if !ok {
			return fmt.Errorf("not found included idl %s", dep)
		}
		depNamespace, err := resolver.LoadOne(file)
		if err != nil {
			return fmt.Errorf("load idl '%s' failed: %s", dep, err)
		}
		ns, existed := astMap[file.GetPackage()]
		if existed {
			depNamespace = mergeNamespace(ns, depNamespace)
		}
		astMap[file.GetPackage()] = depNamespace
	}
	resolver.deps = astMap
	return nil
}

func mergeNamespace(first, second NameSpace) NameSpace {
	for k, v := range second {
		if _, existed := first[k]; !existed {
			first[k] = v
		}
	}
	return first
}

func LoadBaseIdentifier(ast *descriptorpb.FileDescriptorProto) map[string]*Symbol {
	ret := make(NameSpace, len(ast.GetEnumType())+len(ast.GetMessageType())+len(ast.GetExtension())+len(ast.GetService()))

	ret["true"] = &ConstTrue
	ret["false"] = &ConstFalse
	ret[`""`] = &ConstEmptyString
	ret["bool"] = &Symbol{
		Type:  model.TypeBool,
		Scope: ast,
	}
	ret["uint32"] = &Symbol{
		Type:  model.TypeUint32,
		Scope: ast,
	}
	ret["uint64"] = &Symbol{
		Type:  model.TypeUint64,
		Scope: ast,
	}
	ret["fixed32"] = &Symbol{
		Type:  model.TypeUint32,
		Scope: ast,
	}
	ret["fixed64"] = &Symbol{
		Type:  model.TypeUint64,
		Scope: ast,
	}
	ret["int32"] = &Symbol{
		Type:  model.TypeInt32,
		Scope: ast,
	}
	ret["int64"] = &Symbol{
		Type:  model.TypeInt64,
		Scope: ast,
	}
	ret["sint32"] = &Symbol{
		Type:  model.TypeInt32,
		Scope: ast,
	}
	ret["sint64"] = &Symbol{
		Type:  model.TypeInt64,
		Scope: ast,
	}
	ret["sfixed32"] = &Symbol{
		Type:  model.TypeInt32,
		Scope: ast,
	}
	ret["sfixed64"] = &Symbol{
		Type:  model.TypeInt64,
		Scope: ast,
	}
	ret["double"] = &Symbol{
		Type:  model.TypeFloat64,
		Scope: ast,
	}
	ret["float"] = &Symbol{
		Type:  model.TypeFloat32,
		Scope: ast,
	}
	ret["string"] = &Symbol{
		Type:  model.TypeString,
		Scope: ast,
	}
	ret["bytes"] = &Symbol{
		Type:  model.TypeBinary,
		Scope: ast,
	}
	return ret
}

func (resolver *Resolver) LoadOne(ast *descriptorpb.FileDescriptorProto) (NameSpace, error) {
	ret := LoadBaseIdentifier(ast)
	space := util.BaseName(ast.GetPackage(), "")
	prefix := "." + space

	for _, e := range ast.GetEnumType() {
		name := strings.TrimLeft(e.GetName(), prefix)
		ret[e.GetName()] = &Symbol{
			Name:    name,
			Space:   space,
			IsValue: false,
			Value:   e,
			Scope:   ast,
			Type:    model.NewEnumType(name, model.CategoryEnum),
		}
		for _, ee := range e.GetValue() {
			name := strings.TrimLeft(ee.GetName(), prefix)
			ret[ee.GetName()] = &Symbol{
				Name:    name,
				Space:   space,
				IsValue: true,
				Value:   ee,
				Scope:   ast,
				Type:    model.NewCategoryType(model.TypeInt, model.CategoryEnum),
			}
		}
	}

	for _, mt := range ast.GetMessageType() {
		name := strings.TrimLeft(mt.GetName(), prefix)
		ret[mt.GetName()] = &Symbol{
			Name:    name,
			Space:   space,
			IsValue: false,
			Value:   mt,
			Scope:   ast,
			Type:    model.NewStructType(name, model.CategoryStruct),
		}

		for _, nt := range mt.GetNestedType() {
			ntname := name + "_" + nt.GetName()
			ret[name+"."+nt.GetName()] = &Symbol{
				Name:    ntname,
				Space:   space,
				IsValue: false,
				Value:   nt,
				Scope:   ast,
				Type:    model.NewStructType(ntname, model.CategoryStruct),
			}
		}
	}

	for _, s := range ast.GetService() {
		name := strings.TrimLeft(s.GetName(), prefix)
		ret[s.GetName()] = &Symbol{
			Name:    name,
			Space:   space,
			IsValue: false,
			Value:   s,
			Scope:   ast,
			Type:    model.NewFuncType(name, model.CategoryService),
		}
	}

	return ret, nil
}

func (resolver *Resolver) GetFiles() FileInfos {
	return resolver.files
}
