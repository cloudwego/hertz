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

	"github.com/cloudwego/hertz/cmd/hz/internal/generator"
	"github.com/cloudwego/hertz/cmd/hz/internal/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/internal/util"
	"github.com/cloudwego/hertz/cmd/hz/internal/util/logs"
	"github.com/cloudwego/thriftgo/generator/golang/styles"
	"github.com/cloudwego/thriftgo/parser"
)

/*---------------------------Import-----------------------------*/

func getGoPackage(ast *parser.Thrift, pkgMap map[string]string) string {
	filePackage := ast.GetFilename()
	if opt, ok := pkgMap[filePackage]; ok {
		return opt
	} else {
		goPackage := ast.GetNamespaceOrReferenceName("go")
		if goPackage != "" {
			return util.SplitPackage(goPackage, "")
		}
		// If namespace is not declared, the file name (without the extension) is used as the package name
		return util.SplitPackage(filePackage, ".thrift")
	}
}

/*---------------------------Service-----------------------------*/

func astToService(ast *parser.Thrift, resolver *Resolver) ([]*generator.Service, error) {
	ss := ast.GetServices()
	out := make([]*generator.Service, 0, len(ss))
	var models model.Models

	for _, s := range ss {
		resolver.ExportReferred(true, false)
		service := &generator.Service{
			Name: s.GetName(),
		}

		ms := s.GetFunctions()
		methods := make([]*generator.HttpMethod, 0, len(ms))
		for _, m := range ms {

			rs := getAnnotations(m.Annotations, HttpMethodAnnotations)
			if len(rs) > 1 {
				return nil, fmt.Errorf("invalid http router '%v' for %s.%s", rs, s.Name, m.Name)
			}
			if len(rs) == 0 {
				continue
			}
			if len(rs) > 1 {
				return nil, fmt.Errorf("too many 'api.XXX' annotations: %s", rs)
			}

			var handlerOutDir string
			genPaths := getAnnotation(m.Annotations, ApiGenPath)
			if len(genPaths) == 0 {
				handlerOutDir = ""
			} else if len(genPaths) > 1 {
				return nil, fmt.Errorf("too many 'api.handler_path' for %s", m.Name)
			} else {
				handlerOutDir = genPaths[0]
			}

			hmethod, path := util.GetFirstKV(rs)
			if len(path) != 1 || path[0] == "" {
				return nil, fmt.Errorf("invalid api.%s  for %s.%s: %s", hmethod, s.Name, m.Name, path)
			}

			var reqName string
			if len(m.Arguments) >= 1 {
				rt, err := resolver.ResolveIdentifier(m.Arguments[0].GetType().GetName())
				if err != nil {
					return nil, err
				}
				// if len(m.Arguments) > 1 {
				// 	*warns = append(*warns, fmt.Sprintf("function '%s' has more than one argument, but only the first can be used in hertz now", m.GetName()))
				// }
				reqName = rt.Expression()
			}
			var respName string
			if !m.Oneway {
				respType, err := resolver.ResolveIdentifier(m.GetFunctionType().GetName())
				if err != nil {
					return nil, err
				}
				respName = respType.Expression()
			}

			sr, _ := util.GetFirstKV(getAnnotations(m.Annotations, SerializerTags))
			method := &generator.HttpMethod{
				Name:            util.CamelString(m.GetName()),
				HTTPMethod:      hmethod,
				RequestTypeName: reqName,
				ReturnTypeName:  respName,
				Path:            path[0],
				Serializer:      sr,
				OutputDir:       handlerOutDir,
				// Annotations:     m.Annotations,
			}
			refs := resolver.ExportReferred(false, true)
			method.Models = make(map[string]*model.Model, len(refs))
			for _, ref := range refs {
				// method.Models[ref.Model.PackageName] = ref.Model
				if v, ok := method.Models[ref.Model.PackageName]; ok && (v.Package != ref.Model.Package) {
					return nil, fmt.Errorf("Package name: %s  redeclared in %s and %s ", ref.Model.PackageName, v.Package, ref.Model.Package)
				}
				method.Models[ref.Model.PackageName] = ref.Model
			}
			models.MergeMap(method.Models)
			methods = append(methods, method)
		}

		service.Methods = methods
		service.Models = models
		out = append(out, service)
	}
	return out, nil
}

/*---------------------------Model-----------------------------*/

var BaseThrift = parser.Thrift{}

func switchBaseType(typ *parser.Type) *model.Type {
	switch typ.Name {
	case "bool":
		return model.TypeBool
	case "byte":
		return model.TypeByte
	case "i8":
		return model.TypeInt8
	case "i16":
		return model.TypeInt16
	case "i32":
		return model.TypeInt32
	case "i64":
		return model.TypeInt64
	case "int":
		return model.TypeInt
	case "double":
		return model.TypeFloat64
	case "string":
		return model.TypeString
	case "binary":
		return model.TypeBinary
	}
	return nil
}

func newBaseType(typ *model.Type, cg model.Category) *model.Type {
	cyp := *typ
	cyp.Category = cg
	return &cyp
}

func newStructType(name string, cg model.Category) *model.Type {
	return &model.Type{
		Name:     name,
		Scope:    nil,
		Kind:     model.KindStruct,
		Category: cg,
		Indirect: false,
		Extra:    nil,
		HasNew:   true,
	}
}

func newEnumType(name string, cg model.Category) *model.Type {
	return &model.Type{
		Name:     name,
		Scope:    &model.BaseModel,
		Kind:     model.KindInt,
		Category: cg,
	}
}

func newFuncType(name string, cg model.Category) *model.Type {
	return &model.Type{
		Name:     name,
		Scope:    nil,
		Kind:     model.KindFunc,
		Category: cg,
		Indirect: false,
		Extra:    nil,
		HasNew:   false,
	}
}

func (resolver *Resolver) getFieldType(typ *parser.Type) (*model.Type, error) {
	if dt, _ := resolver.getBaseType(typ); dt != nil {
		return dt, nil
	}
	sb := resolver.Get(typ.Name)
	if sb != nil {
		return sb.Type, nil
	}
	return nil, fmt.Errorf("unknown type: %s", typ.Name)
}

type ResolvedSymbol struct {
	Base string
	Src  string
	*Symbol
}

func (rs ResolvedSymbol) Expression() string {
	base, err := NameStyle.Identify(rs.Base)
	if err != nil {
		logs.Warnf("%s naming style for %s failed, fall back to %s, please refer to the variable manually!", NameStyle.Name(), rs.Base, rs.Base)
		base = rs.Base
	}
	if rs.Src != "" {
		if !rs.IsValue && model.IsBaseType(rs.Type) {
			return base
		}
		return fmt.Sprintf("%s.%s", rs.Src, base)
	}
	return base
}

func astToModel(ast *parser.Thrift, rs *Resolver) (*model.Model, error) {
	main := rs.mainPkg.Model
	if main == nil {
		main = new(model.Model)
	}

	// typedefs
	tds := ast.GetTypedefs()
	typdefs := make([]model.TypeDef, 0, len(tds))
	for _, t := range tds {
		td := model.TypeDef{
			Scope: main,
			Alias: t.Alias,
		}
		if bt, err := rs.ResolveType(t.Type); bt == nil || err != nil {
			return nil, fmt.Errorf("%s has no type definition, error: %s", t.String(), err)
		} else {
			td.Type = bt
		}
		typdefs = append(typdefs, td)
	}
	main.Typedefs = typdefs

	// constants
	cts := ast.GetConstants()
	constants := make([]model.Constant, 0, len(cts))
	variables := make([]model.Variable, 0, len(cts))
	for _, c := range cts {
		ft, err := rs.ResolveType(c.Type)
		if err != nil {
			return nil, err
		}
		if ft.Name == model.TypeBaseList.Name || ft.Name == model.TypeBaseMap.Name || ft.Name == model.TypeBaseSet.Name {
			resolveValue, err := rs.ResolveConstantValue(c.Value)
			if err != nil {
				return nil, err
			}
			vt := model.Variable{
				Scope: main,
				Name:  c.Name,
				Type:  ft,
				Value: resolveValue,
			}
			variables = append(variables, vt)
		} else {
			resolveValue, err := rs.ResolveConstantValue(c.Value)
			if err != nil {
				return nil, err
			}
			ct := model.Constant{
				Scope: main,
				Name:  c.Name,
				Type:  ft,
				Value: resolveValue,
			}
			constants = append(constants, ct)
		}
	}
	main.Constants = constants
	main.Variables = variables

	// Enums
	ems := ast.GetEnums()
	enums := make([]model.Enum, 0, len(ems))
	for _, e := range ems {
		em := model.Enum{
			Scope:  main,
			Name:   e.GetName(),
			GoType: "int64",
		}
		vs := make([]model.Constant, 0, len(e.Values))
		for _, ee := range e.Values {
			vs = append(vs, model.Constant{
				Scope: main,
				Name:  ee.Name,
				Type:  model.TypeInt64,
				Value: model.IntExpression{Src: int(ee.Value)},
			})
		}
		em.Values = vs
		enums = append(enums, em)
	}
	main.Enums = enums

	// Structs
	sts := make([]*parser.StructLike, 0, len(ast.Structs))
	sts = append(sts, ast.Structs...)
	structs := make([]model.Struct, 0, len(ast.Structs)+len(ast.Unions)+len(ast.Exceptions))
	for _, st := range sts {
		s := model.Struct{
			Scope:           main,
			Name:            st.GetName(),
			Category:        model.CategoryStruct,
			LeadingComments: removeCommentsSlash(st.GetReservedComments()),
		}

		vs := make([]model.Field, 0, len(st.Fields))
		for _, f := range st.Fields {
			fieldName, _ := (&styles.ThriftGo{}).Identify(f.Name)
			isP, err := isPointer(f, rs)
			if err != nil {
				return nil, err
			}
			resolveType, err := rs.ResolveType(f.Type)
			if err != nil {
				return nil, err
			}
			field := model.Field{
				Scope: &s,
				Name:  fieldName,
				Type:  resolveType,
				// IsSetDefault:    f.IsSetDefault(),
				LeadingComments: removeCommentsSlash(f.GetReservedComments()),
				IsPointer:       isP,
			}
			err = injectTags(f, &field, true, true)
			if err != nil {
				return nil, err
			}
			vs = append(vs, field)
		}
		checkDuplicatedFileName(vs)
		s.Fields = vs
		structs = append(structs, s)
	}

	sts = make([]*parser.StructLike, 0, len(ast.Unions))
	sts = append(sts, ast.Unions...)
	for _, st := range sts {
		s := model.Struct{
			Scope:           main,
			Name:            st.GetName(),
			Category:        model.CategoryUnion,
			LeadingComments: removeCommentsSlash(st.GetReservedComments()),
		}
		vs := make([]model.Field, 0, len(st.Fields))
		for _, f := range st.Fields {
			fieldName, _ := (&styles.ThriftGo{}).Identify(f.Name)
			isP, err := isPointer(f, rs)
			if err != nil {
				return nil, err
			}
			resolveType, err := rs.ResolveType(f.Type)
			if err != nil {
				return nil, err
			}
			field := model.Field{
				Scope:           &s,
				Name:            fieldName,
				Type:            resolveType,
				LeadingComments: removeCommentsSlash(f.GetReservedComments()),
				IsPointer:       isP,
			}
			err = injectTags(f, &field, true, true)
			if err != nil {
				return nil, err
			}
			vs = append(vs, field)
		}
		checkDuplicatedFileName(vs)
		s.Fields = vs
		structs = append(structs, s)
	}

	sts = make([]*parser.StructLike, 0, len(ast.Exceptions))
	sts = append(sts, ast.Exceptions...)
	for _, st := range sts {
		s := model.Struct{
			Scope:           main,
			Name:            st.GetName(),
			Category:        model.CategoryException,
			LeadingComments: removeCommentsSlash(st.GetReservedComments()),
		}
		vs := make([]model.Field, 0, len(st.Fields))
		for _, f := range st.Fields {
			fieldName, _ := (&styles.ThriftGo{}).Identify(f.Name)
			isP, err := isPointer(f, rs)
			if err != nil {
				return nil, err
			}
			resolveType, err := rs.ResolveType(f.Type)
			if err != nil {
				return nil, err
			}
			field := model.Field{
				Scope:           &s,
				Name:            fieldName,
				Type:            resolveType,
				LeadingComments: removeCommentsSlash(f.GetReservedComments()),
				IsPointer:       isP,
			}
			err = injectTags(f, &field, true, true)
			if err != nil {
				return nil, err
			}
			vs = append(vs, field)
		}
		checkDuplicatedFileName(vs)
		s.Fields = vs
		structs = append(structs, s)
	}
	main.Structs = structs

	// In case of only the service refers another model, therefore scanning service is necessary
	ss := ast.GetServices()
	var err error
	for _, s := range ss {
		for _, m := range s.GetFunctions() {
			_, err = rs.ResolveType(m.GetFunctionType())
			if err != nil {
				return nil, err
			}
			for _, a := range m.GetArguments() {
				_, err = rs.ResolveType(a.GetType())
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return main, nil
}

// removeCommentsSlash can remove double slash for comments with thrift
func removeCommentsSlash(comments string) string {
	if comments == "" {
		return ""
	}

	return comments[2:]
}

func isPointer(f *parser.Field, rs *Resolver) (bool, error) {
	typ, err := rs.ResolveType(f.GetType())
	if err != nil {
		return false, err
	}
	if typ == nil {
		return false, fmt.Errorf("can not get type: %s for %s", f.GetType(), f.GetName())
	}
	if typ.Kind == model.KindStruct || typ.Kind == model.KindMap || typ.Kind == model.KindSlice {
		return false, nil
	}

	if f.GetRequiredness().IsOptional() {
		return true, nil
	} else {
		return false, nil
	}
}

func getNewFieldName(fieldName string, fieldNameSet map[string]bool) string {
	if _, ex := fieldNameSet[fieldName]; ex {
		fieldName = fieldName + "_"
		return getNewFieldName(fieldName, fieldNameSet)
	}
	return fieldName
}

func checkDuplicatedFileName(vs []model.Field) {
	fieldNameSet := make(map[string]bool)
	for i := 0; i < len(vs); i++ {
		if _, ex := fieldNameSet[vs[i].Name]; ex {
			newName := getNewFieldName(vs[i].Name, fieldNameSet)
			fieldNameSet[newName] = true
			vs[i].Name = newName
		} else {
			fieldNameSet[vs[i].Name] = true
		}
	}
}
