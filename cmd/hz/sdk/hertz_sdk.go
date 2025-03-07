package sdk

import (
	"bytes"
	"fmt"
	"github.com/cloudwego/hertz/cmd/hz/config"
	"github.com/cloudwego/hertz/cmd/hz/thrift"
	"github.com/cloudwego/thriftgo/plugin"
	"os/exec"
	"strings"
)

func RunHertzTool(wd string, plugins []plugin.SDKPlugin, kitexArgs ...string) error {
	return nil
}

type HertzSDKPlugin struct {
	HertzParams    []string
	ThriftgoParams []string
	Pwd            string
}

func (k *HertzSDKPlugin) Invoke(req *plugin.Request) (res *plugin.Response) {
	r := thrift.Plugin{}
	return r.HandleRequest(&config.Argument{}, req)
}

func (k *HertzSDKPlugin) GetName() string {
	return "kitex"
}

func (k *HertzSDKPlugin) GetPluginParameters() []string {
	return k.HertzParams
}

func (k *HertzSDKPlugin) GetThriftgoParameters() []string {
	return k.ThriftgoParams
}

func GetHertzSDKPlugin(pwd string, rawHertzArgs []string) (*HertzSDKPlugin, error) {
	// run as kitex
	//err := args.ParseArgs(kitex.Version, pwd, rawHertzArgs)
	//if err != nil {
	//	return nil, err
	//}

	out := new(bytes.Buffer)
	cmd, err := args.BuildCmd(out)
	if err != nil {
		return nil, err
	}

	hertzPlugin := &HertzSDKPlugin{}

	hertzPlugin.ThriftgoParams, hertzPlugin.HertzParams, err:= ParseHertzCmd(cmd)
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
	return thriftgoParams,hertzParams, nil
}
