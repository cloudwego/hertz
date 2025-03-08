// Copyright 2025 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sdk

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/app"
	"github.com/cloudwego/hertz/cmd/hz/config"
	"github.com/cloudwego/hertz/cmd/hz/thrift"
	"github.com/cloudwego/thriftgo/plugin"
	"github.com/cloudwego/thriftgo/sdk"
	"github.com/urfave/cli/v2"
)

func RunHertzTool(wd, cmdType string, plugins []plugin.SDKPlugin, hertzArgs ...string) error {
	hertzPlugin, err := GetHertzSDKPlugin(wd, cmdType, hertzArgs)
	if err != nil {
		return err
	}
	s := []plugin.SDKPlugin{hertzPlugin}
	s = append(s, plugins...)

	return sdk.RunThriftgoAsSDK(wd, s, hertzPlugin.GetThriftgoParameters()...)
}

type HertzSDKPlugin struct {
	HertzParams    []string
	ThriftgoParams []string
	Pwd            string
	Args           *config.Argument
}
}

func (k *HertzSDKPlugin) Invoke(req *plugin.Request) (res *plugin.Response) {
	r := thrift.Plugin{}
	return r.HandleRequest(k.Args, req)
}

func (k *HertzSDKPlugin) GetName() string {
	return "hz"
}

func (k *HertzSDKPlugin) GetPluginParameters() []string {
	return k.HertzParams
}

func (k *HertzSDKPlugin) GetThriftgoParameters() []string {
	return k.ThriftgoParams
}

func findCommon(client *cli.App, cmdType string) *cli.Command {
	for _, cmd := range client.Commands {
		if cmd.Name == cmdType {
			return cmd
		}
	}
	return nil
}

func GetHertzSDKPlugin(pwd, cmdType string, rawHertzArgs []string) (*HertzSDKPlugin, error) {
	client := app.Init()

	c := findCommon(client, cmdType)
	if c == nil {
		return nil, fmt.Errorf("command not found: %s", cmdType)
	}

	flagSet := flag.NewFlagSet("hz-parse", flag.ContinueOnError)

	for _, f := range c.Flags {
		if err := f.Apply(flagSet); err != nil {
			return nil, err
		}
	}

	if err := flagSet.Parse(rawHertzArgs); err != nil {
		return nil, err
	}

	ctx := cli.NewContext(client, flagSet, nil)

	args, err := app.GetGlobalArgs().Parse(ctx, cmdType)
	if err != nil {
		return nil, err
	}

	cmd, err := config.BuildPluginCmd(args)
	if err != nil {
		return nil, err
	}

	hertzPlugin := &HertzSDKPlugin{}

	hertzPlugin.ThriftgoParams, hertzPlugin.HertzParams, err = ParseHertzCmd(cmd)
	hertzPlugin.Args = args

	if err != nil {
		return nil, err
	}
	hertzPlugin.Pwd = pwd

	return hertzPlugin, nil
}

func ParseHertzCmd(cmd *exec.Cmd) (thriftgoParams, hertzParams []string, err error) {
	cmdArgs := cmd.Args
	// thriftgo -r -o kitex_gen -g go:xxx -p kitex=xxxx -p otherplugin xxx.thrift
	// ignore first argument, and remove -p kitex=xxxx

	thriftgoParams = []string{}
	hertzParams = []string{}
	if len(cmdArgs) < 1 {
		return nil, nil, fmt.Errorf("cmd args too short: %s", cmdArgs)
	}

	for i := 1; i < len(cmdArgs); i++ {
		arg := cmdArgs[i]
		if arg == "-p" && i+1 < len(cmdArgs) {
			pluginArgs := cmdArgs[i+1]
			if strings.HasPrefix(pluginArgs, "hertz") {
				hertzParams = strings.Split(pluginArgs, ",")
				i++
				continue
			}
		}
		thriftgoParams = append(thriftgoParams, arg)
	}
	return thriftgoParams, hertzParams, nil
}
