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
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/hertz/tool/generator/model"
	"github.com/cloudwego/hertz/tool/util"
	"github.com/cloudwego/hertz/tool/util/logs"
)

type HttpMethod struct {
	Name            string
	HTTPMethod      string
	Comment         string
	RequestTypeName string
	ReturnTypeName  string
	Path            string
	Serializer      string
	// Annotations     map[string]string
	Models map[string]*model.Model
}

type Handler struct {
	FilePath    string
	PackageName string
	Imports     map[string]*model.Model
	Methods     []*HttpMethod
}

type Client struct {
	Handler
	ServiceName string
}

func (pkgGen *HttpPackageGenerator) genHandler(pkg *HttpPackage, handlerDir, handlerPackage string, root *RouterNode) error {
	for _, s := range pkg.Services {
		handler := Handler{
			FilePath:    filepath.Join(handlerDir, util.ToSnakeCase(s.Name)+".go"),
			PackageName: util.SplitPackage(handlerPackage, ""),
			Methods:     s.Methods,
		}

		handler.Imports = make(map[string]*model.Model, len(s.Methods))
		for _, m := range s.Methods {
			for key, mm := range m.Models {
				if v, ok := handler.Imports[mm.PackageName]; ok && v.Package != mm.Package {
					handler.Imports[key] = mm
					continue
				}
				handler.Imports[mm.PackageName] = mm
			}
			err := root.Update(m, handler.PackageName)
			if err != nil {
				return err
			}
		}
		handler.Format()
		if err := pkgGen.updateHandler(handler, handlerTplName, handler.FilePath, false); err != nil {
			return fmt.Errorf("generate handler %s failed, err: %v", handler.FilePath, err.Error())
		}

		if len(pkgGen.ClientDir) != 0 {
			clientDir := util.SubDir(pkgGen.ClientDir, pkg.Package)
			clientPackage := util.SubPackage(pkgGen.ProjPackage, clientDir)
			client := Client{}
			client.Handler = handler
			client.ServiceName = s.Name
			client.PackageName = util.SplitPackage(clientPackage, "")
			client.FilePath = filepath.Join(clientDir, util.ToSnakeCase(s.Name)+".go")
			if err := pkgGen.updateClient(client, clientTplName, client.FilePath, false); err != nil {
				return fmt.Errorf("generate client %s failed, err: %v", client.FilePath, err.Error())
			}
		}

	}
	return nil
}

func (pkgGen *HttpPackageGenerator) updateHandler(handler interface{}, handlerTpl, filePath string, noRepeat bool) error {
	isExist, err := util.PathExist(filePath)
	if err != nil {
		return err
	}
	if !isExist {
		return pkgGen.TemplateGenerator.Generate(handler, handlerTpl, filePath, noRepeat)
	}

	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	hertzImport := regexp.MustCompile(`"github.com/cloudwego/hertz/pkg/app"\n`)
	// insert new imports
	for alias, model := range handler.(Handler).Imports {
		if bytes.Contains(file, []byte(model.Package)) {
			continue
		}

		subIndexImport := hertzImport.FindSubmatchIndex(file)
		if len(subIndexImport) != 2 || subIndexImport[0] < 1 {
			return fmt.Errorf("\"github.com/cloudwego/hertz/pkg/app\" not found in %s", string(file))
		}

		buf := bytes.NewBuffer(nil)
		buf.Write(file[:subIndexImport[1]])
		buf.WriteString("\n\t" + fmt.Sprintf("%s \"%s\"\n", alias, model.Package))
		buf.Write(file[subIndexImport[1]:])
		file = buf.Bytes()
	}

	// insert new handler
	for _, method := range handler.(Handler).Methods {
		if bytes.Contains(file, []byte(fmt.Sprintf("func %s(ctx context.Context, c *app.RequestContext)", method.Name))) {
			continue
		}

		handlerFunc := fmt.Sprintf("%s\n"+
			"func %s(ctx context.Context, c *app.RequestContext) { \n"+
			"\tvar err error\n"+
			"\tvar req %s\n"+
			"\terr = c.BindAndValidate(&req)\n"+
			"\tif err != nil {\n"+
			"\t\tc.String(400, err.Error())\n"+
			"\t\treturn\n"+
			"\t}\n\n"+
			"\tresp := new(%s)\n\n"+
			"\tc.%s(200, resp)\n"+
			"}\n", method.Comment, method.Name, method.RequestTypeName, method.ReturnTypeName, method.Serializer)

		if len(method.Name) == 0 {
			handlerFunc = fmt.Sprintf("%s\n"+
				"func %s(ctx context.Context, c *app.RequestContext) { \n"+
				"\tvar err error\n"+
				"\tresp := new(%s)\n\n"+
				"\tc.%s(200, resp)\n"+
				"}\n", method.Comment, method.Name, method.ReturnTypeName, method.Serializer)
		}

		buf := bytes.NewBuffer(nil)
		buf.Write(file)
		buf.Write([]byte(handlerFunc))
		file = buf.Bytes()
	}

	pkgGen.files = append(pkgGen.files, File{filePath, string(file), false, ""})

	return nil
}

func (pkgGen *HttpPackageGenerator) updateClient(client interface{}, clientTpl, filePath string, noRepeat bool) error {
	isExist, err := util.PathExist(filePath)
	if err != nil {
		return err
	}
	if !isExist {
		return pkgGen.TemplateGenerator.Generate(client, clientTpl, filePath, noRepeat)
	}
	logs.Infof("Client file:%s has been generated, so don't update it", filePath)

	return nil
}

func (m *HttpMethod) InitComment() {
	text := strings.TrimLeft(strings.TrimSpace(m.Comment), "/")
	if text == "" {
		text = "// " + m.Name + " ."
	} else if strings.HasPrefix(text, m.Name) {
		text = "// " + text
	} else {
		text = "// " + m.Name + " " + text
	}
	text = strings.Replace(text, "\n", "\n// ", -1)
	if !strings.Contains(text, "@router ") {
		text += "\n// @router " + m.Path
	}
	m.Comment = text + " [" + m.HTTPMethod + "]"
}

func MapSerializer(serializer string) string {
	switch serializer {
	case "json":
		return "JSON"
	case "thrift":
		return "Thrift"
	case "pb":
		return "ProtoBuf"
	default:
		return "JSON"
	}
}

func (h *Handler) Format() {
	for _, m := range h.Methods {
		m.Serializer = MapSerializer(m.Serializer)
		m.InitComment()
	}
}
