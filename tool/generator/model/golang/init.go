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

package golang

import (
	"fmt"
	"strings"
	"text/template"
)

var tpls *template.Template

var list = map[string]string{
	"file":      file,
	"typedef":   typedef,
	"constants": constants,
	"variables": variables,
	"funtion":   function,
	"enum":      enum,
	"struct":    structLike,
	"method":    method,
	"oneof":     oneof,
}

/***********************Export API*******************************/

func Template() (*template.Template, error) {
	if tpls != nil {
		return tpls, nil
	}
	tpls = new(template.Template)

	tpls = tpls.Funcs(funcMap)

	var err error
	for k, li := range list {
		tpls, err = tpls.Parse(li)
		if err != nil {
			return nil, fmt.Errorf("parse template '%s' failed, err: %v", k, err.Error())
		}
	}
	return tpls, nil
}

func List() map[string]string {
	return list
}

/***********************Template Funcs**************************/

var funcMap = template.FuncMap{
	"Features":            getFeatures,
	"Identify":            identify,
	"CamelCase":           camelCase,
	"SnakeCase":           snakeCase,
	"GetTypedefReturnStr": getTypedefReturnStr,
}

func Funcs(name string, fn interface{}) error {
	if _, ok := funcMap[name]; ok {
		return fmt.Errorf("duplicate function: %s has been registered", name)
	}
	funcMap[name] = fn
	return nil
}

func identify(name string) string {
	return name
}

func camelCase(name string) string {
	return name
}

func snakeCase(name string) string {
	return name
}

func getTypedefReturnStr(name string) string {
	if strings.Contains(name, ".") {
		idx := strings.LastIndex(name, ".")
		return name[:idx] + "." + "New" + name[idx+1:] + "()"

	}
	return "New" + name + "()"
}

/***********************Template Options**************************/

type feature struct {
	MarshalEnumToText  bool
	TypedefAsTypeAlias bool
}

var features = feature{}

func getFeatures() feature {
	return features
}

func SetOption(opt string) error {
	switch opt {
	case "MarshalEnumToText":
		features.MarshalEnumToText = true
	case "TypedefAsTypeAlias":
		features.TypedefAsTypeAlias = true
	}
	return nil
}

var Options = []string{
	"MarshalEnumToText",
	"TypedefAsTypeAlias",
}

func GetOptions() []string {
	return Options
}
