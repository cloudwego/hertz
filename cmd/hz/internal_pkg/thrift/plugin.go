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
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/config"
	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/generator"
	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/meta"
	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/util"
	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/util/logs"
	"github.com/cloudwego/thriftgo/generator/golang/styles"
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
	defer func() {
		if args.Verbose {
			verboseLog := plugin.recvVerboseLogger()
			if len(verboseLog) != 0 {
				fmt.Fprintf(os.Stderr, verboseLog)
			}
		} else {
			warning := plugin.recvWarningLogger()
			if len(warning) != 0 {
				fmt.Fprintf(os.Stderr, warning)
			}
		}
	}()

	err := plugin.handleRequest()
	if err != nil {
		logs.Errorf("handle request failed: %s", err.Error())
		return meta.PluginError
	}

	args, err = plugin.parseArgs()
	if err != nil {
		logs.Errorf("parse args failed: %s", err.Error())
		return meta.PluginError
	}

	err = plugin.initNameStyle()
	if err != nil {
		logs.Errorf("init naming style failed: %s", err.Error())
		return meta.PluginError
	}

	options := CheckTagOption(plugin.args)

	pkgInfo, err := plugin.getPackageInfo()

	cf, _ := util.GetColonPair(args.CustomizePackage)
	pkg, err := args.GetGoPackage()
	if err != nil {
		logs.Errorf("get go package failed: %s", err.Error())
		return meta.PluginError
	}
	handlerDir, err := args.GetHandlerDir()
	if err != nil {
		logs.Errorf("get handler dir failed: %s", err.Error())
		return meta.PluginError
	}
	routerDir, err := args.GetRouterDir()
	if err != nil {
		logs.Errorf("get router dir failed: %s", err.Error())
		return meta.PluginError
	}
	modelDir, err := args.GetModelDir()
	if err != nil {
		logs.Errorf("get model dir failed: %s", err.Error())
		return meta.PluginError
	}
	clientDir, err := args.GetClientDir()
	if err != nil {
		logs.Errorf("get client dir failed: %s", err.Error())
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
		logs.Errorf("generate package failed: %s", err.Error())
		return meta.PluginError
	}
	files, err := sg.GetFormatAndExcludedFiles()
	if err != nil {
		logs.Errorf("format file failed: %s", err.Error())
		return meta.PluginError
	}

	res, err := plugin.GetResponse(files, sg.OutputDir)
	if err != nil {
		logs.Errorf("get response failed: %s", err.Error())
		return meta.PluginError
	}
	plugin.response(res)
	if err != nil {
		logs.Errorf("response failed: %s", err.Error())
		return meta.PluginError
	}
	return 0
}

func (plugin *Plugin) setLogger() {
	plugin.logger = logs.NewStdLogger(logs.LevelInfo)
	plugin.logger.Defer = true
	plugin.logger.ErrOnly = true
	logs.SetLogger(plugin.logger)
}

func (plugin *Plugin) recvWarningLogger() string {
	warns := plugin.logger.Warn()
	plugin.logger.Flush()
	logs.SetLogger(logs.NewStdLogger(logs.LevelInfo))
	return warns
}

func (plugin *Plugin) recvVerboseLogger() string {
	info := plugin.logger.Out()
	warns := plugin.logger.Warn()
	verboseLog := string(info) + warns
	plugin.logger.Flush()
	logs.SetLogger(logs.NewStdLogger(logs.LevelInfo))
	return verboseLog
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
		logs.Errorf("unpack args failed: %s", err.Error())
	}
	plugin.args = args
	return args, nil
}

// initNameStyle initializes the naming style based on the "naming_style" option for thrift.
func (plugin *Plugin) initNameStyle() error {
	if len(plugin.args.ThriftOptions) == 0 {
		return nil
	}
	for _, opt := range plugin.args.ThriftOptions {
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) == 2 && parts[0] == "naming_style" {
			NameStyle = styles.NewNamingStyle(parts[1])
			if NameStyle == nil {
				return fmt.Errorf(fmt.Sprintf("do not support \"%s\" naming style", parts[1]))
			}
			break
		}
	}

	return nil
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
	rs, err := NewResolver(ast, main, pkgMap)
	if err != nil {
		return nil, fmt.Errorf("new thrift resolver failed, err:%v", err)
	}
	err = rs.LoadAll(ast)
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
				field := model.Field{}
				err := injectTags(f, &field, true, false)
				if err != nil {
					return nil, err
				}
				tags := field.Tags
				var tagString string
				for idx, tag := range tags {
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
				field := model.Field{}
				err := injectTags(f, &field, true, false)
				if err != nil {
					return nil, err
				}
				tags := field.Tags
				var tagString string
				for idx, tag := range tags {
					if idx == 0 {
						tagString += " " + tag.Key + ":\"" + tag.Value + "\"" + " "
					} else if idx == len(tags)-1 {
						tagString += tag.Key + ":\"" + tag.Value + "\""
					} else {
						tagString += tag.Key + ":\"" + tag.Value + "\"" + " "
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

	return &thriftgo_plugin.Response{
		Contents: contents,
	}, nil
}
