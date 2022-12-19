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

	"github.com/cloudwego/hertz/cmd/hz/util"
)

type CustomTplRenderInfo struct {
	FilePath          string
	Service           string
	Module            string
	CustomPackagePath string
	CustomPackageName string
	Methods           []*HttpMethod
	TPL               *template.Template
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
					genFilePath, err := pkgGen.generateCustomFileName(tplPath, service.Name, pkg.Package, method.Name)
					if err != nil {
						return err
					}
					customPackagePath := util.SubPackage(pkgGen.ProjPackage, util.SubPackageDir(genFilePath))
					customInfo := &CustomTplRenderInfo{
						FilePath:          genFilePath,
						Service:           service.Name,
						Module:            pkg.Package,
						CustomPackagePath: customPackagePath,
						CustomPackageName: util.SplitPackage(customPackagePath, ""),
						Methods:           service.Methods,
						TPL:               customTpl,
					}
					if err = pkgGen.generateCustomTemplateFile(customInfo); err != nil {
						return err
					}
				}
			} else {
				genFilePath, err := pkgGen.generateCustomFileName(tplPath, service.Name, pkg.Package, "")
				if err != nil {
					return err
				}
				customPackagePath := util.SubPackage(pkgGen.ProjPackage, util.SubPackageDir(genFilePath))
				customInfo := &CustomTplRenderInfo{
					FilePath:          genFilePath,
					Service:           service.Name,
					Module:            pkg.Package,
					CustomPackagePath: customPackagePath,
					CustomPackageName: util.SplitPackage(customPackagePath, ""),
					Methods:           service.Methods,
					TPL:               customTpl,
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
	data["serviceName"] = serviceName
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
	data := make(map[string]interface{})
	data["Methods"] = customRenderInfo.Methods
	data["Module"] = customRenderInfo.Module
	data["ServiceName"] = customRenderInfo.Service
	data["PackagePath"] = customRenderInfo.CustomPackagePath
	data["PackageName"] = customRenderInfo.CustomPackageName

	file := bytes.NewBuffer(nil)
	if err = customRenderInfo.TPL.Execute(file, data); err != nil {
		return err
	}
	pkgGen.files = append(pkgGen.files, File{customRenderInfo.FilePath, file.String(), false, ""})
	return nil
}
