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
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/internal/meta"
	"github.com/cloudwego/hertz/cmd/hz/internal/util"
	"github.com/urfave/cli/v2"
)

type Argument struct {
	// Mode              meta.Mode // operating mode（0-compiler, 1-plugin)
	CmdType    string // command type
	Verbose    bool   // print verbose log
	Cwd        string // execution path
	OutDir     string // output path
	HandlerDir string // handler path
	ModelDir   string // model path
	RouterDir  string // router path
	ClientDir  string // client path

	IdlType   string   // idl type
	IdlPaths  []string // master idl path
	RawOptPkg []string // user-specified package import path
	OptPkgMap map[string]string
	Includes  []string
	PkgPrefix string

	Gopath      string // $GOPATH
	Gosrc       string // $GOPATH/src
	Gomod       string
	Gopkg       string // $GOPATH/src/{{gopkg}}
	ServiceName string // service name

	JSONEnumStr    bool
	UnsetOmitempty bool
	ProtocOptions  []string // options to pass through to protoc
	ThriftOptions  []string // options to pass through to thriftgo
	SnakeName      bool
	Excludes       []string
	NoRecurse      bool

	// 第三方layout模板配置
	CustomizeLayout  string
	CustomizePackage string
	ModelBackend     string
}

func NewArgument() *Argument {
	return &Argument{
		OptPkgMap:     make(map[string]string),
		Includes:      make([]string, 0, 4),
		Excludes:      make([]string, 0, 4),
		ProtocOptions: make([]string, 0, 4),
		ThriftOptions: make([]string, 0, 4),
	}
}

// Parse initializes a new argument based on its own information
func (arg *Argument) Parse(c *cli.Context, cmd string) (*Argument, error) {
	// v2 cli cannot put the StringSlice flag to struct, so we need to parse it here
	arg.parseStringSlice(c)
	args := arg.Fork()
	args.CmdType = cmd

	err := args.checkPath()
	if err != nil {
		return nil, err
	}

	err = args.checkIDL()
	if err != nil {
		return nil, err
	}

	err = args.checkPackage()
	if err != nil {
		return nil, err
	}

	return args, nil
}

func (arg *Argument) parseStringSlice(c *cli.Context) {
	arg.IdlPaths = c.StringSlice("idl")
	arg.Includes = c.StringSlice("proto_path")
	arg.Excludes = c.StringSlice("exclude_file")
	arg.RawOptPkg = c.StringSlice("option_package")
	arg.ThriftOptions = c.StringSlice("thriftgo")
	arg.ProtocOptions = c.StringSlice("protoc")
}

// checkPath sets the project path and verifies that the model、handler、router and client path is compliant
func (arg *Argument) checkPath() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current path failed: %s", err)
	}
	arg.Cwd = dir
	if arg.OutDir == "" {
		arg.OutDir = dir
	}
	if !filepath.IsAbs(arg.OutDir) {
		ap := filepath.Join(arg.Cwd, arg.OutDir)
		arg.OutDir = ap
	}
	if arg.ModelDir != "" && filepath.IsAbs(arg.ModelDir) {
		return fmt.Errorf("model path %s must be relative to out_dir", arg.ModelDir)
	}
	if arg.HandlerDir != "" && filepath.IsAbs(arg.HandlerDir) {
		return fmt.Errorf("handler path %s must be relative to out_dir", arg.HandlerDir)
	}
	if arg.RouterDir != "" && filepath.IsAbs(arg.RouterDir) {
		return fmt.Errorf("router path %s must be relative to out_dir", arg.RouterDir)
	}
	if arg.ClientDir != "" && filepath.IsAbs(arg.ClientDir) {
		return fmt.Errorf("router path %s must be relative to out_dir", arg.ClientDir)
	}
	return nil
}

// checkIDL check if the idl path exists, set and check the idl type
func (arg *Argument) checkIDL() error {
	for i, path := range arg.IdlPaths {
		abPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("idl path %s is not absolute", path)
		}
		ext := filepath.Ext(abPath)
		if ext == "" || ext[0] != '.' {
			return fmt.Errorf("idl path %s is not a valid file", path)
		}
		ext = ext[1:]
		switch ext {
		case meta.IdlThrift:
			arg.IdlType = meta.IdlThrift
		case meta.IdlProto:
			arg.IdlType = meta.IdlProto
		default:
			return fmt.Errorf("IDL type %s is not supported", ext)
		}
		arg.IdlPaths[i] = abPath
	}
	return nil
}

func (arg Argument) IsUpdate() bool {
	return arg.CmdType == meta.CmdUpdate
}

// checkPackage check and set the gopath、 module and package name
func (arg *Argument) checkPackage() error {
	gopath, err := util.GetGOPATH()
	if err != nil {
		return fmt.Errorf("get gopath failed: %s", err)
	}
	if gopath == "" {
		return fmt.Errorf("GOPATH is not set")
	}

	arg.Gopath = gopath
	arg.Gosrc = filepath.Join(gopath, "src")

	// Generate the project under gopath, use the relative path as the package name
	if strings.HasPrefix(arg.Cwd, arg.Gosrc) {
		if gopkg, err := filepath.Rel(arg.Gosrc, arg.Cwd); err != nil {
			return fmt.Errorf("get relative path to GOPATH/src failed: %s", err)
		} else {
			arg.Gopkg = gopkg
		}
		if arg.Gomod == "" {
			arg.Gomod = arg.Gopkg
		}
	}
	if !arg.IsUpdate() && arg.Gomod == "" {
		return fmt.Errorf("output directory %s is not under GOPATH/src. Please specify a module name with the '-module' flag", arg.Cwd)
	}

	module, path, ok := util.SearchGoMod(".", false)
	if ok {
		// go.mod exists
		if module != arg.Gomod && !arg.IsUpdate() {
			return fmt.Errorf("module name given by the '-module' option ('%s') is not consist with the name defined in go.mod ('%s' from %s)\n", arg.Gomod, module, path)
		}
		arg.Gomod = module
	}

	if len(arg.RawOptPkg) > 0 {
		arg.OptPkgMap = make(map[string]string, len(arg.RawOptPkg))
		for _, op := range arg.RawOptPkg {
			ps := strings.SplitN(op, "=", 2)
			if len(ps) != 2 {
				return fmt.Errorf("invalid option package: %s", op)
			}
			arg.OptPkgMap[ps[0]] = ps[1]
		}
		arg.RawOptPkg = nil
	}
	return nil
}

func (arg *Argument) Pack() ([]string, error) {
	data, err := util.PackArgs(arg)
	if err != nil {
		return nil, fmt.Errorf("pack argument failed: %s", err)
	}
	return data, nil
}

func (arg *Argument) Unpack(data []string) error {
	err := util.UnpackArgs(data, arg)
	if err != nil {
		return fmt.Errorf("unpack argument failed: %s", err)
	}
	return nil
}

// Fork can copy its own parameters to a new argument
func (arg *Argument) Fork() *Argument {
	args := NewArgument()
	*args = *arg
	util.CopyString2StringMap(arg.OptPkgMap, args.OptPkgMap)
	util.CopyStringSlice(&arg.Includes, &args.Includes)
	util.CopyStringSlice(&arg.Excludes, &args.Excludes)
	util.CopyStringSlice(&arg.ProtocOptions, &args.ProtocOptions)
	util.CopyStringSlice(&arg.ThriftOptions, &args.ThriftOptions)
	return args
}

func (arg *Argument) GetGoPackage() (string, error) {
	if arg.Gomod != "" {
		return arg.Gomod, nil
	} else if arg.Gopkg != "" {
		return arg.Gopkg, nil
	}
	return "", fmt.Errorf("project package name is not set")
}

func IdlTypeToCompiler(idlType string) (string, error) {
	switch idlType {
	case meta.IdlProto:
		return meta.TpCompilerProto, nil
	case meta.IdlThrift:
		return meta.TpCompilerThrift, nil
	default:
		return "", fmt.Errorf("IDL type %s is not supported", idlType)
	}
}

func (arg *Argument) ModelPackagePrefix() (string, error) {
	ret := arg.Gomod
	if arg.ModelDir == "" {
		path, err := util.RelativePath(meta.ModelDir)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if err != nil {
			return "", err
		}
		ret += path
	} else {
		path, err := util.RelativePath(arg.ModelDir)
		if err != nil {
			return "", err
		}
		ret += "/" + path
	}
	return strings.ReplaceAll(ret, string(filepath.Separator), "/"), nil
}

func (arg *Argument) ModelOutDir() string {
	ret := arg.OutDir
	if arg.ModelDir == "" {
		ret = filepath.Join(ret, meta.ModelDir)
	} else {
		ret = filepath.Join(ret, arg.ModelDir)
	}
	return ret
}

func (arg *Argument) GetHandlerDir() (string, error) {
	if arg.HandlerDir == "" {
		return util.RelativePath(meta.HandlerDir)
	}
	return util.RelativePath(arg.HandlerDir)
}

func (arg *Argument) GetModelDir() (string, error) {
	if arg.ModelDir == "" {
		return util.RelativePath(meta.ModelDir)
	}
	return util.RelativePath(arg.ModelDir)
}

func (arg *Argument) GetRouterDir() (string, error) {
	if arg.RouterDir == "" {
		return util.RelativePath(meta.RouterDir)
	}
	return util.RelativePath(arg.RouterDir)
}

func (arg *Argument) GetClientDir() (string, error) {
	if arg.ClientDir == "" {
		return "", nil
	}
	return util.RelativePath(arg.ClientDir)
}
