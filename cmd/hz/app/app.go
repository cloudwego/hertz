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

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/config"
	"github.com/cloudwego/hertz/cmd/hz/generator"
	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/protobuf"
	"github.com/cloudwego/hertz/cmd/hz/thrift"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/hertz/cmd/hz/util/logs"
	"github.com/urfave/cli/v2"
)

// global args. MUST fork it when use
var globalArgs = config.NewArgument()

func New(c *cli.Context) error {
	args, err := globalArgs.Parse(c, meta.CmdNew)
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}
	setLogVerbose(args.Verbose)
	logs.Debugf("args: %#v\n", args)

	exist, err := util.PathExist(filepath.Join(args.OutDir, meta.ManifestFile))
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}

	if exist && !args.ForceNew {
		return cli.Exit(fmt.Errorf("the current is already a hertz project, if you want to regenerate it you can specify \"-force\""), meta.LoadError)
	}

	err = GenerateLayout(args)
	if err != nil {
		return cli.Exit(err, meta.GenerateLayoutError)
	}

	err = TriggerPlugin(args)
	if err != nil {
		return cli.Exit(err, meta.PluginError)
	}
	// ".hz" file converges to the hz tool
	manifest := new(meta.Manifest)
	args.InitManifest(manifest)
	err = manifest.Persist(args.OutDir)
	if err != nil {
		return cli.Exit(fmt.Errorf("persist manifest failed: %v", err), meta.PersistError)
	}
	if !args.NeedGoMod && args.IdlType == meta.IdlThrift {
		logs.Warn(meta.AddThriftReplace)
	}

	return nil
}

func Update(c *cli.Context) error {
	// begin to update
	args, err := globalArgs.Parse(c, meta.CmdUpdate)
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}
	setLogVerbose(args.Verbose)
	logs.Debugf("Args: %#v\n", args)

	manifest := new(meta.Manifest)
	err = manifest.InitAndValidate(args.OutDir)
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}
	// update argument by ".hz", can automatically get "handler_dir"/"model_dir"/"router_dir"
	args.UpdateByManifest(manifest)

	err = TriggerPlugin(args)
	if err != nil {
		return cli.Exit(err, meta.PluginError)
	}
	// If the "handler_dir"/"model_dir" is updated, write it back to ".hz"
	args.UpdateManifest(manifest)
	err = manifest.Persist(args.OutDir)
	if err != nil {
		return cli.Exit(fmt.Errorf("persist manifest failed: %v", err), meta.PersistError)
	}

	return nil
}

func Model(c *cli.Context) error {
	args, err := globalArgs.Parse(c, meta.CmdModel)
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}
	setLogVerbose(args.Verbose)
	logs.Debugf("Args: %#v\n", args)

	err = TriggerPlugin(args)
	if err != nil {
		return cli.Exit(err, meta.PluginError)
	}

	return nil
}

func Client(c *cli.Context) error {
	args, err := globalArgs.Parse(c, meta.CmdClient)
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}
	setLogVerbose(args.Verbose)
	logs.Debugf("Args: %#v\n", args)

	err = TriggerPlugin(args)
	if err != nil {
		return cli.Exit(err, meta.PluginError)
	}

	return nil
}

func PluginMode() {
	mode := os.Getenv(meta.EnvPluginMode)
	if len(os.Args) <= 1 && mode != "" {
		switch mode {
		case meta.ThriftPluginName:
			plugin := new(thrift.Plugin)
			os.Exit(plugin.Run())
		case meta.ProtocPluginName:
			plugin := new(protobuf.Plugin)
			os.Exit(plugin.Run())
		}
	}
}

func Init() *cli.App {
	// flags
	verboseFlag := cli.BoolFlag{Name: "verbose,vv", Usage: "turn on verbose mode", Destination: &globalArgs.Verbose}

	idlFlag := cli.StringSliceFlag{Name: "idl", Usage: "Specify the IDL file path. (.thrift or .proto)"}
	moduleFlag := cli.StringFlag{Name: "module", Aliases: []string{"mod"}, Usage: "Specify the Go module name.", Destination: &globalArgs.Gomod}
	serviceNameFlag := cli.StringFlag{Name: "service", Usage: "Specify the service name.", Destination: &globalArgs.ServiceName}
	outDirFlag := cli.StringFlag{Name: "out_dir", Usage: "Specify the project path.", Destination: &globalArgs.OutDir}
	handlerDirFlag := cli.StringFlag{Name: "handler_dir", Usage: "Specify the handler relative path (based on \"out_dir\").", Destination: &globalArgs.HandlerDir}
	modelDirFlag := cli.StringFlag{Name: "model_dir", Usage: "Specify the model relative path (based on \"out_dir\").", Destination: &globalArgs.ModelDir}
	routerDirFlag := cli.StringFlag{Name: "router_dir", Usage: "Specify the router relative path (based on \"out_dir\").", Destination: &globalArgs.RouterDir}
	useFlag := cli.StringFlag{Name: "use", Usage: "Specify the model package to import for handler.", Destination: &globalArgs.Use}
	baseDomainFlag := cli.StringFlag{Name: "base_domain", Usage: "Specify the request domain.", Destination: &globalArgs.BaseDomain}
	clientDirFlag := cli.StringFlag{Name: "client_dir", Usage: "Specify the client path. If not specified, IDL generated path is used for 'client' command; no client code is generated for 'new' command", Destination: &globalArgs.ClientDir}
	forceClientDirFlag := cli.StringFlag{Name: "force_client_dir", Usage: "Specify the client path, and won't use namespaces as subpaths", Destination: &globalArgs.ForceClientDir}

	optPkgFlag := cli.StringSliceFlag{Name: "option_package", Aliases: []string{"P"}, Usage: "Specify the package path. ({include_path}={import_path})"}
	includesFlag := cli.StringSliceFlag{Name: "proto_path", Aliases: []string{"I"}, Usage: "Add an IDL search path for includes. (Valid only if idl is protobuf)"}
	excludeFilesFlag := cli.StringSliceFlag{Name: "exclude_file", Aliases: []string{"E"}, Usage: "Specify the files that do not need to be updated."}
	thriftOptionsFlag := cli.StringSliceFlag{Name: "thriftgo", Aliases: []string{"t"}, Usage: "Specify arguments for the thriftgo. ({flag}={value})"}
	protoOptionsFlag := cli.StringSliceFlag{Name: "protoc", Aliases: []string{"p"}, Usage: "Specify arguments for the protoc. ({flag}={value})"}
	thriftPluginsFlag := cli.StringSliceFlag{Name: "thrift-plugins", Usage: "Specify plugins for the thriftgo. ({plugin_name}:{options})"}
	protoPluginsFlag := cli.StringSliceFlag{Name: "protoc-plugins", Usage: "Specify plugins for the protoc. ({plugin_name}:{options}:{out_dir})"}
	noRecurseFlag := cli.BoolFlag{Name: "no_recurse", Usage: "Generate master model only.", Destination: &globalArgs.NoRecurse}
	forceNewFlag := cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Force new a project, which will overwrite the generated files", Destination: &globalArgs.ForceNew}
	forceUpdateClientFlag := cli.BoolFlag{Name: "force_client", Usage: "Force update 'hertz_client.go'", Destination: &globalArgs.ForceUpdateClient}
	enableExtendsFlag := cli.BoolFlag{Name: "enable_extends", Usage: "Parse 'extends' for thrift IDL", Destination: &globalArgs.EnableExtends}
	sortRouterFlag := cli.BoolFlag{Name: "sort_router", Usage: "Sort router register code, to avoid code difference", Destination: &globalArgs.SortRouter}

	jsonEnumStrFlag := cli.BoolFlag{Name: "json_enumstr", Usage: "Use string instead of num for json enums when idl is thrift.", Destination: &globalArgs.JSONEnumStr}
	queryEnumIntFlag := cli.BoolFlag{Name: "query_enumint", Usage: "Use num instead of string for query enum parameter.", Destination: &globalArgs.QueryEnumAsInt}
	unsetOmitemptyFlag := cli.BoolFlag{Name: "unset_omitempty", Usage: "Remove 'omitempty' tag for generated struct.", Destination: &globalArgs.UnsetOmitempty}
	protoCamelJSONTag := cli.BoolFlag{Name: "pb_camel_json_tag", Usage: "Convert Name style for json tag to camel(Only works protobuf).", Destination: &globalArgs.ProtobufCamelJSONTag}
	snakeNameFlag := cli.BoolFlag{Name: "snake_tag", Usage: "Use snake_case style naming for tags. (Only works for 'form', 'query', 'json')", Destination: &globalArgs.SnakeName}
	rmTagFlag := cli.StringSliceFlag{Name: "rm_tag", Usage: "Remove the default tag(json/query/form). If the annotation tag is set explicitly, it will not be removed."}
	customLayout := cli.StringFlag{Name: "customize_layout", Usage: "Specify the path for layout template.", Destination: &globalArgs.CustomizeLayout}
	customLayoutData := cli.StringFlag{Name: "customize_layout_data_path", Usage: "Specify the path for layout template render data.", Destination: &globalArgs.CustomizeLayoutData}
	customPackage := cli.StringFlag{Name: "customize_package", Usage: "Specify the path for package template.", Destination: &globalArgs.CustomizePackage}
	handlerByMethod := cli.BoolFlag{Name: "handler_by_method", Usage: "Generate a separate handler file for each method.", Destination: &globalArgs.HandlerByMethod}
	trimGoPackage := cli.StringFlag{Name: "trim_gopackage", Aliases: []string{"trim_pkg"}, Usage: "Trim the prefix of go_package for protobuf.", Destination: &globalArgs.TrimGoPackage}

	// client flag
	enableClientOptionalFlag := cli.BoolFlag{Name: "enable_optional", Usage: "Optional field do not transfer for thrift if not set.(Only works for query tag)", Destination: &globalArgs.EnableClientOptional}

	// app
	app := cli.NewApp()
	app.Name = "hz"
	app.Usage = "A idl parser and code generator for Hertz projects"
	app.Version = meta.Version
	// The default separator for multiple parameters is modified to ";"
	app.SliceFlagSeparator = ";"

	// global flags
	app.Flags = []cli.Flag{
		&verboseFlag,
	}

	// Commands
	app.Commands = []*cli.Command{
		{
			Name:  meta.CmdNew,
			Usage: "Generate a new Hertz project",
			Flags: []cli.Flag{
				&idlFlag,
				&serviceNameFlag,
				&moduleFlag,
				&outDirFlag,
				&handlerDirFlag,
				&modelDirFlag,
				&routerDirFlag,
				&clientDirFlag,
				&useFlag,

				&includesFlag,
				&thriftOptionsFlag,
				&protoOptionsFlag,
				&optPkgFlag,
				&trimGoPackage,
				&noRecurseFlag,
				&forceNewFlag,
				&enableExtendsFlag,
				&sortRouterFlag,

				&jsonEnumStrFlag,
				&unsetOmitemptyFlag,
				&protoCamelJSONTag,
				&snakeNameFlag,
				&rmTagFlag,
				&excludeFilesFlag,
				&customLayout,
				&customLayoutData,
				&customPackage,
				&handlerByMethod,
				&protoPluginsFlag,
				&thriftPluginsFlag,
			},
			Action: New,
		},
		{
			Name:  meta.CmdUpdate,
			Usage: "Update an existing Hertz project",
			Flags: []cli.Flag{
				&idlFlag,
				&moduleFlag,
				&outDirFlag,
				&handlerDirFlag,
				&modelDirFlag,
				&clientDirFlag,
				&useFlag,

				&includesFlag,
				&thriftOptionsFlag,
				&protoOptionsFlag,
				&optPkgFlag,
				&trimGoPackage,
				&noRecurseFlag,
				&enableExtendsFlag,
				&sortRouterFlag,

				&jsonEnumStrFlag,
				&unsetOmitemptyFlag,
				&protoCamelJSONTag,
				&snakeNameFlag,
				&rmTagFlag,
				&excludeFilesFlag,
				&customPackage,
				&handlerByMethod,
				&protoPluginsFlag,
				&thriftPluginsFlag,
			},
			Action: Update,
		},
		{
			Name:  meta.CmdModel,
			Usage: "Generate model code only",
			Flags: []cli.Flag{
				&idlFlag,
				&moduleFlag,
				&outDirFlag,
				&modelDirFlag,

				&includesFlag,
				&thriftOptionsFlag,
				&protoOptionsFlag,
				&noRecurseFlag,
				&trimGoPackage,

				&jsonEnumStrFlag,
				&unsetOmitemptyFlag,
				&protoCamelJSONTag,
				&snakeNameFlag,
				&rmTagFlag,
				&excludeFilesFlag,
			},
			Action: Model,
		},
		{
			Name:  meta.CmdClient,
			Usage: "Generate hertz client based on IDL",
			Flags: []cli.Flag{
				&idlFlag,
				&moduleFlag,
				&baseDomainFlag,
				&modelDirFlag,
				&clientDirFlag,
				&useFlag,
				&forceClientDirFlag,
				&forceUpdateClientFlag,

				&includesFlag,
				&thriftOptionsFlag,
				&protoOptionsFlag,
				&noRecurseFlag,
				&enableExtendsFlag,
				&trimGoPackage,

				&jsonEnumStrFlag,
				&enableClientOptionalFlag,
				&queryEnumIntFlag,
				&unsetOmitemptyFlag,
				&protoCamelJSONTag,
				&snakeNameFlag,
				&rmTagFlag,
				&excludeFilesFlag,
				&customPackage,
				&protoPluginsFlag,
				&thriftPluginsFlag,
			},
			Action: Client,
		},
	}
	return app
}

func setLogVerbose(verbose bool) {
	if verbose {
		logs.SetLevel(logs.LevelDebug)
	} else {
		logs.SetLevel(logs.LevelWarn)
	}
}

func GenerateLayout(args *config.Argument) error {
	lg := &generator.LayoutGenerator{
		TemplateGenerator: generator.TemplateGenerator{
			OutputDir: args.OutDir,
			Excludes:  args.Excludes,
		},
	}

	layout := generator.Layout{
		GoModule:        args.Gomod,
		ServiceName:     args.ServiceName,
		UseApacheThrift: args.IdlType == meta.IdlThrift,
		HasIdl:          0 != len(args.IdlPaths),
		ModelDir:        args.ModelDir,
		HandlerDir:      args.HandlerDir,
		RouterDir:       args.RouterDir,
		NeedGoMod:       args.NeedGoMod,
	}

	if args.CustomizeLayout == "" {
		// generate by default
		err := lg.GenerateByService(layout)
		if err != nil {
			return fmt.Errorf("generating layout failed: %v", err)
		}
	} else {
		// generate by customized layout
		configPath, dataPath := args.CustomizeLayout, args.CustomizeLayoutData
		logs.Infof("get customized layout info, layout_config_path: %s, template_data_path: %s", configPath, dataPath)
		exist, err := util.PathExist(configPath)
		if err != nil {
			return fmt.Errorf("check customized layout config file exist failed: %v", err)
		}
		if !exist {
			return errors.New("layout_config_path doesn't exist")
		}
		lg.ConfigPath = configPath
		// generate by service info
		if dataPath == "" {
			err := lg.GenerateByService(layout)
			if err != nil {
				return fmt.Errorf("generating layout failed: %v", err)
			}
		} else {
			// generate by customized data
			err := lg.GenerateByConfig(dataPath)
			if err != nil {
				return fmt.Errorf("generating layout failed: %v", err)
			}
		}
	}

	err := lg.Persist()
	if err != nil {
		return fmt.Errorf("generating layout failed: %v", err)
	}
	return nil
}

func TriggerPlugin(args *config.Argument) error {
	if len(args.IdlPaths) == 0 {
		return nil
	}
	cmd, err := config.BuildPluginCmd(args)
	if err != nil {
		return fmt.Errorf("build plugin command failed: %v", err)
	}

	compiler, err := config.IdlTypeToCompiler(args.IdlType)
	if err != nil {
		return fmt.Errorf("get compiler failed: %v", err)
	}

	logs.Debugf("begin to trigger plugin, compiler: %s, idl_paths: %v", compiler, args.IdlPaths)
	buf, err := cmd.CombinedOutput()
	if err != nil {
		out := strings.TrimSpace(string(buf))
		if !strings.HasSuffix(out, meta.TheUseOptionMessage) {
			return fmt.Errorf("plugin %s_gen_hertz returns error: %v, cause:\n%v", compiler, err, string(buf))
		}
	}

	// If len(buf) != 0, the plugin returned the log.
	if len(buf) != 0 {
		fmt.Println(string(buf))
	}
	logs.Debugf("end run plugin %s_gen_hertz", compiler)
	return nil
}
