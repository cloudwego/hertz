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
	"io/ioutil"
	"testing"

	"github.com/cloudwego/hertz/cmd/hz/generator"
	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/thriftgo/plugin"
)

func TestRun(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/request_thrift.out")
	if err != nil {
		t.Fatal(err)
	}

	req, err := plugin.UnmarshalRequest(data)
	if err != nil {
		t.Fatal(err)
	}

	plu := new(Plugin)
	plu.setLogger()

	plu.req = req

	_, err = plu.parseArgs()
	if err != nil {
		t.Fatal(err)
	}
	options := CheckTagOption(plu.args)

	pkgInfo, err := plu.getPackageInfo()
	if err != nil {
		t.Fatal(err)
	}

	args := plu.args
	cf, _ := util.GetColonPair(args.CustomizePackage)
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

	err = sg.Generate(pkgInfo)
	if err != nil {
		t.Fatalf("generate package failed: %v", err)
	}
	files, err := sg.GetFormatAndExcludedFiles()
	if err != nil {
		return
	}

	res, err := plu.GetResponse(files, sg.OutputDir)
	if err != nil {
		return
	}
	plu.response(res)
	if err != nil {
		return
	}
}
