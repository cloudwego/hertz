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

package main

import (
	"os"
	"path/filepath"

	"github.com/cloudwego/hertz/cmd/hz/internal/app"
	"github.com/cloudwego/hertz/cmd/hz/internal/meta"
	"github.com/cloudwego/hertz/cmd/hz/internal/protobuf"
	"github.com/cloudwego/hertz/cmd/hz/internal/thrift"
	"github.com/cloudwego/hertz/cmd/hz/internal/util/logs"
)

func main() {
	// run in plugin mode
	pluginMode()

	// run in normal mode
	Run()
}

func Run() {
	defer func() {
		logs.Flush()
	}()

	cli := app.Init()
	err := cli.Run(os.Args)
	if err != nil {
		logs.Errorf("%v\n", err)
	}
}

func pluginMode() {
	switch filepath.Base(os.Args[0]) {
	case meta.ThriftPluginName:
		plugin := new(thrift.Plugin)
		os.Exit(plugin.Run())
	case meta.ProtocPluginName:
		plugin := new(protobuf.Plugin)
		os.Exit(plugin.Run())
	}
}
