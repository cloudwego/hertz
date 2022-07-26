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

package plugin

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/internal/meta"
	"github.com/cloudwego/hertz/cmd/hz/internal/protobuf"
	"github.com/cloudwego/hertz/cmd/hz/internal/thrift"
	"github.com/cloudwego/hertz/cmd/hz/internal/util"
)

// PluginMode makes the tool run in plugin mode.
// If you use thriftgo, it will run as "thrift-gen-hertz"
// If you use protoc, it will run as "protoc-gen-hertz"
func PluginMode() {
	pluginName := filepath.Base(os.Args[0])
	if util.IsWindows() {
		pluginName = strings.TrimSuffix(pluginName, ".exe")
	}
	switch pluginName {
	case meta.ThriftPluginName:
		plugin := new(thrift.Plugin)
		os.Exit(plugin.Run())
	case meta.ProtocPluginName:
		plugin := new(protobuf.Plugin)
		os.Exit(plugin.Run())
	}
}

// NewThriftgoPlugin creates a plugin for 'thriftgo' that can be called separately as a 'thriftgo plugin'.
func NewThriftgoPlugin() *thrift.Plugin {
	return new(thrift.Plugin)
}

// NewProtocPlugin creates a plugin for 'protoc' that can be called separately as a 'protoc plugin'.
func NewProtocPlugin() *protobuf.Plugin {
	return new(protobuf.Plugin)
}
