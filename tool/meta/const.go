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

// Version hz version
const Version = "0.0.1"

// Mode hz run modes
type Mode int

// hz Commands
const (
	CmdUpdate = "update"
	CmdNew    = "new"
	// CmdClient = "client"
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
