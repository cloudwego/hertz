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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/config"
	"github.com/cloudwego/hertz/cmd/hz/generator"
	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/hertz/cmd/hz/util/logs"
	"github.com/cloudwego/thriftgo/generator/backend"
	"github.com/cloudwego/thriftgo/generator/golang"
	"github.com/cloudwego/thriftgo/generator/golang/styles"
	"github.com/cloudwego/thriftgo/parser"
	thriftgo_plugin "github.com/cloudwego/thriftgo/plugin"
)

type Plugin struct {
	req    *thriftgo_plugin.Request
	args   *config.Argument
	logger *logs.StdLogger
	rmTags []string
}

func (plugin *Plugin) Run() int {
	plugin.setLogger()
	args := &config.Argument{}
	defer func() {
		if args == nil {
			return
		}
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
	plugin.rmTags = args.RmTags
	if args.CmdType == meta.CmdModel {
		// check tag options for model mode
		CheckTagOption(plugin.args)
		res, err := plugin.GetResponse(nil, args.OutDir)
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

	err = plugin.initNameStyle()
	if err != nil {
		logs.Errorf("init naming style failed: %s", err.Error())
		return meta.PluginError
	}

	options := CheckTagOption(plugin.args)

	pkgInfo, err := plugin.getPackageInfo()
	if err != nil {
		logs.Errorf("get http package info failed: %s", err.Error())
		return meta.PluginError
	}

	customPackageTemplate := args.CustomizePackage
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
		GenBizService:        args.GenBizService,
		CmdType:              args.CmdType,
		IdlClientDir:         util.SubDir(modelDir, pkgInfo.Package),
		ForceClientDir:       args.ForceClientDir,
		BaseDomain:           args.BaseDomain,
		QueryEnumAsInt:       args.QueryEnumAsInt,
		SnakeStyleMiddleware: args.SnakeStyleMiddleware,
		SortRouter:           args.SortRouter,
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
	if len(args.Use) != 0 {
		err = sg.Persist()
		if err != nil {
			logs.Errorf("persist file failed within '-use' option: %s", err.Error())
			return meta.PluginError
		}
		res := thriftgo_plugin.BuildErrorResponse(errors.New(meta.TheUseOptionMessage).Error())
		err = plugin.response(res)
		if err != nil {
			logs.Errorf("response failed: %s", err.Error())
			return meta.PluginError
		}
		return 0
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
	err = plugin.response(res)
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

func getReq() *thriftgo_plugin.Request {
	a := &thriftgo_plugin.Request{}
	_ = json.Unmarshal([]byte(`{"Version":"0.3.12","GeneratorParameters":["reserve_comments=","gen_json_tag=false","package_prefix=code.byted.org/life_service/craftsman_merchant_api/biz/model"],"PluginParameters":["CmdType=update","Cwd=/Users/bytedance/GolandProjects/craftsman_merchant_api","OutDir=/Users/bytedance/GolandProjects/craftsman_merchant_api","IdlType=thrift","IdlPaths=/Users/bytedance/GolandProjects/life_idl/hermes/craftsman/craftsman_merchant_api.thrift","Gopath=/Users/bytedance/GolandProjects","Gosrc=/Users/bytedance/GolandProjects/src","Gomod=code.byted.org/life_service/craftsman_merchant_api","HandlerByMethod=false","CustomizePackage=/Users/bytedance/GolandProjects/hertztool/v3/template/packageTpl/package_v2.yaml"],"Language":"go","OutputPath":"/Users/bytedance/GolandProjects/craftsman_merchant_api/biz/model","Recursive":true,"AST":{"Filename":"../life_idl/hermes/craftsman/craftsman_merchant_api.thrift","Includes":[{"Path":"../../base.thrift","Reference":{"Filename":"../life_idl/base.thrift","Includes":null,"CppIncludes":null,"Namespaces":[{"Language":"py","Name":"base","Annotations":null},{"Language":"go","Name":"base","Annotations":null},{"Language":"java","Name":"com.bytedance.thrift.base","Annotations":null}],"Typedefs":null,"Constants":null,"Enums":null,"Structs":[{"Category":"struct","Name":"TrafficEnv","Fields":[{"ID":1,"Name":"Open","Requiredness":0,"Type":{"Name":"bool","CppType":"","Annotations":null,"Category":1},"Default":{"Type":3,"TypedValue":{"Identifier":"false"}},"Annotations":null,"ReservedComments":""},{"ID":2,"Name":"Env","Requiredness":0,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Default":{"Type":2,"TypedValue":{"Literal":""}},"Annotations":null,"ReservedComments":""}],"Annotations":null,"ReservedComments":""},{"Category":"struct","Name":"Base","Fields":[{"ID":1,"Name":"LogID","Requiredness":0,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Default":{"Type":2,"TypedValue":{"Literal":""}},"Annotations":null,"ReservedComments":""},{"ID":2,"Name":"Caller","Requiredness":0,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Default":{"Type":2,"TypedValue":{"Literal":""}},"Annotations":null,"ReservedComments":""},{"ID":3,"Name":"Addr","Requiredness":0,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Default":{"Type":2,"TypedValue":{"Literal":""}},"Annotations":null,"ReservedComments":""},{"ID":4,"Name":"Client","Requiredness":0,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Default":{"Type":2,"TypedValue":{"Literal":""}},"Annotations":null,"ReservedComments":""},{"ID":5,"Name":"TrafficEnv","Requiredness":2,"Type":{"Name":"TrafficEnv","CppType":"","Annotations":null,"Category":13},"Annotations":null,"ReservedComments":""},{"ID":6,"Name":"Extra","Requiredness":2,"Type":{"Name":"map","KeyType":{"Name":"string","CppType":"","Annotations":null,"Category":7},"ValueType":{"Name":"string","CppType":"","Annotations":null,"Category":7},"CppType":"","Annotations":null,"Category":9},"Annotations":null,"ReservedComments":""}],"Annotations":null,"ReservedComments":""},{"Category":"struct","Name":"BaseResp","Fields":[{"ID":1,"Name":"StatusMessage","Requiredness":0,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Default":{"Type":2,"TypedValue":{"Literal":""}},"Annotations":null,"ReservedComments":""},{"ID":2,"Name":"StatusCode","Requiredness":0,"Type":{"Name":"i32","CppType":"","Annotations":null,"Category":4},"Default":{"Type":1,"TypedValue":{"Int":0}},"Annotations":null,"ReservedComments":""},{"ID":3,"Name":"Extra","Requiredness":2,"Type":{"Name":"map","KeyType":{"Name":"string","CppType":"","Annotations":null,"Category":7},"ValueType":{"Name":"string","CppType":"","Annotations":null,"Category":7},"CppType":"","Annotations":null,"Category":9},"Annotations":null,"ReservedComments":""}],"Annotations":null,"ReservedComments":""}],"Unions":null,"Exceptions":null,"Services":null,"Name2Category":{"BaseResp":13,"TrafficEnv":13,"Base":13}},"Used":true}],"CppIncludes":null,"Namespaces":[{"Language":"go","Name":"life.hermes.craftsman_merchant_api","Annotations":null}],"Typedefs":null,"Constants":null,"Enums":null,"Structs":[{"Category":"struct","Name":"CreateOrderRequest","Fields":[{"ID":1,"Name":"shopping_car_id","Requiredness":1,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":"//poi列表id"},{"ID":255,"Name":"Base","Requiredness":2,"Type":{"Name":"base.Base","CppType":"","Annotaons":null,"Category":13,"Reference":{"Name":"Base","Index":0}},"Annotations":null,"ReservedComments":""}],"Annotations":null,"ReservedComments":""},{"Category":"struct","Name":"CreateOrderResponse","Fields":[{"ID":1,"Name":"data","Requiredness":2,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":""},{"ID":250,"Name":"status_code","Requiredness":1,"Type":{"Name":"i32","CppType":"","Annotations":null,"Category":4},"Annotations":null,"ReservedComments":""},{"ID":251,"Name":"status_msg","Requiredness":1,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":""},{"ID":252,"Name":"log_id","Requiredness":1,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":""},{"ID":253,"Name":"now","Requiredness":1,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":""},{"ID":255,"Name":"BaseResp","Requiredness":0,"Type":{"Name":"base.BaseResp","CppType":"","Annotations":null,"Category":13,"Reference":{"Name":"BaseResp","Index":0}},"Annotations":null,"ReservedComments":""}],"Annotations":null,"ReservedComments":""},{"Category":"struct","Name":"CreateOrderRequest1","Fields":[{"ID":1,"Name":"shopping_car_id","Requiredness":1,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":"//poi列id"},{"ID":255,"Name":"Base","Requiredness":2,"Type":{"Name":"base.Base","CppType":"","Annotations":null,"Category":13,"Reference":{"Name":"Base","Index":0}},"Annotations":null,"ReservedComments":""}],"Annotations":null,"ReservedComments":""},{"Category":"struct","Name":"CreateOrderResponse1","Fields":[{"ID":1,"Name":"data","Requiredness":2,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":""},{"ID":250,"Name":"status_code","Requiredness":1,"Type":{"Name":"i32","CppType":"","Annotations":null,"Category":4},"Annotations":null,"ReservedComments":""},{"ID":251,"Name":"status_msg","Requiredness":1,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":""},{"ID":252,"Name":"log_id","Requiredness":1,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":""},{"ID":253,"Name":"now","Requiredness":1,"Type":{"Name":"string","CppType":"","Annotations":null,"Category":7},"Annotations":null,"ReservedComments":""},{"ID":255,"Name":"BaseResp","Requiredness":0,"Type":{"Name":"base.BaseResp","CppType":"","Annotations":null,"Category":13,"Reference":{"Name":"BaseResp","Index":0}},"Annotations":null,"ReservedComments":""}],"Annotations":null,"ReservedComments":""}],"Unions":null,"Exceptions":null,"Services":[{"Name":"CraftsmanMerchantService","Extends":"","Functions":[{"Name":"CreateOrder","Oneway":false,"Void":false,"FunctionType":{"Name":"CreateOrderResponse","CppType":"","Annotations":null,"Category":13},"Arguments":[{"ID":1,"Name":"request","Requiredness":0,"Type":{"Name":"CreateOrderRequest","CppType":"","Annotations":null,"Category":13},"Annotations":null,"ReservedComments":""}],"Throws":null,"Annotations":[{"Key":"api.post","Values":["/life/comprehensive/v1/increase_value/pay/create_order"]},{"Key":"biz.service_file","Values":["biz/service/order1_service/order1_service.go"]}],"ReservedComments":"//创建订单"},{"Name":"CreateOrder2","Oneway":false,"Void":false,"FunctionType":{"Name":"CreateOrderResponse1","CppType":"","Antions":null,"Category":13},"Arguments":[{"ID":1,"Name":"request","Requiredness":0,"Type":{"Name":"CreateOrderRequest1","CppType":"","Annotations":null,"Category":13},"Annotations":null,"ReservedComments":""}],"Throws":null,"Annotations":[{"Key":"api.post","Values":["/life/comprehensive/v1/increase_value/pay/create_order2"]},{"Key":"biz.service_file","Values":["biz/service/order2_service/order2_service.go"]}],"ReservedComments":"//创建订单2"},{"Name":"CreateOrder3","Oneway":false,"":false,"FunctionType":{"Name":"CreateOrderResponse","CppType":"","Annotations":null,"Category":13},"Arguments":[{"ID":1,"Name":"request","Requiredness":0,"Type":{"Name":"CreateOrderRequest","CppType":"","Annotations":null,"Category":13},"Annotations":null,"ReservedComments":""}],"Throws":null,"Annotations":[{"Key":"api.post","Values":["/life/comprehensive/v1/increase_value/pay/create_order3"]},{"Key":"biz.service_file","Values":["biz/service/order2_service/order2_service.go"]}],"ReservedComments":"//创建订单3"}],"Annotations":[{"Key":"api.service_gen_dir","Values":["biz/handler"]}],"ReservedComments":""}],"Name2Category":{"CreateOrderReq1":13,"CreateOrderResponse1":13,"CraftsmanMerchantService":17,"CreateOrderRequest":13,"CreateOrderResponse":13}}}`), a)
	return a
}

func (plugin *Plugin) handleRequest() error {
	var req *thriftgo_plugin.Request
	if os.Getenv("test") == "1" {
		req = getReq()
	} else {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read request failed: %s", err.Error())
		}
		req, err = thriftgo_plugin.UnmarshalRequest(data)
		if err != nil {
			return fmt.Errorf("unmarshal request failed: %s", err.Error())
		}
	}
	plugin.req = req
	// init thriftgo utils
	thriftgoUtil = golang.NewCodeUtils(backend.DummyLogFunc())
	thriftgoUtil.HandleOptions(req.GeneratorParameters)

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

	services, err := astToService(ast, rs, args)
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
				tagString, err := getTagString(f, plugin.rmTags)
				if err != nil {
					return nil, err
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
				tagString, err := getTagString(f, plugin.rmTags)
				if err != nil {
					return nil, err
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

func getTagString(f *parser.Field, rmTags []string) (string, error) {
	field := model.Field{}
	err := injectTags(f, &field, true, false)
	if err != nil {
		return "", err
	}
	disableTag := false
	if v := getAnnotation(f.Annotations, AnnotationNone); len(v) > 0 {
		if strings.EqualFold(v[0], "true") {
			disableTag = true
		}
	}

	for _, rmTag := range rmTags {
		for _, t := range field.Tags {
			if t.IsDefault && strings.EqualFold(t.Key, rmTag) {
				field.Tags.Remove(t.Key)
			}
		}
	}

	var tagString string
	tags := field.Tags
	for idx, tag := range tags {
		value := tag.Value
		if disableTag {
			value = "-"
		}
		if idx == 0 {
			tagString += " " + tag.Key + ":\"" + value + "\"" + " "
		} else if idx == len(tags)-1 {
			tagString += tag.Key + ":\"" + value + "\""
		} else {
			tagString += tag.Key + ":\"" + value + "\"" + " "
		}
	}

	return tagString, nil
}
