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

package meta

import "runtime"

// Version hz version
const Version = "v0.6.7"

const DefaultServiceName = "hertz_service"

// Mode hz run modes
type Mode int

// SysType is the running program's operating system type
const SysType = runtime.GOOS

const WindowsOS = "windows"

const EnvPluginMode = "HERTZ_PLUGIN_MODE"

// hz Commands
const (
	CmdUpdate = "update"
	CmdNew    = "new"
	CmdModel  = "model"
	CmdClient = "client"
)

// hz IDLs
const (
	IdlThrift = "thrift"
	IdlProto  = "proto"
)

// Third-party Compilers
const (
	TpCompilerThrift = "thriftgo"
	TpCompilerProto  = "protoc"
)

// hz Plugins
const (
	ProtocPluginName = "protoc-gen-hertz"
	ThriftPluginName = "thrift-gen-hertz"
)

// hz Errors
const (
	LoadError           = 1
	GenerateLayoutError = 2
	PersistError        = 3
	PluginError         = 4
)

// Package Dir
const (
	ModelDir   = "biz/model"
	RouterDir  = "biz/router"
	HandlerDir = "biz/handler"
)

// Backend Model Backends
type Backend string

const (
	BackendGolang Backend = "golang"
)

// template const value
const (
	SetBodyParam = "setBodyParam(req).\n"
)

// TheUseOptionMessage indicates that the generating of 'model code' is aborted due to the -use option for thrift IDL.
const TheUseOptionMessage = "'model code' is not generated due to the '-use' option"

const AddThriftReplace = "do not generate 'go.mod', please add 'replace github.com/apache/thrift => github.com/apache/thrift v0.13.0' to your 'go.mod'"
