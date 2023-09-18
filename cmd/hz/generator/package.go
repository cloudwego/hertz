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

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"gopkg.in/yaml.v2"
)

type HttpPackage struct {
	IdlName    string
	Package    string
	Services   []*Service
	Models     []*model.Model
	RouterInfo *Router
}

type Service struct {
	Name          string
	Methods       []*HttpMethod
	ClientMethods []*ClientMethod
	Models        []*model.Model // all dependency models
	BaseDomain    string         // base domain for client code
	ServiceGroup  string         // service level router group
	ServiceGenDir string         // handler_dir for handler_by_service
}

// HttpPackageGenerator is used to record the configuration related to generating hertz http code.
type HttpPackageGenerator struct {
	ConfigPath     string       // package template path
	Backend        meta.Backend // model template
	Options        []Option
	CmdType        string
	ProjPackage    string // go module for project
	HandlerDir     string
	RouterDir      string
	ModelDir       string
	UseDir         string // model dir for third repo
	ClientDir      string // client dir for "new"/"update" command
	IdlClientDir   string // client dir for "client" command
	ForceClientDir string // client dir without namespace for "client" command
	BaseDomain     string // request domain for "client" command
	ServiceGenDir  string

	NeedModel            bool
	HandlerByMethod      bool // generate handler files with method dimension
	SnakeStyleMiddleware bool // use snake name style for middleware

	loadedBackend   Backend
	curModel        *model.Model
	processedModels map[*model.Model]bool

	TemplateGenerator
}

func (pkgGen *HttpPackageGenerator) Init() error {
	defaultConfig := packageConfig
	customConfig := TemplateConfig{}
	// unmarshal from user-defined config file if it exists
	if pkgGen.ConfigPath != "" {
		cdata, err := ioutil.ReadFile(pkgGen.ConfigPath)
		if err != nil {
			return fmt.Errorf("read layout config from  %s failed, err: %v", pkgGen.ConfigPath, err.Error())
		}
		if err = yaml.Unmarshal(cdata, &customConfig); err != nil {
			return fmt.Errorf("unmarshal layout config failed, err: %v", err.Error())
		}
		if reflect.DeepEqual(customConfig, TemplateConfig{}) {
			return errors.New("empty config")
		}
	}

	if pkgGen.tpls == nil {
		pkgGen.tpls = make(map[string]*template.Template, len(defaultConfig.Layouts))
	}
	if pkgGen.tplsInfo == nil {
		pkgGen.tplsInfo = make(map[string]*Template, len(defaultConfig.Layouts))
	}

	// extract routerTplName/middlewareTplName/handlerTplName/registerTplName/modelTplName/clientTplName directories
	// load default template
	for _, layout := range defaultConfig.Layouts {
		// default template use "fileName" as template name
		path := filepath.Base(layout.Path)
		err := pkgGen.loadLayout(layout, path, true)
		if err != nil {
			return err
		}
	}

	// override the default template, other customized file template will be loaded by "TemplateGenerator.Init"
	for _, layout := range customConfig.Layouts {
		if !IsDefaultPackageTpl(layout.Path) {
			continue
		}
		err := pkgGen.loadLayout(layout, layout.Path, true)
		if err != nil {
			return err
		}
	}

	pkgGen.Config = &customConfig
	// load Model tpl if need
	if pkgGen.Backend != "" {
		if err := pkgGen.LoadBackend(pkgGen.Backend); err != nil {
			return fmt.Errorf("load model template failed, err: %v", err.Error())
		}
	}

	pkgGen.processedModels = make(map[*model.Model]bool)
	pkgGen.TemplateGenerator.isPackageTpl = true

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

	if pkgGen.CmdType == meta.CmdClient {
		// default client dir
		clientDir := pkgGen.IdlClientDir
		// user specify client dir
		if len(pkgGen.ClientDir) != 0 {
			clientDir = pkgGen.ClientDir
		}
		if err := pkgGen.genClient(pkg, clientDir); err != nil {
			return err
		}
		if err := pkgGen.genCustomizedFile(pkg); err != nil {
			return err
		}
		return nil
	}

	// this is for handler_by_service, the handler_dir is {$HANDLER_DIR}/{$PKG}
	handlerDir := util.SubDir(pkgGen.HandlerDir, pkg.Package)
	if pkgGen.HandlerByMethod {
		handlerDir = pkgGen.HandlerDir
	}
	handlerPackage := util.SubPackage(pkgGen.ProjPackage, handlerDir)
	routerDir := util.SubDir(pkgGen.RouterDir, pkg.Package)
	routerPackage := util.SubPackage(pkgGen.ProjPackage, routerDir)

	root := NewRouterTree()
	if err := pkgGen.genHandler(pkg, handlerDir, handlerPackage, root); err != nil {
		return err
	}

	if err := pkgGen.genRouter(pkg, root, handlerPackage, routerDir, routerPackage); err != nil {
		return err
	}

	if err := pkgGen.genCustomizedFile(pkg); err != nil {
		return err
	}

	return nil
}
