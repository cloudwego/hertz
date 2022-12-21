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

package generator

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/util"
)

type CustomTplRenderInfo struct {
	FilePath    string
	Service     string
	Module      string
	PackagePath string
	PackageName string
	Methods     []*HttpMethod
	Imports     map[string]*model.Model
	TPL         *template.Template
}

func (pkgGen *HttpPackageGenerator) generateCustomTemplate(pkg *HttpPackage) error {
	customTpls := pkgGen.customTpls
	divideMethods := pkgGen.divideMethods
	if len(customTpls) == 0 {
		return nil
	}
	for tplPath, customTpl := range customTpls {
		for _, service := range pkg.Services {
			if dm := divideMethods[tplPath]; dm {
				for _, method := range service.Methods {
					genFilePath, err := pkgGen.generateCustomFileName(tplPath, util.ToSnakeCase(service.Name), pkg.Package, util.ToSnakeCase(method.Name))
					if err != nil {
						return err
					}
					customPackagePath := util.SubPackage(pkgGen.ProjPackage, util.SubPackageDir(genFilePath))
					customInfo := &CustomTplRenderInfo{
						FilePath:    genFilePath,
						Service:     service.Name,
						Module:      pkg.Package,
						PackagePath: customPackagePath,
						PackageName: util.SplitPackage(customPackagePath, ""),
						Methods:     []*HttpMethod{method},
						TPL:         customTpl,
						Imports:     getImports([]*HttpMethod{method}),
					}
					if err = pkgGen.generateCustomTemplateFile(customInfo); err != nil {
						return err
					}
				}
			} else {
				genFilePath, err := pkgGen.generateCustomFileName(tplPath, util.ToSnakeCase(service.Name), pkg.Package, "")
				if err != nil {
					return err
				}
				customPackagePath := util.SubPackage(pkgGen.ProjPackage, util.SubPackageDir(genFilePath))
				customInfo := &CustomTplRenderInfo{
					FilePath:    genFilePath,
					Service:     service.Name,
					Module:      pkg.Package,
					PackagePath: customPackagePath,
					PackageName: util.SplitPackage(customPackagePath, ""),
					Methods:     service.Methods,
					TPL:         customTpl,
					Imports:     getImports(service.Methods),
				}
				if err = pkgGen.generateCustomTemplateFile(customInfo); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (pkgGen *HttpPackageGenerator) generateCustomFileName(path, serviceName, packagePath, method string) (string, error) {
	pathTpl, ok := pkgGen.pathNameTpls[path]
	if !ok {
		return "", fmt.Errorf("path is not exist,path = %s", path)
	}
	data := make(map[string]interface{})
	data["Method"] = method
	data["PackagePath"] = packagePath
	data["ServiceName"] = serviceName
	fileName := bytes.NewBuffer(nil)
	if err := pathTpl.Execute(fileName, data); err != nil {
		return "", err
	}
	return fileName.String(), nil
}

func (pkgGen *HttpPackageGenerator) generateCustomTemplateFile(customRenderInfo *CustomTplRenderInfo) error {
	isExist, err := util.PathExist(customRenderInfo.FilePath)
	if err != nil {
		return err
	}
	if isExist {
		return nil
	}
	file := bytes.NewBuffer(nil)
	if err = customRenderInfo.TPL.Execute(file, customRenderInfo); err != nil {
		return err
	}
	pkgGen.files = append(pkgGen.files, File{customRenderInfo.FilePath, file.String(), false, ""})
	return nil
}

func getImports(Methods []*HttpMethod) map[string]*model.Model {
	ims := make(map[string]*model.Model, len(Methods))
	for _, m := range Methods {
		// Iterate over the request and return parameters of the method to get import path.
		for key, mm := range m.Models {
			if v, ok := ims[mm.PackageName]; ok && v.Package != mm.Package {
				ims[key] = mm
				continue
			}
			ims[mm.PackageName] = mm
		}
	}
	return ims
}
