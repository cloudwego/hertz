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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudwego/hertz/cmd/hz/internal/config"
	"github.com/cloudwego/hertz/cmd/hz/internal/generator"
	"github.com/cloudwego/hertz/cmd/hz/internal/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/internal/meta"
	"github.com/cloudwego/hertz/cmd/hz/internal/util"
	"github.com/cloudwego/hertz/cmd/hz/internal/util/logs"
	thriftgo_plugin "github.com/cloudwego/thriftgo/plugin"
)

type Plugin struct {
	req    *thriftgo_plugin.Request
	args   *config.Argument
	logger *logs.StdLogger
}

func (plugin *Plugin) Run() int {
	plugin.setLogger()
	args := &config.Argument{}

	err := plugin.handleRequest()
	if err != nil {
		plugin.logger.Errorf("handle request failed: %s", err.Error())
		return meta.PluginError
	}

	args, err = plugin.parseArgs()
	if err != nil {
		plugin.logger.Errorf("parse args failed: %s", err.Error())
		return meta.PluginError
	}
	options := CheckTagOption(plugin.args)

	pkgInfo, err := plugin.getPackageInfo()

	cf, _ := util.GetColonPair(args.CustomizePackage)
	pkg, err := args.GetGoPackage()
	if err != nil {
		plugin.logger.Errorf("get go package failed: %s", err.Error())
		return meta.PluginError
	}
	handlerDir, err := args.GetHandlerDir()
	if err != nil {
		plugin.logger.Errorf("get handler dir failed: %s", err.Error())
		return meta.PluginError
	}
	routerDir, err := args.GetRouterDir()
	if err != nil {
		plugin.logger.Errorf("get router dir failed: %s", err.Error())
		return meta.PluginError
	}
	modelDir, err := args.GetModelDir()
	if err != nil {
		plugin.logger.Errorf("get model dir failed: %s", err.Error())
		return meta.PluginError
	}
	clientDir, err := args.GetClientDir()
	if err != nil {
		plugin.logger.Errorf("get client dir failed: %s", err.Error())
		return meta.PluginError
	}
	sg := generator.HttpPackageGenerator{
		ConfigPath: cf,
		HandlerDir: handlerDir,
		RouterDir:  routerDir,
		ModelDir:   modelDir,
		ClientDir:  clientDir,
		TemplateGenerator: generator.TemplateGenerator{
			OutputDir: args.OutDir,
		},
		ProjPackage: pkg,
		Options:     options,
	}
	if args.ModelBackend != "" {
		sg.Backend = meta.Backend(args.ModelBackend)
	}
	generator.SetDefaultTemplateConfig()

	err = sg.Generate(pkgInfo)
	if err != nil {
		plugin.logger.Errorf("generate package failed: %s", err.Error())
		return meta.PluginError
	}
	files, err := sg.GetFormatAndExcludedFiles()
	if err != nil {
		plugin.logger.Errorf("format file failed: %s", err.Error())
		return meta.PluginError
	}

	res, err := plugin.GetResponse(files, sg.OutputDir)
	if err != nil {
		plugin.logger.Errorf("get response failed: %s", err.Error())
		return meta.PluginError
	}
	plugin.response(res)
	if err != nil {
		plugin.logger.Errorf("response failed: %s", err.Error())
		return meta.PluginError
	}
	return 0
}

func (plugin *Plugin) setLogger() {
	plugin.logger = logs.NewStdLogger(logs.LevelInfo)
	plugin.logger.Defer = true
	logs.SetLogger(plugin.logger)
}

func (plugin *Plugin) recvLogger() []string {
	warns := plugin.logger.ErrLines()
	logs.SetLogger(logs.NewStdLogger(logs.LevelInfo))
	return warns
}

func (plugin *Plugin) handleRequest() error {
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read request failed: %s", err.Error())
	}
	req, err := thriftgo_plugin.UnmarshalRequest(data)
	if err != nil {
		return fmt.Errorf("unmarshal request failed: %s", err.Error())
	}
	plugin.req = req
	return nil
}

func (plugin *Plugin) parseArgs() (*config.Argument, error) {
	if plugin.req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	args := new(config.Argument)
	err := args.Unpack(plugin.req.PluginParameters)
	if err != nil {
		plugin.logger.Errorf("unpack args failed: %s", err.Error())
	}
	plugin.args = args
	return args, nil
}

func (plugin *Plugin) getPackageInfo() (*generator.HttpPackage, error) {
	req := plugin.req
	args := plugin.args

	ast := req.GetAST()
	if ast == nil {
		return nil, fmt.Errorf("no ast")
	}
	logs.Infof("Processing %s", ast.GetFilename())

	pkgMap := args.OptPkgMap
	pkg := getGoPackage(ast, pkgMap)
	main := &model.Model{
		FilePath:    ast.Filename,
		Package:     pkg,
		PackageName: util.SplitPackageName(pkg, ""),
	}
	rs := NewResolver(ast, main, pkgMap)
	err := rs.LoadAll(ast)
	if err != nil {
		return nil, err
	}

	idlPackage := getGoPackage(ast, pkgMap)
	if idlPackage == "" {
		return nil, fmt.Errorf("go package for '%s' is not defined", ast.GetFilename())
	}

	services, err := astToService(ast, rs)
	if err != nil {
		return nil, err
	}
	var models model.Models
	for _, s := range services {
		models.MergeArray(s.Models)
	}

	return &generator.HttpPackage{
		Services: services,
		IdlName:  ast.GetFilename(),
		Package:  idlPackage,
		Models:   models,
	}, nil
}

func (plugin *Plugin) response(res *thriftgo_plugin.Response) error {
	data, err := thriftgo_plugin.MarshalResponse(res)
	if err != nil {
		return fmt.Errorf("marshal response failed: %s", err.Error())
	}
	_, err = os.Stdout.Write(data)
	if err != nil {
		return fmt.Errorf("write response failed: %s", err.Error())
	}
	return nil
}

func (plugin *Plugin) InsertTag() ([]*thriftgo_plugin.Generated, error) {
	var res []*thriftgo_plugin.Generated

	if plugin.args.NoRecurse {
		outPath := plugin.req.OutputPath
		packageName := getGoPackage(plugin.req.AST, nil)
		fileName := util.BaseNameAndTrim(plugin.req.AST.GetFilename()) + ".go"
		outPath = filepath.Join(outPath, packageName, fileName)
		for _, st := range plugin.req.AST.Structs {
			stName := st.GetName()
			for _, f := range st.Fields {
				fieldName := f.GetName()
				hasBodyTag := false
				for _, t := range f.Annotations {
					if t.Key == AnnotationBody {
						hasBodyTag = true
						break
					}
				}
				field := model.Field{}
				err := injectTags(f, &field, true, false)
				if err != nil {
					return nil, err
				}
				tags := field.Tags
				var tagString string
				for idx, tag := range tags {
					if tag.Key == "json" && !hasBodyTag {
						continue
					}
					if idx == 0 {
						tagString += " " + tag.Key + ":\"" + tag.Value + ":\"" + " "
					} else if idx == len(tags)-1 {
						tagString += tag.Key + ":\"" + tag.Value + ":\""
					} else {
						tagString += tag.Key + ":\"" + tag.Value + ":\"" + " "
					}
				}
				insertPointer := "struct." + stName + "." + fieldName + "." + "tag"
				gen := &thriftgo_plugin.Generated{
					Content:        tagString,
					Name:           &outPath,
					InsertionPoint: &insertPointer,
				}
				res = append(res, gen)
			}
		}
		return res, nil
	}

	for ast := range plugin.req.AST.DepthFirstSearch() {
		outPath := plugin.req.OutputPath
		packageName := getGoPackage(ast, nil)
		fileName := util.BaseNameAndTrim(ast.GetFilename()) + ".go"
		outPath = filepath.Join(outPath, packageName, fileName)

		for _, st := range ast.Structs {
			stName := st.GetName()
			for _, f := range st.Fields {
				fieldName := f.GetName()
				hasBodyTag := false
				for _, t := range f.Annotations {
					if t.Key == AnnotationBody {
						hasBodyTag = true
						break
					}
				}
				field := model.Field{}
				err := injectTags(f, &field, true, false)
				if err != nil {
					return nil, err
				}
				tags := field.Tags
				var tagString string
				for idx, tag := range tags {
					if tag.Key == "json" && !hasBodyTag {
						continue
					}
					if idx == 0 {
						tagString += " " + tag.Key + ":\"" + tag.Value + ":\"" + " "
					} else if idx == len(tags)-1 {
						tagString += tag.Key + ":\"" + tag.Value + ":\""
					} else {
						tagString += tag.Key + ":\"" + tag.Value + ":\"" + " "
					}
				}
				insertPointer := "struct." + stName + "." + fieldName + "." + "tag"
				gen := &thriftgo_plugin.Generated{
					Content:        tagString,
					Name:           &outPath,
					InsertionPoint: &insertPointer,
				}
				res = append(res, gen)
			}
		}
	}
	return res, nil
}

func (plugin *Plugin) GetResponse(files []generator.File, outputDir string) (*thriftgo_plugin.Response, error) {
	var contents []*thriftgo_plugin.Generated
	for _, file := range files {
		filePath := filepath.Join(outputDir, file.Path)
		content := &thriftgo_plugin.Generated{
			Content: file.Content,
			Name:    &filePath,
		}
		contents = append(contents, content)
	}

	insertTag, err := plugin.InsertTag()
	if err != nil {
		return nil, err
	}

	contents = append(contents, insertTag...)
	warns := plugin.recvLogger()

	return &thriftgo_plugin.Response{
		Warnings: warns,
		Contents: contents,
	}, nil
}
