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

package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/cloudwego/hertz/cmd/hz/internal/util"
	"gopkg.in/yaml.v2"
)

// Layout contains the basic information of idl
type Layout struct {
	GoModule        string
	ServiceName     string
	UseApacheThrift bool
	HasIdl          bool
}

// LayoutGenerator contains the information generated by generating the layout template
type LayoutGenerator struct {
	ConfigPath string
	TemplateGenerator
}

var (
	layoutConfig  = defaultLayoutConfig
	packageConfig = defaultPkgConfig
)

func SetDefaultTemplateConfig() {
	layoutConfig = defaultLayoutConfig
	packageConfig = defaultPkgConfig
}

func (lg *LayoutGenerator) Init() error {
	config := layoutConfig
	// unmarshal from user-defined config file if it exists
	if lg.ConfigPath != "" {
		cdata, err := ioutil.ReadFile(lg.ConfigPath)
		if err != nil {
			return fmt.Errorf("read layout config from  %s failed, err: %v", lg.ConfigPath, err.Error())
		}
		config = TemplateConfig{}
		if err = yaml.Unmarshal(cdata, &config); err != nil {
			return fmt.Errorf("unmarshal layout config failed, err: %v", err.Error())
		}
	}

	if reflect.DeepEqual(config, TemplateConfig{}) {
		return errors.New("empty config")
	}
	lg.Config = &config

	return lg.TemplateGenerator.Init()
}

// checkInited initialize template definition
func (lg *LayoutGenerator) checkInited() error {
	if lg.tpls == nil || lg.dirs == nil {
		if err := lg.Init(); err != nil {
			return fmt.Errorf("init layout config failed, err: %v", err.Error())
		}
	}
	return nil
}

func (lg *LayoutGenerator) Generate(data map[string]interface{}) error {
	if err := lg.checkInited(); err != nil {
		return err
	}
	return lg.TemplateGenerator.Generate(data, "", "", false)
}

func (lg *LayoutGenerator) GenerateByService(service Layout) error {
	if err := lg.checkInited(); err != nil {
		return err
	}

	sd, err := serviceToLayoutData(service)
	if err != nil {
		return err
	}

	rd, err := serviceToRouterData(service)
	if err != nil {
		return err
	}
	if service.HasIdl {
		delete(lg.tpls, defaultRouterDir+sp+registerTplName)
	}

	data := map[string]interface{}{
		"*":                                    sd,
		layoutConfig.Layouts[routerIndex].Path: rd, // router.go
		layoutConfig.Layouts[routerGenIndex].Path: rd, // router_gen.go
	}

	return lg.Generate(data)
}

// serviceToLayoutData stores go mod, serviceName, UseApacheThrift mapping
func serviceToLayoutData(service Layout) (map[string]interface{}, error) {
	goMod := service.GoModule
	if goMod == "" {
		return nil, errors.New("please specify go-module")
	}

	return map[string]interface{}{
		"GoModule":        goMod,
		"ServiceName":     service.ServiceName,
		"UseApacheThrift": service.UseApacheThrift,
	}, nil
}

// serviceToRouterData stores the registers function, router import path, handler import path
func serviceToRouterData(service Layout) (map[string]interface{}, error) {
	return map[string]interface{}{
		"Registers":      []string{},
		"RouterPkgPath":  service.GoModule + util.PathToImport(sp+defaultRouterDir, ""),
		"HandlerPkgPath": service.GoModule + util.PathToImport(sp+defaultHandlerDir, ""),
	}, nil
}

func (lg *LayoutGenerator) GenerateByConfig(configPath string) error {
	if err := lg.checkInited(); err != nil {
		return err
	}
	buf, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read data file '%s' failed, err: %v", configPath, err.Error())
	}
	var data map[string]interface{}
	if err := json.Unmarshal(buf, &data); err != nil {
		return fmt.Errorf("unmarshal json data failed, err: %v", err.Error())
	}
	return lg.Generate(data)
}

func (lg *LayoutGenerator) Degenerate() error {
	return lg.TemplateGenerator.Degenerate()
}
