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
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"text/template"

	"github.com/cloudwego/hertz/cmd/hz/internal/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/internal/meta"
	"github.com/cloudwego/hertz/cmd/hz/internal/util"
	"gopkg.in/yaml.v2"
)

type HttpPackage struct {
	IdlName  string
	Package  string
	Services []*Service
	Models   []*model.Model
}

type Service struct {
	Name    string
	Methods []*HttpMethod
	Models  []*model.Model // all dependency models
}

type HttpPackageGenerator struct {
	ConfigPath  string
	Backend     meta.Backend
	Options     []Option
	ProjPackage string
	HandlerDir  string
	RouterDir   string
	ModelDir    string
	ClientDir   string
	NeedModel   bool

	loadedBackend   Backend
	curModel        *model.Model
	processedModels map[*model.Model]bool

	TemplateGenerator
}

func (pkgGen *HttpPackageGenerator) Init() error {
	config := packageConfig
	// unmarshal from user-defined config file if it exists
	if pkgGen.ConfigPath != "" {
		cdata, err := ioutil.ReadFile(pkgGen.ConfigPath)
		if err != nil {
			return fmt.Errorf("read layout config from  %s failed, err: %v", pkgGen.ConfigPath, err.Error())
		}
		config = TemplateConfig{}
		if err = yaml.Unmarshal(cdata, &config); err != nil {
			return fmt.Errorf("unmarshal layout config failed, err: %v", err.Error())
		}
		if reflect.DeepEqual(config, TemplateConfig{}) {
			return errors.New("empty config")
		}
	}

	// extract routerTplName/middlewareTplName/handlerTplName/registerTplName/modelTplName/clientTplName directories
	for _, layout := range config.Layouts {
		name := filepath.Base(layout.Path)
		if !IsDefaultTpl(name) {
			continue
		}
		path := name
		delims := DefaultDelimiters
		if layout.Delims[0] != "" && layout.Delims[1] != "" {
			delims = layout.Delims
		}
		tpl := template.New(path)
		tpl = tpl.Delims(delims[0], delims[1])
		var err error
		if layout.Body != "" && layout.TemplatePath == "" {
			if tpl, err = tpl.Parse(layout.Body); err != nil {
				return fmt.Errorf("parse template '%s' failed, err: %v", path, err.Error())
			}
		}
		if layout.TemplatePath != "" && layout.Body == "" {
			abspath, err := filepath.Abs(layout.TemplatePath)
			if err != nil {
				return fmt.Errorf("get absolute path of template '%s' failed, err: %v", layout.TemplatePath, err.Error())
			}
			tplBytes, err := ioutil.ReadFile(abspath)
			if err != nil {
				fmt.Errorf("read %s fail, error: %v", abspath, err.Error())
			}
			if tpl, err = tpl.Parse(util.Bytes2Str(tplBytes)); err != nil {
				return fmt.Errorf("parse template '%s' failed, err: %v", path, err.Error())
			}
		} else if layout.TemplatePath != "" && layout.Body != "" {
			return fmt.Errorf("only one of Body and TemplatePath can be used at the same time")
		}

		if pkgGen.tpls == nil {
			pkgGen.tpls = make(map[string]*template.Template, len(config.Layouts))
		}
		pkgGen.tpls[path] = tpl
	}

	pkgGen.Config = &config
	// load Model tpl if need
	if pkgGen.Backend != "" {
		if err := pkgGen.LoadBackend(pkgGen.Backend); err != nil {
			return fmt.Errorf("load model template failed, err: %v", err.Error())
		}
	}

	pkgGen.processedModels = make(map[*model.Model]bool)

	return pkgGen.TemplateGenerator.Init()
}

func (pkgGen *HttpPackageGenerator) checkInited() (bool, error) {
	if pkgGen.tpls == nil {
		if err := pkgGen.Init(); err != nil {
			return false, fmt.Errorf("init layout config failed, err: %v", err.Error())
		}
	}
	return pkgGen.ConfigPath == "", nil
}

func (pkgGen *HttpPackageGenerator) Generate(pkg *HttpPackage) error {
	if _, err := pkgGen.checkInited(); err != nil {
		return err
	}
	if len(pkg.Models) != 0 {
		for _, m := range pkg.Models {
			if err := pkgGen.GenModel(m, pkgGen.NeedModel); err != nil {
				return fmt.Errorf("generate model %s failed, err: %v", m.FilePath, err.Error())
			}
		}
	}

	handlerDir := util.SubDir(pkgGen.HandlerDir, pkg.Package)
	handlerPackage := util.SubPackage(pkgGen.ProjPackage, handlerDir)
	routerDir := util.SubDir(pkgGen.RouterDir, pkg.Package)
	routerPackage := util.SubPackage(pkgGen.ProjPackage, routerDir)

	root := NewRouterTree()
	if err := pkgGen.genHandler(pkg, handlerDir, handlerPackage, root); err != nil {
		return err
	}

	return pkgGen.genRouter(pkg, root, handlerPackage, routerDir, routerPackage)
}
