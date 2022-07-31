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
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/internal/util"
)

type Router struct {
	FilePath        string
	PackageName     string
	HandlerPackages map[string]string // {{basename}}:{{import_path}}
	Router          *RouterNode
}

type RouterNode struct {
	GroupName  string
	MiddleWare string

	Path     string
	Children childrenRouterInfo

	Handler    string // {{HandlerPackage}}.{{HandlerName}}
	HttpMethod string
}

type RegisterDependency struct {
	PkgAlias string
	Pkg      string
}

// NewRouterTree contains "/" as root node
func NewRouterTree() *RouterNode {
	return &RouterNode{
		GroupName:  "root",
		MiddleWare: "root",
		Path:       "/",
	}
}

func (routerNode *RouterNode) Sort() {
	sort.Sort(routerNode.Children)
}

func (routerNode *RouterNode) Update(method *HttpMethod, handlerType string) error {
	if method.Path == "" {
		return fmt.Errorf("empty path for method '%s'", method.Name)
	}
	paths := strings.Split(method.Path, "/")
	if paths[0] == "" {
		paths = paths[1:]
	}
	parent, last := routerNode.FindNearest(paths)
	if last == len(paths) {
		return fmt.Errorf("path '%s' has been registered", method.Path)
	}
	name := util.ToVarName(paths[:last])
	parent.Insert(name, method, handlerType, paths[last:])
	parent.Sort()
	return nil
}

// DyeGroupName traverses the routing tree in depth and names the middleware for each node.
func (routerNode *RouterNode) DyeGroupName() error {
	groups := []string{"root"}
	unique := map[string]bool{}

	hook := func(layer int, node *RouterNode) error {
		node.GroupName = groups[layer]
		if node.MiddleWare == "" {
			pname := node.Path
			if len(pname) > 1 && pname[0] == '/' {
				pname = pname[1:]
			}
			if len(node.Children) == 0 && node.Handler != "" {
				handleName := strings.Split(node.Handler, ".")
				pname = handleName[len(handleName)-1]
			}
			origin := util.ToVarName([]string{pname})
			pname = origin
			// There may be a node with the same name on the routing tree.
			// In order to prevent the generated routing groups from having the same name,
			// the names of these routing groups are renamed.
			// 10000 is a large number that cannot be repeated.
			for i := 0; i < 10000; i++ {
				if _, ok := unique[pname]; ok {
					pname = origin + strconv.Itoa(i)
					if i == 9999 {
						return fmt.Errorf("cannot generate a unique name for the routing group. "+
							"The sequence number of the routing group for %s with the same name cannot exceed 9999. "+
							"\"+\n\t\t\t\t\t\t\t\"Please reduce the use of routing groups with the same name", origin)
					}
				} else {
					unique[pname] = true
					break
				}
			}
			node.MiddleWare = "_" + strings.ToLower(pname)
		}
		if layer >= len(groups)-1 {
			groups = append(groups, node.MiddleWare)
		} else {
			groups[layer+1] = node.MiddleWare
		}
		return nil
	}

	// Deep traversal from the 0th level of the routing tree.
	err := routerNode.DFS(0, hook)
	return err
}

func (routerNode *RouterNode) DFS(i int, hook func(layer int, node *RouterNode) error) error {
	if routerNode == nil {
		return nil
	}
	err := hook(i, routerNode)
	if err != nil {
		return err
	}
	for _, n := range routerNode.Children {
		err = n.DFS(i+1, hook)
		if err != nil {
			return err
		}
	}
	return nil
}

func (routerNode *RouterNode) Insert(name string, method *HttpMethod, handlerType string, paths []string) {
	cur := routerNode
	for i, p := range paths {
		c := &RouterNode{
			Path: "/" + p,
		}
		if i == len(paths)-1 {
			c.Handler = handlerType + "." + method.Name
			c.HttpMethod = getHttpMethod(method.HTTPMethod)
		}
		if cur.Children == nil {
			cur.Children = make([]*RouterNode, 0, 1)
		}
		cur.Children = append(cur.Children, c)
		cur = c
	}
}

func getHttpMethod(method string) string {
	if strings.EqualFold(method, "Any") {
		return "Any"
	}
	return strings.ToUpper(method)
}

func (routerNode *RouterNode) FindNearest(paths []string) (*RouterNode, int) {
	ns := len(paths)
	cur := routerNode
	i := 0
	path := paths[i]
	for j := 0; j < len(cur.Children); j++ {
		c := cur.Children[j]
		if ("/" + path) == c.Path {
			i++
			if i == ns {
				return cur, i - 1
			}
			path = paths[i]
			cur = c
			j = -1
		}
	}
	return cur, i
}

type childrenRouterInfo []*RouterNode

// Len is the number of elements in the collection.
func (c childrenRouterInfo) Len() int {
	return len(c)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (c childrenRouterInfo) Less(i, j int) bool {
	ci := c[i].Path
	if len(c[i].Children) != 0 {
		ci = ci[1:]
	}
	cj := c[j].Path
	if len(c[j].Children) != 0 {
		cj = cj[1:]
	}
	return ci < cj
}

// Swap swaps the elements with indexes i and j.
func (c childrenRouterInfo) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

var (
	regRegisterV3 = regexp.MustCompile(insertPointPatternNew)
	regImport     = regexp.MustCompile(`import \(\n`)
)

func (pkgGen *HttpPackageGenerator) updateRegister(pkg, rDir, pkgName string) error {
	register := RegisterDependency{
		PkgAlias: strings.ReplaceAll(pkgName, "/", "_"),
		Pkg:      pkg,
	}
	registerPath := filepath.Join(rDir, registerTplName)
	isExist, err := util.PathExist(registerPath)
	if err != nil {
		return err
	}
	if !isExist {
		return pkgGen.TemplateGenerator.Generate(register, registerTplName, registerPath, false)
	}

	file, err := ioutil.ReadFile(registerPath)
	if err != nil {
		return fmt.Errorf("read register '%s' failed, err: %v", registerPath, err.Error())
	}

	insertImport := register.PkgAlias + " " + "\"" + register.Pkg + "\"\n"
	if !bytes.Contains(file, []byte(register.Pkg)) {

		subIndexImport := regImport.FindSubmatchIndex(file)
		if len(subIndexImport) != 2 || subIndexImport[0] < 1 {
			return fmt.Errorf("wrong format %s: 'import (' not found", string(file))
		}

		bufImport := bytes.NewBuffer(nil)
		bufImport.Write(file[:subIndexImport[1]])
		bufImport.WriteString("\n\t" + insertImport)
		bufImport.Write(file[subIndexImport[1]:])

		file = bufImport.Bytes()

		insertReg := register.PkgAlias + ".Register(r)\n"
		if bytes.Contains(file, []byte(insertReg)) {
			return fmt.Errorf("the router(%s) has been registered", insertReg)
		}

		subIndexReg := regRegisterV3.FindSubmatchIndex(file)
		if len(subIndexReg) != 2 || subIndexReg[0] < 1 {
			return fmt.Errorf("wrong format %s: insert-point '%s' not found", string(file), insertPointPatternNew)
		}

		bufReg := bytes.NewBuffer(nil)
		bufReg.Write(file[:subIndexReg[1]])
		bufReg.WriteString("\n\t" + insertReg)
		bufReg.Write(file[subIndexReg[1]:])

		pkgGen.files = append(pkgGen.files, File{registerPath, string(bufReg.Bytes()), false, registerTplName})
	}

	return nil
}

func (pkgGen *HttpPackageGenerator) genRouter(pkg *HttpPackage, root *RouterNode, handlerPackage, routerDir, routerPackage string) error {
	err := root.DyeGroupName()
	if err != nil {
		return err
	}
	router := Router{
		FilePath:    filepath.Join(routerDir, util.BaseNameAndTrim(pkg.IdlName)+".go"),
		PackageName: util.ToCamelCase(util.BaseName(routerPackage, "")),
		HandlerPackages: map[string]string{
			util.BaseName(handlerPackage, ""): handlerPackage,
		},
		Router: root,
	}
	if err := pkgGen.TemplateGenerator.Generate(router, routerTplName, router.FilePath, false); err != nil {
		return fmt.Errorf("generate router %s failed, err: %v", router.FilePath, err.Error())
	}
	if err := pkgGen.updateMiddlewareReg(router, middlewareTplName, filepath.Join(routerDir, "middleware.go")); err != nil {
		return fmt.Errorf("generate middleware %s failed, err: %v", filepath.Join(routerDir, "middleware.go"), err.Error())
	}

	if err := pkgGen.updateRegister(routerPackage, pkgGen.RouterDir, pkg.Package); err != nil {
		return fmt.Errorf("update register for %s failed, err: %v", filepath.Join(routerDir, registerTplName), err.Error())
	}
	return nil
}

func (pkgGen *HttpPackageGenerator) updateMiddlewareReg(router interface{}, middlewareTpl, filePath string) error {
	isExist, err := util.PathExist(filePath)
	if err != nil {
		return err
	}
	if !isExist {
		return pkgGen.TemplateGenerator.Generate(router, middlewareTpl, filePath, false)
	}
	middlewareList := make([]string, 1)

	_ = router.(Router).Router.DFS(0, func(layer int, node *RouterNode) error {
		middlewareList = append(middlewareList, node.MiddleWare)
		return nil
	})

	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	for _, mw := range middlewareList {
		if bytes.Contains(file, []byte(mw+"Mw")) {
			continue
		}
		middlewareSingleTpl := pkgGen.tpls[middlewareSingleTplName]
		if middlewareSingleTpl == nil {
			return fmt.Errorf("tpl %s not found", middlewareSingleTplName)
		}
		data := make(map[string]string, 1)
		data["MiddleWare"] = mw
		middlewareFunc := bytes.NewBuffer(nil)
		err = middlewareSingleTpl.Execute(middlewareFunc, data)
		if err != nil {
			return fmt.Errorf("execute template \"%s\" failed, %v", middlewareSingleTplName, err)
		}

		buf := bytes.NewBuffer(nil)
		_, err = buf.Write(file)
		if err != nil {
			return fmt.Errorf("write middleware \"%s\" failed, %v", mw, err)
		}
		_, err = buf.Write(middlewareFunc.Bytes())
		if err != nil {
			return fmt.Errorf("write middleware \"%s\" failed, %v", mw, err)
		}
		file = buf.Bytes()
	}

	pkgGen.files = append(pkgGen.files, File{filePath, string(file), false, middlewareTplName})

	return nil
}
