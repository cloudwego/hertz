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
	"testing"

	"github.com/cloudwego/hertz/cmd/hz/config"
	"github.com/cloudwego/hertz/cmd/hz/generator"
	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/thriftgo/parser"
	thriftgo_plugin "github.com/cloudwego/thriftgo/plugin"
)

// buildTestRequest parses the test thrift IDL and builds a thriftgo plugin request.
func buildTestRequest(t *testing.T) *thriftgo_plugin.Request {
	t.Helper()
	idlPath := "../testdata/thrift/psm.thrift"
	ast, err := parser.ParseFile(idlPath, []string{"../testdata/thrift"}, true)
	if err != nil {
		t.Fatalf("parse thrift IDL failed: %v", err)
	}

	args := config.NewArgument()
	args.Gomod = "github.com/cloudwego/hertz/test"
	args.OutDir = t.TempDir()
	args.CmdType = meta.CmdNew
	packed, err := util.PackArgs(args)
	if err != nil {
		t.Fatalf("pack args failed: %v", err)
	}

	return &thriftgo_plugin.Request{
		Version:             "0.2.0",
		GeneratorParameters: []string{"go:reserve_comments,gen_json_tag=false,package_prefix=github.com/cloudwego/hertz/test/biz/model"},
		PluginParameters:    packed,
		Language:            "go",
		OutputPath:          args.OutDir,
		Recursive:           true,
		AST:                 ast,
	}
}

func TestRun(t *testing.T) {
	req := buildTestRequest(t)

	plu := new(Plugin)
	plu.setLogger()
	plu.req = req

	_, err := plu.parseArgs()
	if err != nil {
		t.Fatal(err)
	}
	options := CheckTagOption(plu.args)

	pkgInfo, err := plu.getPackageInfo()
	if err != nil {
		t.Fatal(err)
	}

	args := plu.args
	customPackageTemplate := args.CustomizePackage
	pkg, err := args.GetGoPackage()
	if err != nil {
		t.Fatal(err)
	}
	handlerDir, err := args.GetHandlerDir()
	if err != nil {
		t.Fatal(err)
	}
	routerDir, err := args.GetRouterDir()
	if err != nil {
		t.Fatal(err)
	}
	modelDir, err := args.GetModelDir()
	if err != nil {
		t.Fatal(err)
	}
	clientDir, err := args.GetClientDir()
	if err != nil {
		t.Fatal(err)
	}
	sg := generator.HttpPackageGenerator{
		ConfigPath: customPackageTemplate,
		HandlerDir: handlerDir,
		RouterDir:  routerDir,
		ModelDir:   modelDir,
		UseDir:     args.Use,
		ClientDir:  clientDir,
		TemplateGenerator: generator.TemplateGenerator{
			OutputDir: args.OutDir,
			Excludes:  args.Excludes,
		},
		ProjPackage:          pkg,
		Options:              options,
		HandlerByMethod:      args.HandlerByMethod,
		CmdType:              args.CmdType,
		ForceClientDir:       args.ForceClientDir,
		BaseDomain:           args.BaseDomain,
		QueryEnumAsInt:       args.QueryEnumAsInt,
		SnakeStyleMiddleware: args.SnakeStyleMiddleware,
		SortRouter:           args.SortRouter,
	}
	if args.ModelBackend != "" {
		sg.Backend = meta.Backend(args.ModelBackend)
	}

	err = sg.Generate(pkgInfo)
	if err != nil {
		t.Fatalf("generate package failed: %v", err)
	}
	files, err := sg.GetFormatAndExcludedFiles()
	if err != nil {
		t.Fatal(err)
	}

	res, err := plu.GetResponse(files, sg.OutputDir)
	if err != nil {
		t.Fatal(err)
	}
	plu.response(res)
}
