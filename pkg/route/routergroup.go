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
 * The MIT License (MIT)
 *
 * Copyright (c) 2014 Manuel Martínez-Almeida
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package route

import (
	"context"
	"path"
	"regexp"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	rConsts "github.com/cloudwego/hertz/pkg/route/consts"
)

// IRouter defines all router handle interface includes single and group router.
type IRouter interface {
	IRoutes
	Group(string, ...app.HandlerFunc) *RouterGroup
}

// IRoutes defines all router handle interface.
type IRoutes interface {
	Use(...app.HandlerFunc) IRoutes
	Handle(string, string, ...app.HandlerFunc) IRoutes
	Any(string, ...app.HandlerFunc) IRoutes
	GET(string, ...app.HandlerFunc) IRoutes
	POST(string, ...app.HandlerFunc) IRoutes
	DELETE(string, ...app.HandlerFunc) IRoutes
	PATCH(string, ...app.HandlerFunc) IRoutes
	PUT(string, ...app.HandlerFunc) IRoutes
	OPTIONS(string, ...app.HandlerFunc) IRoutes
	HEAD(string, ...app.HandlerFunc) IRoutes
	StaticFile(string, string) IRoutes
	Static(string, string) IRoutes
	StaticFS(string, *app.FS) IRoutes
}

// RouterGroup is used internally to configure router, a RouterGroup is associated with
// a prefix and an array of handlers (middleware).
type RouterGroup struct {
	Handlers app.HandlersChain
	basePath string
	engine   *Engine
	root     bool
}

var _ IRouter = (*RouterGroup)(nil)

// Use adds middleware to the group, see example code in GitHub.
func (group *RouterGroup) Use(middleware ...app.HandlerFunc) IRoutes {
	group.Handlers = append(group.Handlers, middleware...)
	return group.returnObj()
}

// Group creates a new router group. You should add all the routes that have common middlewares or the same path prefix.
// For example, all the routes that use a common middleware for authorization could be grouped.
func (group *RouterGroup) Group(relativePath string, handlers ...app.HandlerFunc) *RouterGroup {
	return &RouterGroup{
		Handlers: group.combineHandlers(handlers),
		basePath: group.calculateAbsolutePath(relativePath),
		engine:   group.engine,
	}
}

// BasePath returns the base path of router group.
// For example, if v := router.Group("/rest/n/v1/api"), v.BasePath() is "/rest/n/v1/api".
func (group *RouterGroup) BasePath() string {
	return group.basePath
}

func (group *RouterGroup) handle(httpMethod, relativePath string, handlers app.HandlersChain) IRoutes {
	absolutePath := group.calculateAbsolutePath(relativePath)
	handlers = group.combineHandlers(handlers)
	group.engine.addRoute(httpMethod, absolutePath, handlers)
	return group.returnObj()
}

var upperLetterReg = regexp.MustCompile("^[A-Z]+$")

// Handle registers a new request handle and middleware with the given path and method.
// The last handler should be the real handler, the other ones should be middleware that can and should be shared among different routes.
// See the example code in GitHub.
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
//
// This function is intended for bulk loading and to allow the usage of less
// frequently used, non-standardized or custom methods (e.g. for internal
// communication with a proxy).
func (group *RouterGroup) Handle(httpMethod, relativePath string, handlers ...app.HandlerFunc) IRoutes {
	if matches := upperLetterReg.MatchString(httpMethod); !matches {
		panic("http method " + httpMethod + " is not valid")
	}
	return group.handle(httpMethod, relativePath, handlers)
}

// POST is a shortcut for router.Handle("POST", path, handle).
func (group *RouterGroup) POST(relativePath string, handlers ...app.HandlerFunc) IRoutes {
	return group.handle(consts.MethodPost, relativePath, handlers)
}

// GET is a shortcut for router.Handle("GET", path, handle).
func (group *RouterGroup) GET(relativePath string, handlers ...app.HandlerFunc) IRoutes {
	return group.handle(consts.MethodGet, relativePath, handlers)
}

// DELETE is a shortcut for router.Handle("DELETE", path, handle).
func (group *RouterGroup) DELETE(relativePath string, handlers ...app.HandlerFunc) IRoutes {
	return group.handle(consts.MethodDelete, relativePath, handlers)
}

// PATCH is a shortcut for router.Handle("PATCH", path, handle).
func (group *RouterGroup) PATCH(relativePath string, handlers ...app.HandlerFunc) IRoutes {
	return group.handle(consts.MethodPatch, relativePath, handlers)
}

// PUT is a shortcut for router.Handle("PUT", path, handle).
func (group *RouterGroup) PUT(relativePath string, handlers ...app.HandlerFunc) IRoutes {
	return group.handle(consts.MethodPut, relativePath, handlers)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, handle).
func (group *RouterGroup) OPTIONS(relativePath string, handlers ...app.HandlerFunc) IRoutes {
	return group.handle(consts.MethodOptions, relativePath, handlers)
}

// HEAD is a shortcut for router.Handle("HEAD", path, handle).
func (group *RouterGroup) HEAD(relativePath string, handlers ...app.HandlerFunc) IRoutes {
	return group.handle(consts.MethodHead, relativePath, handlers)
}

// Any registers a route that matches all the HTTP methods.
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.
func (group *RouterGroup) Any(relativePath string, handlers ...app.HandlerFunc) IRoutes {
	group.handle(consts.MethodGet, relativePath, handlers)
	group.handle(consts.MethodPost, relativePath, handlers)
	group.handle(consts.MethodPut, relativePath, handlers)
	group.handle(consts.MethodPatch, relativePath, handlers)
	group.handle(consts.MethodHead, relativePath, handlers)
	group.handle(consts.MethodOptions, relativePath, handlers)
	group.handle(consts.MethodDelete, relativePath, handlers)
	group.handle(consts.MethodConnect, relativePath, handlers)
	group.handle(consts.MethodTrace, relativePath, handlers)
	return group.returnObj()
}

// StaticFile registers a single route in order to Serve a single file of the local filesystem.
// router.StaticFile("favicon.ico", "./resources/favicon.ico")
func (group *RouterGroup) StaticFile(relativePath, filepath string) IRoutes {
	if strings.Contains(relativePath, ":") || strings.Contains(relativePath, "*") {
		panic("URL parameters can not be used when serving a static file")
	}
	handler := func(c context.Context, ctx *app.RequestContext) {
		ctx.File(filepath)
	}
	group.GET(relativePath, handler)
	group.HEAD(relativePath, handler)
	return group.returnObj()
}

// Static serves files from the given file system root.
// To use the operating system's file system implementation,
// use :
//
//	router.Static("/static", "/var/www")
func (group *RouterGroup) Static(relativePath, root string) IRoutes {
	return group.StaticFS(relativePath, &app.FS{Root: root})
}

// StaticFS works just like `Static()` but a custom `FS` can be used instead.
func (group *RouterGroup) StaticFS(relativePath string, fs *app.FS) IRoutes {
	if strings.Contains(relativePath, ":") || strings.Contains(relativePath, "*") {
		panic("URL parameters can not be used when serving a static folder")
	}
	handler := fs.NewRequestHandler()
	urlPattern := path.Join(relativePath, "/*filepath")

	// Register GET and HEAD handlers
	group.GET(urlPattern, handler)
	group.HEAD(urlPattern, handler)
	return group.returnObj()
}

func (group *RouterGroup) combineHandlers(handlers app.HandlersChain) app.HandlersChain {
	finalSize := len(group.Handlers) + len(handlers)
	if finalSize >= int(rConsts.AbortIndex) {
		panic("too many handlers")
	}
	mergedHandlers := make(app.HandlersChain, finalSize)
	copy(mergedHandlers, group.Handlers)
	copy(mergedHandlers[len(group.Handlers):], handlers)
	return mergedHandlers
}

func (group *RouterGroup) calculateAbsolutePath(relativePath string) string {
	return joinPaths(group.basePath, relativePath)
}

func (group *RouterGroup) returnObj() IRoutes {
	if group.root {
		return group.engine
	}
	return group
}

// GETEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) GETEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.GET(relativePath, handler)
}

// POSTEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) POSTEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.POST(relativePath, handler)
}

// PUTEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) PUTEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.PUT(relativePath, handler)
}

// DELETEEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) DELETEEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.DELETE(relativePath, handler)
}

// HEADEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) HEADEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.HEAD(relativePath, handler)
}

// AnyEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) AnyEX(relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.Any(relativePath, handler)
}

// HandleEX adds a handlerName param. When handler is decorated or handler is an anonymous function,
// Hertz cannot get handler name directly. In this case, pass handlerName explicitly.
func (group *RouterGroup) HandleEX(httpMethod, relativePath string, handler app.HandlerFunc, handlerName string) IRoutes {
	app.SetHandlerName(handler, handlerName)
	return group.Handle(httpMethod, relativePath, handler)
}

func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}

	finalPath := path.Join(absolutePath, relativePath)
	appendSlash := lastChar(relativePath) == '/' && lastChar(finalPath) != '/'
	if appendSlash {
		return finalPath + "/"
	}
	return finalPath
}

func lastChar(str string) uint8 {
	if str == "" {
		panic("The length of the string can't be 0")
	}
	return str[len(str)-1]
}
