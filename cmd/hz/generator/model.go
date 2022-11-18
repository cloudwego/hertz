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
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/generator/model/golang"
	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/util"
)

//---------------------------------Backend----------------------------------

type Option string

const (
	OptionMarshalEnumToText  Option = "MarshalEnumToText"
	OptionTypedefAsTypeAlias Option = "TypedefAsTypeAlias"
)

type Backend interface {
	Template() (*template.Template, error)
	List() map[string]string
	SetOption(opts string) error
	GetOptions() []string
	Funcs(name string, fn interface{}) error
}

type GolangBackend struct{}

func (gb *GolangBackend) Template() (*template.Template, error) {
	return golang.Template()
}

func (gb *GolangBackend) List() map[string]string {
	return golang.List()
}

func (gb *GolangBackend) SetOption(opts string) error {
	return golang.SetOption(opts)
}

func (gb *GolangBackend) GetOptions() []string {
	return golang.GetOptions()
}

func (gb *GolangBackend) Funcs(name string, fn interface{}) error {
	return golang.Funcs(name, fn)
}

func switchBackend(backend meta.Backend) Backend {
	switch backend {
	case meta.BackendGolang:
		return &GolangBackend{}
	}
	return loadThirdPartyBackend(string(backend))
}

func loadThirdPartyBackend(plugin string) Backend {
	panic("no implement yet!")
}

/**********************Generating*************************/

func (pkgGen *HttpPackageGenerator) LoadBackend(backend meta.Backend) error {
	bd := switchBackend(backend)
	if bd == nil {
		return fmt.Errorf("no found backend '%s'", backend)
	}
	for _, opt := range pkgGen.Options {
		if err := bd.SetOption(string(opt)); err != nil {
			return fmt.Errorf("set option %s error, err: %v", opt, err.Error())
		}
	}

	err := bd.Funcs("ROOT", func() *model.Model {
		return pkgGen.curModel
	})
	if err != nil {
		return fmt.Errorf("register global function in model template failed, err: %v", err.Error())
	}

	tpl, err := bd.Template()
	if err != nil {
		return fmt.Errorf("load backend %s failed, err: %v", backend, err.Error())
	}

	if pkgGen.tpls == nil {
		pkgGen.tpls = map[string]*template.Template{}
	}
	pkgGen.tpls[modelTplName] = tpl
	pkgGen.loadedBackend = bd
	return nil
}

func (pkgGen *HttpPackageGenerator) GenModel(data *model.Model, gen bool) error {
	if pkgGen.processedModels == nil {
		pkgGen.processedModels = map[*model.Model]bool{}
	}

	if _, ok := pkgGen.processedModels[data]; !ok {
		var path string
		var updatePackage bool
		if strings.HasPrefix(data.Package, pkgGen.ProjPackage) && data.PackageName != pkgGen.ProjPackage {
			path = data.Package[len(pkgGen.ProjPackage):]
		} else {
			path = data.Package
			updatePackage = true
		}
		modelDir := util.SubDir(pkgGen.ModelDir, path)
		if updatePackage {
			data.Package = util.SubPackage(pkgGen.ProjPackage, modelDir)
		}
		data.FilePath = filepath.Join(modelDir, util.BaseNameAndTrim(data.FilePath)+".go")

		pkgGen.processedModels[data] = true
	}

	for _, dep := range data.Imports {
		if err := pkgGen.GenModel(dep, false); err != nil {
			return fmt.Errorf("generate model %s failed, err: %v", dep.FilePath, err.Error())
		}
	}

	if gen && !data.IsEmpty() {
		pkgGen.curModel = data
		removeDuplicateImport(data)
		err := pkgGen.TemplateGenerator.Generate(data, modelTplName, data.FilePath, false)
		pkgGen.curModel = nil
		return err
	}
	return nil
}

// Idls with the same Package do not need to refer to each other
func removeDuplicateImport(data *model.Model) {
	for k, v := range data.Imports {
		if data.Package == v.Package {
			delete(data.Imports, k)
		}
	}
}
