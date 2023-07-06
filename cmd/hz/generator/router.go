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
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/util"
)

type Router struct {
	FilePath        string
	PackageName     string
	HandlerPackages map[string]string // {{basename}}:{{import_path}}
	Router          *RouterNode
}

type RouterNode struct {
	GroupName         string // current group name(the parent middleware name), used to register route. example: {{.GroupName}}.{{HttpMethod}}
	MiddleWare        string // current node middleware, used to be group name for children.
	HandlerMiddleware string
	GroupMiddleware   string
	PathPrefix        string

	Path     string
	Parent   *RouterNode
	Children childrenRouterInfo

	Handler             string // {{HandlerPackage}}.{{HandlerName}}
	HandlerPackage      string
	HandlerPackageAlias string
	HttpMethod          string
}

type RegisterInfo struct {
	PackageName string
	DepPkgAlias string
	DepPkg      string
}

// NewRouterTree contains "/" as root node
func NewRouterTree() *RouterNode {
	return &RouterNode{
		GroupName:       "root",
		MiddleWare:      "root",
		GroupMiddleware: "root",
		Path:            "/",
		Parent:          nil,
	}
}

func (routerNode *RouterNode) Sort() {
	sort.Sort(routerNode.Children)
}

func (routerNode *RouterNode) Update(method *HttpMethod, handlerType, handlerPkg string) error {
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
	parent.Insert(name, method, handlerType, paths[last:], handlerPkg)
	parent.Sort()
	return nil
}

func (routerNode *RouterNode) RawHandlerName() string {
	parts := strings.Split(routerNode.Handler, ".")
	handlerName := parts[len(parts)-1]
	return handlerName
}

// DyeGroupName traverses the routing tree in depth and names the handler/group middleware for each node.
// If snakeStyleMiddleware is set to true, the name style of the middleware will use snake name style.
func (routerNode *RouterNode) DyeGroupName(snakeStyleMiddleware bool) error {
	groups := []string{"root"}

	hook := func(layer int, node *RouterNode) error {
		node.GroupName = groups[layer]
		if node.MiddleWare == "" {
			pname := node.Path
			if len(pname) > 1 && pname[0] == '/' {
				pname = pname[1:]
			}

			if node.Parent != nil {
				node.PathPrefix = node.Parent.PathPrefix + "_" + util.ToGoFuncName(pname)
			} else {
				node.PathPrefix = "_" + util.ToGoFuncName(pname)
			}

			handlerMiddlewareName := ""
			isLeafNode := false
			if len(node.Handler) != 0 {
				handlerMiddlewareName = node.RawHandlerName()
				// If it is a leaf node, then "group middleware name" and "handler middleware name" are the same
				if len(node.Children) == 0 {
					pname = handlerMiddlewareName
					isLeafNode = true
				}
			}

			pname = convertToMiddlewareName(pname)
			handlerMiddlewareName = convertToMiddlewareName(handlerMiddlewareName)

			if isLeafNode {
				name, err := util.GetMiddlewareUniqueName(pname)
				if err != nil {
					return fmt.Errorf("get unique name for middleware '%s' failed, err: %v", name, err)
				}
				pname = name
				handlerMiddlewareName = name
			} else {
				var err error
				pname, err = util.GetMiddlewareUniqueName(pname)
				if err != nil {
					return fmt.Errorf("get unique name for middleware '%s' failed, err: %v", pname, err)
				}
				handlerMiddlewareName, err = util.GetMiddlewareUniqueName(handlerMiddlewareName)
				if err != nil {
					return fmt.Errorf("get unique name for middleware '%s' failed, err: %v", handlerMiddlewareName, err)
				}
			}
			node.MiddleWare = "_" + pname
			if len(node.Handler) != 0 {
				node.HandlerMiddleware = "_" + handlerMiddlewareName
				if snakeStyleMiddleware {
					node.HandlerMiddleware = "_" + node.RawHandlerName()
				}
			}
			node.GroupMiddleware = node.MiddleWare
			if snakeStyleMiddleware {
				node.GroupMiddleware = node.PathPrefix
			}
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

var handlerPkgMap map[string]string

func (routerNode *RouterNode) Insert(name string, method *HttpMethod, handlerType string, paths []string, handlerPkg string) {
	cur := routerNode
	for i, p := range paths {
		c := &RouterNode{
			Path:   "/" + p,
			Parent: cur,
		}
		if i == len(paths)-1 {
			// generate handler by method
			if len(handlerPkg) != 0 {
				// get a unique package alias for every handler
				pkgAlias := filepath.Base(handlerPkg)
				pkgAlias = util.ToVarName([]string{pkgAlias})
				val, exist := handlerPkgMap[handlerPkg]
				if !exist {
					pkgAlias, _ = util.GetHandlerPackageUniqueName(pkgAlias)
					if len(handlerPkgMap) == 0 {
						handlerPkgMap = make(map[string]string, 10)
					}
					handlerPkgMap[handlerPkg] = pkgAlias
				} else {
					pkgAlias = val
				}
				c.HandlerPackageAlias = pkgAlias
				c.Handler = pkgAlias + "." + method.Name
				c.HandlerPackage = handlerPkg
				method.RefPackage = c.HandlerPackage
				method.RefPackageAlias = c.HandlerPackageAlias
			} else { // generate handler by service
				c.Handler = handlerType + "." + method.Name
			}
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
	if pkgGen.tplsInfo[registerTplName].Disable {
		return nil
	}
	register := RegisterInfo{
		PackageName: filepath.Base(rDir),
		DepPkgAlias: strings.ReplaceAll(pkgName, "/", "_"),
		DepPkg:      pkg,
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

	if !bytes.Contains(file, []byte(register.DepPkg)) {
		file, err = util.AddImport(registerPath, register.DepPkgAlias, register.DepPkg)
		if err != nil {
			return err
		}

		insertReg := register.DepPkgAlias + ".Register(r)\n"
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
	err := root.DyeGroupName(pkgGen.SnakeStyleMiddleware)
	if err != nil {
		return err
	}
	router := Router{
		FilePath:    filepath.Join(routerDir, util.BaseNameAndTrim(pkg.IdlName)+".go"),
		PackageName: filepath.Base(routerDir),
		HandlerPackages: map[string]string{
			util.BaseName(handlerPackage, ""): handlerPackage,
		},
		Router: root,
	}

	if pkgGen.HandlerByMethod {
		handlerMap := make(map[string]string, 1)
		hook := func(layer int, node *RouterNode) error {
			if len(node.HandlerPackage) != 0 {
				handlerMap[node.HandlerPackageAlias] = node.HandlerPackage
			}
			return nil
		}
		root.DFS(0, hook)
		router.HandlerPackages = handlerMap
	}

	// store router info
	pkg.RouterInfo = &router

	if !pkgGen.tplsInfo[routerTplName].Disable {
		if err := pkgGen.TemplateGenerator.Generate(router, routerTplName, router.FilePath, false); err != nil {
			return fmt.Errorf("generate router %s failed, err: %v", router.FilePath, err.Error())
		}
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
	if pkgGen.tplsInfo[middlewareTpl].Disable {
		return nil
	}
	isExist, err := util.PathExist(filePath)
	if err != nil {
		return err
	}
	if !isExist {
		return pkgGen.TemplateGenerator.Generate(router, middlewareTpl, filePath, false)
	}
	var middlewareList []string

	_ = router.(Router).Router.DFS(0, func(layer int, node *RouterNode) error {
		// non-leaf node will generate group middleware
		if node.Children.Len() > 0 && len(node.GroupMiddleware) > 0 {
			middlewareList = append(middlewareList, node.GroupMiddleware)
		}
		if len(node.HandlerMiddleware) > 0 {
			middlewareList = append(middlewareList, node.HandlerMiddleware)
		}
		return nil
	})

	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	for _, mw := range middlewareList {
		mwNamePattern := fmt.Sprintf(" %sMw", mw)
		if pkgGen.SnakeStyleMiddleware {
			mwNamePattern = fmt.Sprintf(" %s_mw", mw)
		}
		if bytes.Contains(file, []byte(mwNamePattern)) {
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

// convertToMiddlewareName converts a route path to a middleware name
func convertToMiddlewareName(path string) string {
	path = util.ToVarName([]string{path})
	path = strings.ToLower(path)
	return path
}
