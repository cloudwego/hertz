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
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/internal/config"
	"github.com/cloudwego/hertz/cmd/hz/internal/generator"
	"github.com/cloudwego/hertz/cmd/hz/internal/meta"
	"github.com/cloudwego/hertz/cmd/hz/internal/util"
	"github.com/cloudwego/hertz/cmd/hz/internal/util/logs"
	"github.com/cloudwego/hertz/cmd/hz/pkg/argument"
	"github.com/urfave/cli/v2"
)

// global args. MUST fork it when use
var globalArgs = argument.NewArgument()

func New(c *cli.Context) error {
	args, err := globalArgs.Parse(c, meta.CmdNew)
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}
	setLogVerbose(args.Verbose)
	logs.Debugf("args: %#v\n", args)

	err = GenerateLayout(args)
	if err != nil {
		return cli.Exit(err, meta.GenerateLayoutError)
	}

	err = TriggerPlugin(args)
	if err != nil {
		return cli.Exit(err, meta.PluginError)
	}
	return nil
}

func Update(c *cli.Context) error {
	manifest := new(meta.Manifest)
	err := manifest.Validate(".")
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}

	// begin to update
	args, err := globalArgs.Parse(c, meta.CmdUpdate)
	if err != nil {
		return cli.Exit(err, meta.LoadError)
	}
	setLogVerbose(args.Verbose)
	logs.Debugf("Args: %#v\n", args)

	err = TriggerPlugin(args)
	if err != nil {
		return cli.Exit(err, meta.PluginError)
	}

	manifest.Version = meta.GoVersion
	err = manifest.Persist(".")
	if err != nil {
		return cli.Exit(fmt.Errorf("persist manifest failed: %v", err), meta.PersistError)
	}
	return nil
}

func Init() *cli.App {
	// flags
	verboseFlag := cli.BoolFlag{Name: "verbose,vv", Usage: "turn on verbose mode", Destination: &globalArgs.Verbose}

	idlFlag := cli.StringSliceFlag{Name: "idl", Usage: "Specify the IDL file path. (.thrift or .proto)"}
	moduleFlag := cli.StringFlag{Name: "module", Aliases: []string{"mod"}, Usage: "Specify the Go module name to generate go.mod.", Destination: &globalArgs.Gomod}
	serviceNameFlag := cli.StringFlag{Name: "service", Usage: "Specify the service name.", Destination: &globalArgs.ServiceName}
	outDirFlag := cli.StringFlag{Name: "out_dir", Usage: "Specify the project path.", Destination: &globalArgs.OutDir}
	handlerDirFlag := cli.StringFlag{Name: "handler_dir", Usage: "Specify the handler path.", Destination: &globalArgs.HandlerDir}
	modelDirFlag := cli.StringFlag{Name: "model_dir", Usage: "Specify the model path.", Destination: &globalArgs.ModelDir}
	clientDirFlag := cli.StringFlag{Name: "client_dir", Usage: "Specify the client path. If not specified, no client code is generated.", Destination: &globalArgs.ClientDir}

	optPkgFlag := cli.StringSliceFlag{Name: "option_package", Aliases: []string{"P"}, Usage: "Specify the package path. ({include_path}={import_path})"}
	includesFlag := cli.StringSliceFlag{Name: "proto_path", Aliases: []string{"I"}, Usage: "Add an IDL search path for includes. (Valid only if idl is protobuf)"}
	excludeFilesFlag := cli.StringSliceFlag{Name: "exclude_file", Aliases: []string{"E"}, Usage: "Specify the files that do not need to be updated."}
	thriftOptionsFlag := cli.StringSliceFlag{Name: "thriftgo", Aliases: []string{"t"}, Usage: "Specify arguments for the thriftgo. ({flag}={value})"}
	protoOptionsFlag := cli.StringSliceFlag{Name: "protoc", Aliases: []string{"p"}, Usage: "Specify arguments for the protoc. ({flag}={value})"}
	noRecurseFlag := cli.BoolFlag{Name: "no_recurse", Usage: "Generate master model only.", Destination: &globalArgs.NoRecurse}

	jsonEnumStrFlag := cli.BoolFlag{Name: "json_enumstr", Usage: "Use string instead of num for json enums when idl is thrift.", Destination: &globalArgs.JSONEnumStr}
	unsetOmitemptyFlag := cli.BoolFlag{Name: "unset_omitempty", Usage: "Remove 'omitempty' tag for generated struct.", Destination: &globalArgs.UnsetOmitempty}
	snakeNameFlag := cli.BoolFlag{Name: "snake_tag", Usage: "Use snake_case style naming for tags. (Only works for 'form', 'query', 'json')", Destination: &globalArgs.SnakeName}
	customLayout := cli.StringFlag{Name: "customize_layout", Usage: "Specify the layout template. ({{Template Profile}}:{{Rendering Data}})", Destination: &globalArgs.CustomizeLayout}
	customPackage := cli.StringFlag{Name: "customize_package", Usage: "Specify the package template. ({{Template Profile}}:)", Destination: &globalArgs.CustomizePackage}

	// app
	app := cli.NewApp()
	app.Name = "hz"
	app.Usage = "A idl parser and code generator for Hertz projects"
	app.Version = meta.Version

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
				&clientDirFlag,

				&includesFlag,
				&thriftOptionsFlag,
				&protoOptionsFlag,
				&optPkgFlag,
				&noRecurseFlag,

				&jsonEnumStrFlag,
				&unsetOmitemptyFlag,
				&snakeNameFlag,
				&excludeFilesFlag,
				&customLayout,
				&customPackage,
			},
			Action: New,
		},
		{
			Name:  meta.CmdUpdate,
			Usage: "Update an existing Hertz project",
			Flags: []cli.Flag{
				&idlFlag,
				&outDirFlag,
				&handlerDirFlag,
				&modelDirFlag,
				&clientDirFlag,

				&includesFlag,
				&thriftOptionsFlag,
				&protoOptionsFlag,
				&optPkgFlag,
				&noRecurseFlag,

				&jsonEnumStrFlag,
				&unsetOmitemptyFlag,
				&snakeNameFlag,
				&excludeFilesFlag,
				&customPackage,
			},
			Action: Update,
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

func GenerateLayout(args *argument.Argument) error {
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
	}

	if args.CustomizeLayout == "" {
		// generate by default
		err := lg.GenerateByService(layout)
		if err != nil {
			return fmt.Errorf("generating layout failed: %v", err)
		}
	} else {
		// generate by customized layout
		sp := strings.Split(args.CustomizeLayout, ":")
		if len(sp) != 2 || sp[0] == "" {
			return errors.New("custom_layout must be like: {{layout_config_path}}:{{template_data_config_path}}")
		}
		configPath, dataPath := sp[0], sp[1]
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

func TriggerPlugin(args *argument.Argument) error {
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
		return fmt.Errorf("plugin %s_gen_hertz returns error: %v, cause:\n%v", compiler, err, string(buf))
	}

	// If len(buf) != 0, the plugin returned the log.
	if len(buf) != 0 {
		fmt.Println(string(buf))
	}
	logs.Debugf("end run plugin %s_gen_hertz", compiler)
	return nil
}
