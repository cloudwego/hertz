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

package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/hertz/cmd/hz/util/logs"
)

func lookupTool(idlType string) (string, error) {
	tool := meta.TpCompilerThrift
	if idlType == meta.IdlProto {
		tool = meta.TpCompilerProto
	}

	path, err := exec.LookPath(tool)
	logs.Debugf("[DEBUG]path:%v", path)
	if err != nil {
		goPath, err := util.GetGOPATH()
		if err != nil {
			return "", fmt.Errorf("get 'GOPATH' failed for find %s : %v", tool, path)
		}
		path = filepath.Join(goPath, "bin", tool)
	}

	isExist, err := util.PathExist(path)
	if err != nil {
		return "", fmt.Errorf("check '%s' path error: %v", path, err)
	}

	if !isExist {
		if tool == meta.TpCompilerThrift {
			// If thriftgo does not exist, the latest version will be installed automatically.
			err := util.InstallAndCheckThriftgo()
			if err != nil {
				return "", fmt.Errorf("can't install '%s' automatically, please install it manually for https://github.com/cloudwego/thriftgo, err : %v", tool, err)
			}
		} else {
			// todo: protoc automatic installation
			return "", fmt.Errorf("%s is not installed, please install it first", tool)
		}
	}

	if tool == meta.TpCompilerThrift {
		// If thriftgo exists, the version is detected; if the version is lower than v0.2.0 then the latest version of thriftgo is automatically installed.
		err := util.CheckAndUpdateThriftgo()
		if err != nil {
			return "", fmt.Errorf("update thriftgo version failed, please install it manually for https://github.com/cloudwego/thriftgo, err: %v", err)
		}
	}

	return path, nil
}

// link removes the previous symbol link and rebuilds a new one.
func link(src, dst string) error {
	err := syscall.Unlink(dst)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unlink %q: %s", dst, err)
	}
	err = os.Symlink(src, dst)
	if err != nil {
		return fmt.Errorf("symlink %q: %s", dst, err)
	}
	return nil
}

func BuildPluginCmd(args *Argument) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to detect current executable, err: %v", err)
	}

	argPacks, err := args.Pack()
	if err != nil {
		return nil, err
	}
	kas := strings.Join(argPacks, ",")

	path, err := lookupTool(args.IdlType)
	if err != nil {
		return nil, err
	}
	cmd := &exec.Cmd{
		Path: path,
	}

	if args.IdlType == meta.IdlThrift {
		// thriftgo
		os.Setenv(meta.EnvPluginMode, meta.ThriftPluginName)
		cmd.Args = append(cmd.Args, meta.TpCompilerThrift)
		for _, inc := range args.Includes {
			cmd.Args = append(cmd.Args, "-i", inc)
		}

		if args.Verbose {
			cmd.Args = append(cmd.Args, "-v")
		}
		thriftOpt, err := args.GetThriftgoOptions()
		if err != nil {
			return nil, err
		}
		cmd.Args = append(cmd.Args,
			"-o", args.ModelOutDir(),
			"-g", thriftOpt,
			"-p", "hertz="+exe+":"+kas,
		)
		for _, p := range args.ThriftPlugins {
			cmd.Args = append(cmd.Args, "-p", p)
		}
		if !args.NoRecurse {
			cmd.Args = append(cmd.Args, "-r")
		}
	} else {
		// protoc
		os.Setenv(meta.EnvPluginMode, meta.ProtocPluginName)
		cmd.Args = append(cmd.Args, meta.TpCompilerProto)
		for _, inc := range args.Includes {
			cmd.Args = append(cmd.Args, "-I", inc)
		}
		for _, inc := range args.IdlPaths {
			cmd.Args = append(cmd.Args, "-I", filepath.Dir(inc))
		}
		cmd.Args = append(cmd.Args,
			"--plugin=protoc-gen-hertz="+exe,
			"--hertz_out="+args.OutDir,
			"--hertz_opt="+kas,
		)
		for _, p := range args.ProtobufPlugins {
			pluginParams := strings.Split(p, ":")
			if len(pluginParams) != 3 {
				logs.Warnf("Failed to get the correct protoc plugin parameters for %. "+
					"Please specify the protoc plugin in the form of \"plugin_name:options:out_dir\"", p)
				os.Exit(1)
			}
			// pluginParams[0] -> plugin name, pluginParams[1] -> plugin options, pluginParams[2] -> out_dir
			cmd.Args = append(cmd.Args,
				fmt.Sprintf("--%s_out=%s", pluginParams[0], pluginParams[2]),
				fmt.Sprintf("--%s_opt=%s", pluginParams[0], pluginParams[1]),
			)
		}
		for _, kv := range args.ProtocOptions {
			cmd.Args = append(cmd.Args, "--"+kv)
		}
	}

	cmd.Args = append(cmd.Args, args.IdlPaths...)
	logs.Infof(strings.Join(cmd.Args, " "))
	logs.Flush()
	return cmd, nil
}

func (arg *Argument) GetThriftgoOptions() (string, error) {
	prefix, err := arg.ModelPackagePrefix()
	if err != nil {
		return "", err
	}
	arg.ThriftOptions = append(arg.ThriftOptions, "package_prefix="+prefix)
	if arg.JSONEnumStr {
		arg.ThriftOptions = append(arg.ThriftOptions, "json_enum_as_text")
	}
	gas := "go:" + strings.Join(arg.ThriftOptions, ",") + ",reserve_comments,gen_json_tag=false"
	return gas, nil
}
