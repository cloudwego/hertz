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
	"path/filepath"

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/util"
)

type ClientMethod struct {
	*HttpMethod
	BodyParamsCode   string
	QueryParamsCode  string
	PathParamsCode   string
	HeaderParamsCode string
	FormValueCode    string
	FormFileCode     string
}

type ClientFile struct {
	FilePath      string
	ServiceName   string
	BaseDomain    string
	Imports       map[string]*model.Model
	ClientMethods []*ClientMethod
}

func (pkgGen *HttpPackageGenerator) genClient(pkg *HttpPackage, clientDir string) error {
	for _, s := range pkg.Services {
		hertzClientPath := filepath.Join(clientDir, "hertz_client.go")
		isExist, err := util.PathExist(hertzClientPath)
		if err != nil {
			return err
		}
		if !isExist {
			err := pkgGen.TemplateGenerator.Generate(nil, hertzClientTplName, hertzClientPath, false)
			if err != nil {
				return err
			}
		}
		client := ClientFile{
			FilePath:      filepath.Join(clientDir, util.ToSnakeCase(s.Name)+".go"),
			ServiceName:   util.ToSnakeCase(s.Name),
			ClientMethods: s.ClientMethods,
			BaseDomain:    s.BaseDomain,
		}
		client.Imports = make(map[string]*model.Model, len(client.ClientMethods))
		for _, m := range client.ClientMethods {
			// Iterate over the request and return parameters of the method to get import path.
			for key, mm := range m.Models {
				if v, ok := client.Imports[mm.PackageName]; ok && v.Package != mm.Package {
					client.Imports[key] = mm
					continue
				}
				client.Imports[mm.PackageName] = mm
			}
		}
		err = pkgGen.TemplateGenerator.Generate(client, serviceClientName, client.FilePath, false)
		if err != nil {
			return err
		}
	}
	return nil
}
