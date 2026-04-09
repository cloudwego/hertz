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
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/cloudwego/hertz/cmd/hz/util"
)

// Router holds the data needed to render the router registration template.
type Router struct {
	FilePath        string
	PackageName     string
	HandlerPackages map[string]string // package alias -> import path
	Router          *RouterNode
}

// RouterNode represents a node in the routing tree. The tree structure mirrors
// the URL path hierarchy. Each node may have:
//   - Children: sub-path segments forming route groups
//   - Handler + HttpMethod: a leaf endpoint registration
//
// The tree is used to generate nested router group code with middleware hooks.
type RouterNode struct {
	GroupName         string // parent's middleware var name, used for route registration (e.g. root.GET)
	MiddleWare        string // this node's middleware var name, becomes GroupName for children
	HandlerMiddleware string // middleware name specific to this handler (may differ from GroupMiddleware)
	GroupMiddleware   string // middleware name for the route group (may use snake_case or camelCase)
	PathPrefix        string // accumulated path prefix for snake-style middleware naming

	Path     string
	Parent   *RouterNode
	Children childrenRouterInfo

	Handler             string // qualified handler call: "pkg.HandlerName"
	HandlerPackage      string // handler import path
	HandlerPackageAlias string // handler import alias
	HttpMethod          string // HTTP method (GET, POST, etc.) or "" for group-only nodes
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

// Update inserts a new route into the tree. It splits the path into segments,
// finds the deepest existing node matching the path prefix, then inserts
// remaining segments as new child nodes.
func (routerNode *RouterNode) Update(method *HttpMethod, handlerType, handlerPkg string, sortRouter bool) error {
	if method.Path == "" {
		return fmt.Errorf("empty path for method '%s'", method.Name)
	}
	paths := strings.Split(method.Path, "/")
	if paths[0] == "" {
		paths = paths[1:]
	}
	parent, last := routerNode.FindNearest(paths, method.HTTPMethod, sortRouter)
	if last == len(paths) {
		return fmt.Errorf("path '%s' has been registered", method.Path)
	}
	name := util.ToVarName(paths[:last])
	parent.Insert(name, method, handlerType, paths[last:], handlerPkg, sortRouter)
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

// handlerPkgMap tracks handler package -> unique alias mappings across the entire
// generation session to ensure each handler package gets a unique import alias.
var handlerPkgMap map[string]string

// Insert adds child nodes for each remaining path segment. The last segment
// gets the handler and HTTP method assignment.
func (routerNode *RouterNode) Insert(name string, method *HttpMethod, handlerType string, paths []string, handlerPkg string, sortRouter bool) {
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
				if len(method.RefPackage) != 0 {
					c.Handler = method.RefPackageAlias + "." + method.Name
					c.HandlerPackageAlias = method.RefPackageAlias
					c.HandlerPackage = method.RefPackage
				}
			}
			c.HttpMethod = getHttpMethod(method.HTTPMethod)
		}
		if cur.Children == nil {
			cur.Children = make([]*RouterNode, 0, 1)
		}
		cur.Children = append(cur.Children, c)
		if sortRouter {
			sort.Sort(cur.Children)
		}
		cur = c
	}
}

func getHttpMethod(method string) string {
	if strings.EqualFold(method, "Any") {
		return "Any"
	}
	return strings.ToUpper(method)
}

// FindNearest walks the tree to find the deepest node matching the given path segments.
// Returns the parent node and the index of the first unmatched path segment.
// When sortRouter is true, it also checks HTTP method to distinguish same-path different-method routes.
func (routerNode *RouterNode) FindNearest(paths []string, method string, sortRouter bool) (*RouterNode, int) {
	ns := len(paths)
	cur := routerNode
	i := 0
	path := paths[i]
	for j := 0; j < len(cur.Children); j++ {
		c := cur.Children[j]
		tmpMethod := "" // group do not have http method
		if i == ns {    // only i==ns, the path is http method node
			tmpMethod = method
		}
		if ("/" + path) == c.Path {
			if sortRouter && !strings.EqualFold(c.HttpMethod, tmpMethod) {
				continue
			}
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
// Less sorts children: handler nodes (with HttpMethod) come before group nodes (without),
// then alphabetically by path (ignoring leading non-letter chars like "/" and ":").
func (c childrenRouterInfo) Less(i, j int) bool {
	if c[i].HttpMethod == "" && c[j].HttpMethod != "" {
		return false
	}
	if c[i].HttpMethod != "" && c[j].HttpMethod == "" {
		return true
	}
	// remove non-litter char
	// eg. /a -> a
	//     /:a -> a
	ci := removeNonLetterPrefix(c[i].Path)
	cj := removeNonLetterPrefix(c[j].Path)

	// if ci == cj, use HTTP method for sort, preventing sorting inconsistencies
	if ci == cj {
		return c[i].HttpMethod < c[j].HttpMethod
	}

	return ci < cj
}

func removeNonLetterPrefix(str string) string {
	for i, char := range str {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			return str[i:]
		}
	}
	return str
}

// Swap swaps the elements with indexes i and j.
func (c childrenRouterInfo) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

var (
	regRegisterV3 = regexp.MustCompile(insertPointPatternNew)
	regImport     = regexp.MustCompile(`import \(\n`)
)

// updateRegister adds a new sub-router package registration call to the register.go file.
// It inserts both the import statement and the registration call at the marked insert point.
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

	insertReg := register.DepPkgAlias + ".Register(r)\n"

	if !checkDupRegister(file, insertReg) {
		file, err = util.AddImport(registerPath, register.DepPkgAlias, register.DepPkg)
		if err != nil {
			return err
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

func checkDupRegister(file []byte, insertReg string) bool {
	return bytes.Contains(file, []byte("\t"+insertReg)) || bytes.Contains(file, []byte(" "+insertReg))
}

// appendMw appends a middleware name to the list, appending a numeric suffix if the name
// already exists to ensure uniqueness (e.g. "mw", "mw0", "mw1", ...).
func appendMw(mws []string, mw string) ([]string, string) {
	for i := 0; true; i++ {
		if i == math.MaxInt {
			break
		}
		if !stringsIncludes(mws, mw) {
			mws = append(mws, mw)
			break
		}
		mw += strconv.Itoa(i)
	}
	return mws, mw
}

func stringsIncludes(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}
	return false
}

// genRouter generates the router registration file, middleware stubs, and register.go entries.
// It first assigns unique middleware names to each tree node via DyeGroupName, then renders
// the router template and updates middleware and registration files incrementally.
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

	handlerMap := make(map[string]string)
	hook := func(layer int, node *RouterNode) error {
		if len(node.HandlerPackage) != 0 {
			handlerMap[node.HandlerPackageAlias] = node.HandlerPackage
		}
		return nil
	}
	root.DFS(0, hook)
	if len(handlerMap) != 0 {
		router.HandlerPackages = handlerMap
	}

	if pkgGen.SnakeStyleMiddleware { // unique middleware name for SnakeStyleMiddleware
		mws := []string{}
		hook := func(layer int, node *RouterNode) error {
			if len(node.Children) == 0 {
				return nil
			}
			groupMwName := node.GroupMiddleware
			handlerMwName := node.HandlerMiddleware
			if len(groupMwName) != 0 {
				mws, groupMwName = appendMw(mws, groupMwName)
			}
			if len(handlerMwName) != 0 {
				mws, handlerMwName = appendMw(mws, handlerMwName)
			}
			if groupMwName != node.GroupMiddleware {
				node.GroupMiddleware = groupMwName
			}
			if handlerMwName != node.HandlerMiddleware {
				node.HandlerMiddleware = handlerMwName
			}
			return nil
		}
		root.DFS(0, hook)
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

// updateMiddlewareReg appends new middleware stub functions to an existing middleware.go file.
// Each middleware function is only added if its name pattern doesn't already appear in the file.
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
