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
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/hertz/cmd/hz/util/logs"
)

type HttpMethod struct {
	Name               string
	HTTPMethod         string
	Comment            string
	RequestTypeName    string
	RequestTypePackage string
	RequestTypeRawName string
	ReturnTypeName     string
	ReturnTypePackage  string
	ReturnTypeRawName  string
	Path               string
	Serializer         string
	OutputDir          string
	RefPackage         string // handler import dir
	RefPackageAlias    string // handler import alias
	ModelPackage       map[string]string
	GenHandler         bool // Whether to generate one handler, when an idl interface corresponds to multiple http method
	// Annotations     map[string]string
	Models map[string]*model.Model
}

type Handler struct {
	FilePath    string
	PackageName string
	ProjPackage string
	Imports     map[string]*model.Model
	Methods     []*HttpMethod
}

type Client struct {
	Handler
	ServiceName string
}

func (pkgGen *HttpPackageGenerator) genHandler(pkg *HttpPackage, handlerDir, handlerPackage string, root *RouterNode) error {
	for _, s := range pkg.Services {
		var handler Handler
		if pkgGen.HandlerByMethod { // generate handler by method
			for _, m := range s.Methods {
				filePath := filepath.Join(handlerDir, m.OutputDir, util.ToSnakeCase(m.Name)+".go")
				handler = Handler{
					FilePath:    filePath,
					PackageName: util.SplitPackage(filepath.Dir(filePath), ""),
					Methods:     []*HttpMethod{m},
					ProjPackage: pkgGen.ProjPackage,
				}

				if err := pkgGen.processHandler(&handler, root, handlerDir, m.OutputDir, true); err != nil {
					return fmt.Errorf("generate handler %s failed, err: %v", handler.FilePath, err.Error())
				}

				if m.GenHandler {
					if err := pkgGen.updateHandler(handler, handlerTplName, handler.FilePath, false); err != nil {
						return fmt.Errorf("generate handler %s failed, err: %v", handler.FilePath, err.Error())
					}
				}
			}
		} else { // generate handler service
			tmpHandlerDir := handlerDir
			tmpHandlerPackage := handlerPackage
			if len(s.ServiceGenDir) != 0 {
				tmpHandlerDir = s.ServiceGenDir
				tmpHandlerPackage = util.SubPackage(pkgGen.ProjPackage, tmpHandlerDir)
			}
			handler = Handler{
				FilePath:    filepath.Join(tmpHandlerDir, util.ToSnakeCase(s.Name)+".go"),
				PackageName: util.SplitPackage(tmpHandlerPackage, ""),
				Methods:     s.Methods,
				ProjPackage: pkgGen.ProjPackage,
			}

			for _, m := range s.Methods {
				m.RefPackage = tmpHandlerPackage
				m.RefPackageAlias = util.BaseName(tmpHandlerPackage, "")
			}

			if err := pkgGen.processHandler(&handler, root, "", "", false); err != nil {
				return fmt.Errorf("generate handler %s failed, err: %v", handler.FilePath, err.Error())
			}

			// Avoid generating duplicate handlers when IDL interface corresponds to multiple http methods
			methods := handler.Methods
			handler.Methods = []*HttpMethod{}
			for _, m := range methods {
				if m.GenHandler {
					handler.Methods = append(handler.Methods, m)
				}
			}

			if err := pkgGen.updateHandler(handler, handlerTplName, handler.FilePath, false); err != nil {
				return fmt.Errorf("generate handler %s failed, err: %v", handler.FilePath, err.Error())
			}
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

func (pkgGen *HttpPackageGenerator) processHandler(handler *Handler, root *RouterNode, handlerDir, projectOutDir string, handlerByMethod bool) error {
	singleHandlerPackage := ""
	if handlerByMethod {
		singleHandlerPackage = util.SubPackage(pkgGen.ProjPackage, filepath.Join(handlerDir, projectOutDir))
	}
	handler.Imports = make(map[string]*model.Model, len(handler.Methods))
	for _, m := range handler.Methods {
		// Iterate over the request and return parameters of the method to get import path.
		for key, mm := range m.Models {
			if v, ok := handler.Imports[mm.PackageName]; ok && v.Package != mm.Package {
				handler.Imports[key] = mm
				continue
			}
			handler.Imports[mm.PackageName] = mm
		}
		err := root.Update(m, handler.PackageName, singleHandlerPackage)
		if err != nil {
			return err
		}
	}

	if len(pkgGen.UseDir) != 0 {
		oldModelDir := filepath.Clean(filepath.Join(pkgGen.ProjPackage, pkgGen.ModelDir))
		newModelDir := filepath.Clean(pkgGen.UseDir)
		for _, m := range handler.Methods {
			for _, mm := range m.Models {
				mm.Package = strings.Replace(mm.Package, oldModelDir, newModelDir, 1)
			}
		}
	}

	handler.Format()
	return nil
}

func (pkgGen *HttpPackageGenerator) updateHandler(handler interface{}, handlerTpl, filePath string, noRepeat bool) error {
	if pkgGen.tplsInfo[handlerTpl].Disable {
		return nil
	}
	isExist, err := util.PathExist(filePath)
	if err != nil {
		return err
	}
	if !isExist {
		return pkgGen.TemplateGenerator.Generate(handler, handlerTpl, filePath, noRepeat)
	}
	if pkgGen.HandlerByMethod { // method by handler, do not need to insert new content
		return nil
	}

	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	// insert new model imports
	for alias, model := range handler.(Handler).Imports {
		if bytes.Contains(file, []byte(model.Package)) {
			continue
		}
		file, err = util.AddImportForContent(file, alias, model.Package)
		if err != nil {
			return err
		}
	}
	// insert customized imports
	if tplInfo, exist := pkgGen.TemplateGenerator.tplsInfo[handlerTpl]; exist {
		if len(tplInfo.UpdateBehavior.ImportTpl) != 0 {
			imptSlice, err := getInsertImportContent(tplInfo, handler, file)
			if err != nil {
				return err
			}
			for _, impt := range imptSlice {
				if bytes.Contains(file, []byte(impt[1])) {
					continue
				}
				file, err = util.AddImportForContent(file, impt[0], impt[1])
				if err != nil {
					logs.Warnf("can not add import(%s) for file(%s), err: %v\n", impt[1], filePath, err)
				}
			}
		}
	}

	// insert new handler
	for _, method := range handler.(Handler).Methods {
		if bytes.Contains(file, []byte(fmt.Sprintf("func %s(", method.Name))) {
			continue
		}

		// Generate additional handlers using templates
		handlerSingleTpl := pkgGen.tpls[handlerSingleTplName]
		if handlerSingleTpl == nil {
			return fmt.Errorf("tpl %s not found", handlerSingleTplName)
		}
		data := make(map[string]string, 5)
		data["Comment"] = method.Comment
		data["Name"] = method.Name
		data["RequestTypeName"] = method.RequestTypeName
		data["ReturnTypeName"] = method.ReturnTypeName
		data["Serializer"] = method.Serializer
		data["OutputDir"] = method.OutputDir
		handlerFunc := bytes.NewBuffer(nil)
		err = handlerSingleTpl.Execute(handlerFunc, data)
		if err != nil {
			return fmt.Errorf("execute template \"%s\" failed, %v", handlerSingleTplName, err)
		}

		buf := bytes.NewBuffer(nil)
		_, err = buf.Write(file)
		if err != nil {
			return fmt.Errorf("write handler \"%s\" failed, %v", method.Name, err)
		}
		_, err = buf.Write(handlerFunc.Bytes())
		if err != nil {
			return fmt.Errorf("write handler \"%s\" failed, %v", method.Name, err)
		}
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
