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
	"sort"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/config"
	"github.com/cloudwego/hertz/cmd/hz/generator"
	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/hertz/cmd/hz/util/logs"
	"github.com/cloudwego/thriftgo/generator/golang"
	"github.com/cloudwego/thriftgo/generator/golang/styles"
	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/semantic"
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

func astToService(ast *parser.Thrift, resolver *Resolver, args *config.Argument) ([]*generator.Service, error) {
	ss := ast.GetServices()
	out := make([]*generator.Service, 0, len(ss))
	var models model.Models
	extendServices := getExtendServices(ast)
	for _, s := range ss {
		// if the service is extended, it is not processed
		if extendServices.exist(s.Name) && args.EnableExtends {
			logs.Debugf("%s is extended, so skip it\n", s.Name)
			continue
		}

		resolver.ExportReferred(true, false)
		service := &generator.Service{
			Name: s.GetName(),
		}
		service.BaseDomain = ""
		domainAnno := getAnnotation(s.Annotations, ApiBaseDomain)
		if len(domainAnno) == 1 {
			if args.CmdType == meta.CmdClient {
				service.BaseDomain = domainAnno[0]
			}
		}
		service.ServiceGroup = ""
		groupAnno := getAnnotation(s.Annotations, ApiServiceGroup)
		if len(groupAnno) == 1 {
			if args.CmdType != meta.CmdClient {
				service.ServiceGroup = groupAnno[0]
			}
		}
		service.ServiceGenDir = ""
		serviceGenDirAnno := getAnnotation(s.Annotations, ApiServiceGenDir)
		if len(serviceGenDirAnno) == 1 {
			if args.CmdType != meta.CmdClient {
				service.ServiceGenDir = serviceGenDirAnno[0]
			}
		}
		ms := s.GetFunctions()
		if len(s.Extends) != 0 && args.EnableExtends {
			// all the services that are extended to the current service
			extendsFuncs, err := getAllExtendFunction(s, ast, resolver, args)
			if err != nil {
				return nil, fmt.Errorf("parser extend function failed, err=%v", err)
			}
			ms = append(ms, extendsFuncs...)
		}
		methods := make([]*generator.HttpMethod, 0, len(ms))
		clientMethods := make([]*generator.ClientMethod, 0, len(ms))
		servicePathAnno := getAnnotation(s.Annotations, ApiServicePath)
		servicePath := ""
		if len(servicePathAnno) > 0 {
			servicePath = servicePathAnno[0]
		}
		for _, m := range ms {
			rs := getAnnotations(m.Annotations, HttpMethodAnnotations)
			if len(rs) == 0 {
				continue
			}
			httpAnnos := httpAnnotations{}
			for k, v := range rs {
				httpAnnos = append(httpAnnos, httpAnnotation{
					method: k,
					path:   v,
				})
			}
			// turn the map into a slice and sort it to make sure getting the results in the same order every time
			sort.Sort(httpAnnos)
			handlerOutDir := servicePath
			genPaths := getAnnotation(m.Annotations, ApiGenPath)
			if len(genPaths) == 1 {
				handlerOutDir = genPaths[0]
			} else if len(genPaths) > 0 {
				return nil, fmt.Errorf("too many 'api.handler_path' for %s", m.Name)
			}

			hmethod, path := httpAnnos[0].method, httpAnnos[0].path
			if len(path) == 0 || path[0] == "" {
				return nil, fmt.Errorf("invalid api.%s  for %s.%s: %s", hmethod, s.Name, m.Name, path)
			}

			var reqName, reqRawName, reqPackage string
			if len(m.Arguments) >= 1 {
				if len(m.Arguments) > 1 {
					logs.Warnf("function '%s' has more than one argument, but only the first can be used in hertz now", m.GetName())
				}
				var err error
				reqName, err = resolver.ResolveTypeName(m.Arguments[0].GetType())
				if err != nil {
					return nil, err
				}
				if strings.Contains(reqName, ".") && !m.Arguments[0].GetType().Category.IsContainerType() {
					// If reqName contains "." , then it must be of the form "pkg.name".
					// so reqRawName='name', reqPackage='pkg'
					names := strings.Split(reqName, ".")
					if len(names) != 2 {
						return nil, fmt.Errorf("request name: %s is wrong", reqName)
					}
					reqRawName = names[1]
					reqPackage = names[0]
				}
			}
			var respName, respRawName, respPackage string
			if !m.Oneway {
				var err error
				respName, err = resolver.ResolveTypeName(m.GetFunctionType())
				if err != nil {
					return nil, err
				}
				if strings.Contains(respName, ".") && !m.GetFunctionType().Category.IsContainerType() {
					names := strings.Split(respName, ".")
					if len(names) != 2 {
						return nil, fmt.Errorf("response name: %s is wrong", respName)
					}
					// If respName contains "." , then it must be of the form "pkg.name".
					// so respRawName='name', respPackage='pkg'
					respRawName = names[1]
					respPackage = names[0]
				}
			}

			sr, _ := util.GetFirstKV(getAnnotations(m.Annotations, SerializerTags))
			method := &generator.HttpMethod{
				Name:               util.CamelString(m.GetName()),
				HTTPMethod:         hmethod,
				RequestTypeName:    reqName,
				RequestTypeRawName: reqRawName,
				RequestTypePackage: reqPackage,
				ReturnTypeName:     respName,
				ReturnTypeRawName:  respRawName,
				ReturnTypePackage:  respPackage,
				Path:               path[0],
				Serializer:         sr,
				OutputDir:          handlerOutDir,
				GenHandler:         true,
				// Annotations:     m.Annotations,
			}
			refs := resolver.ExportReferred(false, true)
			method.Models = make(map[string]*model.Model, len(refs))
			for _, ref := range refs {
				if v, ok := method.Models[ref.Model.PackageName]; ok && (v.Package != ref.Model.Package) {
					return nil, fmt.Errorf("Package name: %s  redeclared in %s and %s ", ref.Model.PackageName, v.Package, ref.Model.Package)
				}
				method.Models[ref.Model.PackageName] = ref.Model
			}
			models.MergeMap(method.Models)
			methods = append(methods, method)
			for idx, anno := range httpAnnos {
				for i := 0; i < len(anno.path); i++ {
					if idx == 0 && i == 0 { // idx==0 && i==0 has been added above
						continue
					}
					newMethod, err := newHTTPMethod(s, m, method, i, anno)
					if err != nil {
						return nil, err
					}
					methods = append(methods, newMethod)
				}
			}
			if args.CmdType == meta.CmdClient {
				clientMethod := &generator.ClientMethod{}
				clientMethod.HttpMethod = method
				rt, err := resolver.ResolveIdentifier(m.Arguments[0].GetType().GetName())
				if err != nil {
					return nil, err
				}
				err = parseAnnotationToClient(clientMethod, m.Arguments[0].GetType(), rt, args.EnableClientOptional)
				if err != nil {
					return nil, err
				}
				clientMethods = append(clientMethods, clientMethod)
			}
		}

		service.ClientMethods = clientMethods
		service.Methods = methods
		service.Models = models
		out = append(out, service)
	}
	return out, nil
}

func newHTTPMethod(s *parser.Service, m *parser.Function, method *generator.HttpMethod, i int, anno httpAnnotation) (*generator.HttpMethod, error) {
	newMethod := *method
	hmethod, path := anno.method, anno.path
	if path[i] == "" {
		return nil, fmt.Errorf("invalid api.%s for %s.%s: %s", hmethod, s.Name, m.Name, path[i])
	}
	newMethod.HTTPMethod = hmethod
	newMethod.Path = path[i]
	newMethod.GenHandler = false
	return &newMethod, nil
}

func parseAnnotationToClient(clientMethod *generator.ClientMethod, p *parser.Type, symbol ResolvedSymbol, enableOptional bool) error {
	if p == nil {
		return fmt.Errorf("get type failed for parse annotatoon to client")
	}
	typeName := p.GetName()
	if strings.Contains(typeName, ".") {
		ret := strings.Split(typeName, ".")
		typeName = ret[len(ret)-1]
	}
	scope, err := golang.BuildScope(thriftgoUtil, symbol.Scope)
	if err != nil {
		return fmt.Errorf("can not build scope for %s", p.Name)
	}
	thriftgoUtil.SetRootScope(scope)
	st := scope.StructLike(typeName)
	if st == nil {
		logs.Infof("the type '%s' for method '%s' is base type, so skip parse client info\n")
		return nil
	}
	var (
		hasBodyAnnotation bool
		hasFormAnnotation bool
	)
	for _, field := range st.Fields() {
		hasAnnotation := false
		isStringFieldType := false
		isOptional := false
		if field.GetType().String() == "string" {
			isStringFieldType = true
		}
		if field.GetRequiredness() == parser.FieldType_Optional {
			isOptional = true
		}
		if anno := getAnnotation(field.Annotations, AnnotationQuery); len(anno) > 0 {
			hasAnnotation = true
			query := checkSnakeName(anno[0])
			if isOptional && enableOptional {
				clientMethod.QueryParamsCode += fmt.Sprintf("%q: func() interface{} {\n\t\t\t\tif req.IsSet%s() {\n\t\t\t\t\treturn req.Get%s()\n\t\t\t\t} else {\n\t\t\t\t\treturn nil\n\t\t\t\t}}(),\n", query, field.GoName().String(), field.GoName().String())
			} else {
				clientMethod.QueryParamsCode += fmt.Sprintf("%q: req.Get%s(),\n", query, field.GoName().String())
			}
		}

		if anno := getAnnotation(field.Annotations, AnnotationPath); len(anno) > 0 {
			hasAnnotation = true
			path := anno[0]
			if isStringFieldType {
				clientMethod.PathParamsCode += fmt.Sprintf("%q: req.Get%s(),\n", path, field.GoName().String())
			} else {
				clientMethod.PathParamsCode += fmt.Sprintf("%q: fmt.Sprint(req.Get%s()),\n", path, field.GoName().String())
			}
		}

		if anno := getAnnotation(field.Annotations, AnnotationHeader); len(anno) > 0 {
			hasAnnotation = true
			header := anno[0]
			if isStringFieldType {
				clientMethod.HeaderParamsCode += fmt.Sprintf("%q: req.Get%s(),\n", header, field.GoName().String())
			} else {
				clientMethod.HeaderParamsCode += fmt.Sprintf("%q: fmt.Sprint(req.Get%s()),\n", header, field.GoName().String())
			}
		}

		if anno := getAnnotation(field.Annotations, AnnotationForm); len(anno) > 0 {
			hasAnnotation = true
			form := checkSnakeName(anno[0])
			hasFormAnnotation = true
			if isStringFieldType {
				clientMethod.FormValueCode += fmt.Sprintf("%q: req.Get%s(),\n", form, field.GoName().String())
			} else {
				clientMethod.FormValueCode += fmt.Sprintf("%q: fmt.Sprint(req.Get%s()),\n", form, field.GoName().String())
			}
		}

		if anno := getAnnotation(field.Annotations, AnnotationBody); len(anno) > 0 {
			hasAnnotation = true
			hasBodyAnnotation = true
		}

		if anno := getAnnotation(field.Annotations, AnnotationFileName); len(anno) > 0 {
			hasAnnotation = true
			fileName := anno[0]
			hasFormAnnotation = true
			clientMethod.FormFileCode += fmt.Sprintf("%q: req.Get%s(),\n", fileName, field.GoName().String())
		}
		if anno := getAnnotation(field.Annotations, AnnotationCookie); len(anno) > 0 {
			hasAnnotation = true
			// cookie do nothing
		}
		if !hasAnnotation && strings.EqualFold(clientMethod.HTTPMethod, "get") {
			if isOptional && enableOptional {
				clientMethod.QueryParamsCode += fmt.Sprintf("%q: func() interface{} {\n\t\t\t\tif req.IsSet%s() {\n\t\t\t\t\treturn req.Get%s()\n\t\t\t\t} else {\n\t\t\t\t\treturn nil\n\t\t\t\t}}(),\n", checkSnakeName(field.GetName()), field.GoName().String(), field.GoName().String())
			} else {
				clientMethod.QueryParamsCode += fmt.Sprintf("%q: req.Get%s(),\n", checkSnakeName(field.GetName()), field.GoName().String())
			}
		}
	}
	clientMethod.BodyParamsCode = meta.SetBodyParam
	if hasBodyAnnotation && hasFormAnnotation {
		clientMethod.FormValueCode = ""
		clientMethod.FormFileCode = ""
	}
	if !hasBodyAnnotation && hasFormAnnotation {
		clientMethod.BodyParamsCode = ""
	}

	return nil
}

type extendServiceList []string

func (svr extendServiceList) exist(serviceName string) bool {
	for _, s := range svr {
		if s == serviceName {
			return true
		}
	}
	return false
}

func getExtendServices(ast *parser.Thrift) (res extendServiceList) {
	for a := range ast.DepthFirstSearch() {
		for _, svc := range a.Services {
			if len(svc.Extends) > 0 {
				res = append(res, svc.Extends)
			}
		}
	}
	return
}

func getAllExtendFunction(svc *parser.Service, ast *parser.Thrift, resolver *Resolver, args *config.Argument) (res []*parser.Function, err error) {
	if len(svc.Extends) == 0 {
		return
	}
	parts := semantic.SplitType(svc.Extends)
	switch len(parts) {
	case 1:
		if resolver.mainPkg.Ast.Filename == ast.Filename { // extended current service for master IDL
			extendSvc, found := ast.GetService(parts[0])
			if found {
				funcs := extendSvc.GetFunctions()
				// determine if it still has extends
				extendFuncs, err := getAllExtendFunction(extendSvc, ast, resolver, args)
				if err != nil {
					return nil, err
				}
				res = append(res, append(funcs, extendFuncs...)...)
			}
			return res, nil
		} else { // extended current service for other IDL
			extendSvc, found := ast.GetService(parts[0])
			if found {
				base, err := addResolverDependency(resolver, ast, args)
				if err != nil {
					return nil, err
				}
				funcs := extendSvc.GetFunctions()
				for _, f := range funcs {
					processExtendsType(f, base)
				}
				extendFuncs, err := getAllExtendFunction(extendSvc, ast, resolver, args)
				if err != nil {
					return nil, err
				}
				res = append(res, append(funcs, extendFuncs...)...)
			}
			return res, nil
		}
	case 2:
		refAst, found := ast.GetReference(parts[0])
		base, err := addResolverDependency(resolver, refAst, args)
		if err != nil {
			return nil, err
		}
		// ff the service extends from other files, it has to resolve the dependencies of other files as well
		for _, dep := range refAst.Includes {
			_, err := addResolverDependency(resolver, dep.Reference, args)
			if err != nil {
				return nil, err
			}
		}
		if found {
			extendSvc, found := refAst.GetService(parts[1])
			if found {
				funcs := extendSvc.GetFunctions()
				for _, f := range funcs {
					processExtendsType(f, base)
				}
				extendFuncs, err := getAllExtendFunction(extendSvc, refAst, resolver, args)
				if err != nil {
					return nil, err
				}
				res = append(res, append(funcs, extendFuncs...)...)
			}
		}
		return res, nil
	}

	return res, nil
}

func processExtendsType(f *parser.Function, base string) {
	// the method of other file is extended, and the package of req/resp needs to be changed
	// ex. base.thrift -> Resp Method(Req){}
	//					  base.Resp Method(base.Req){}
	if len(f.Arguments) > 0 {
		if f.Arguments[0].Type.Category.IsContainerType() {
			switch f.Arguments[0].Type.Category {
			case parser.Category_Set, parser.Category_List:
				if !strings.Contains(f.Arguments[0].Type.ValueType.Name, ".") && f.Arguments[0].Type.ValueType.Category.IsStruct() {
					f.Arguments[0].Type.ValueType.Name = base + "." + f.Arguments[0].Type.ValueType.Name
				}
			case parser.Category_Map:
				if !strings.Contains(f.Arguments[0].Type.ValueType.Name, ".") && f.Arguments[0].Type.ValueType.Category.IsStruct() {
					f.Arguments[0].Type.ValueType.Name = base + "." + f.Arguments[0].Type.ValueType.Name
				}
				if !strings.Contains(f.Arguments[0].Type.KeyType.Name, ".") && f.Arguments[0].Type.KeyType.Category.IsStruct() {
					f.Arguments[0].Type.KeyType.Name = base + "." + f.Arguments[0].Type.KeyType.Name
				}
			}
		} else {
			if !strings.Contains(f.Arguments[0].Type.Name, ".") && f.Arguments[0].Type.Category.IsStruct() {
				f.Arguments[0].Type.Name = base + "." + f.Arguments[0].Type.Name
			}
		}
	}

	if f.FunctionType.Category.IsContainerType() {
		switch f.FunctionType.Category {
		case parser.Category_Set, parser.Category_List:
			if !strings.Contains(f.FunctionType.ValueType.Name, ".") && f.FunctionType.ValueType.Category.IsStruct() {
				f.FunctionType.ValueType.Name = base + "." + f.FunctionType.ValueType.Name
			}
		case parser.Category_Map:
			if !strings.Contains(f.FunctionType.ValueType.Name, ".") && f.FunctionType.ValueType.Category.IsStruct() {
				f.FunctionType.ValueType.Name = base + "." + f.FunctionType.ValueType.Name
			}
			if !strings.Contains(f.FunctionType.KeyType.Name, ".") && f.FunctionType.KeyType.Category.IsStruct() {
				f.FunctionType.KeyType.Name = base + "." + f.FunctionType.KeyType.Name
			}
		}
	} else {
		if !strings.Contains(f.FunctionType.Name, ".") && f.FunctionType.Category.IsStruct() {
			f.FunctionType.Name = base + "." + f.FunctionType.Name
		}
	}
}

func getUniqueResolveDependentName(name string, resolver *Resolver) string {
	rawName := name
	for i := 0; i < 10000; i++ {
		if _, exist := resolver.deps[name]; !exist {
			return name
		}
		name = rawName + fmt.Sprint(i)
	}

	return name
}

func addResolverDependency(resolver *Resolver, ast *parser.Thrift, args *config.Argument) (string, error) {
	namespace, err := resolver.LoadOne(ast)
	if err != nil {
		return "", err
	}
	baseName := util.BaseName(ast.Filename, ".thrift")
	if refPkg, exist := resolver.refPkgs[baseName]; !exist {
		resolver.deps[baseName] = namespace
	} else {
		if ast.Filename != refPkg.Ast.Filename {
			baseName = getUniqueResolveDependentName(baseName, resolver)
			resolver.deps[baseName] = namespace
		}
	}
	pkg := getGoPackage(ast, args.OptPkgMap)
	impt := ast.Filename
	pkgName := util.SplitPackageName(pkg, "")
	pkgName, err = util.GetPackageUniqueName(pkgName)
	if err != nil {
		return "", err
	}
	ref := &PackageReference{baseName, impt, &model.Model{
		FilePath:    ast.Filename,
		Package:     pkg,
		PackageName: pkgName,
	}, ast, false}
	if _, exist := resolver.refPkgs[baseName]; !exist {
		resolver.refPkgs[baseName] = ref
	}

	return baseName, nil
}

/*---------------------------Model-----------------------------*/

var BaseThrift = parser.Thrift{}

var baseTypes = map[string]string{
	"bool":   "bool",
	"byte":   "int8",
	"i8":     "int8",
	"i16":    "int16",
	"i32":    "int32",
	"i64":    "int64",
	"double": "float64",
	"string": "string",
	"binary": "[]byte",
}

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
	// base type no need to do name style
	if model.IsBaseType(rs.Type) {
		// base type mapping
		if val, exist := baseTypes[rs.Base]; exist {
			base = val
		}
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
