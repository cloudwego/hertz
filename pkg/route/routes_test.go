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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type header struct {
	Key   string
	Value string
}

func performRequest(e *Engine, method, path string, headers ...header) *httptest.ResponseRecorder {
	ctx := e.ctxPool.Get().(*app.RequestContext)
	ctx.HTMLRender = e.htmlRender

	r := protocol.NewRequest(method, path, nil)
	r.CopyTo(&ctx.Request)
	for _, v := range headers {
		ctx.Request.Header.Add(v.Key, v.Value)
	}

	e.ServeHTTP(context.Background(), ctx)

	w := httptest.NewRecorder()
	h := w.Header()
	ctx.Response.Header.VisitAll(func(key, value []byte) {
		h.Add(string(key), string(value))
	})
	w.WriteHeader(ctx.Response.StatusCode())
	if _, err := w.Write(ctx.Response.Body()); err != nil {
		panic(err.Error())
	}
	ctx.Reset()
	e.ctxPool.Put(ctx)

	return w
}

func testRouteOK(method string, t *testing.T) {
	passed := false
	passedAny := false
	r := NewEngine(config.NewOptions(nil))
	r.Any("/test2", func(c context.Context, ctx *app.RequestContext) {
		passedAny = true
	})
	r.Handle(method, "/test", func(c context.Context, ctx *app.RequestContext) {
		passed = true
	})

	w := performRequest(r, method, "/test")
	assert.DeepEqual(t, true, passed)
	assert.DeepEqual(t, consts.StatusOK, w.Code)

	performRequest(r, method, "/test2")
	assert.DeepEqual(t, true, passedAny)
}

// TestSingleRouteOK tests that POST route is correctly invoked.
func testRouteNotOK(method string, t *testing.T) {
	passed := false
	router := NewEngine(config.NewOptions(nil))
	router.Handle(method, "/test_2", func(c context.Context, ctx *app.RequestContext) {
		passed = true
	})

	w := performRequest(router, method, "/test")

	assert.DeepEqual(t, false, passed)
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
}

// TestSingleRouteOK tests that POST route is correctly invoked.
func testRouteNotOK2(method string, t *testing.T) {
	passed := false
	router := NewEngine(config.NewOptions(nil))
	router.options.HandleMethodNotAllowed = true
	var methodRoute string
	if method == consts.MethodPost {
		methodRoute = consts.MethodGet
	} else {
		methodRoute = consts.MethodPost
	}
	router.Handle(methodRoute, "/test", func(c context.Context, ctx *app.RequestContext) {
		passed = true
	})

	w := performRequest(router, method, "/test")

	assert.DeepEqual(t, false, passed)
	assert.DeepEqual(t, consts.StatusMethodNotAllowed, w.Code)
}

func testRouteNotOK3(method string, t *testing.T) {
	passed := false
	router := NewEngine(config.NewOptions(nil))
	router.Handle("GET", "/api/v:version/product/local/products/list", func(c context.Context, ctx *app.RequestContext) {
		passed = true
	})
	router.Handle("GET", "/api/v:version/product/product_creation/preload_all_categories", func(c context.Context, ctx *app.RequestContext) {
		passed = true
	})

	w := performRequest(router, method, "/api/v1/product/products/activate")

	assert.DeepEqual(t, false, passed)
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
}

func TestRouterMethod(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.PUT("/hey2", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusOK, "sup2")
	})

	router.PUT("/hey", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusOK, "called")
	})

	router.PUT("/hey3", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusOK, "sup3")
	})

	w := performRequest(router, consts.MethodPut, "/hey")

	assert.DeepEqual(t, consts.StatusOK, w.Code)
	assert.DeepEqual(t, "called", w.Body.String())
}

func TestRouterGroupRouteOK(t *testing.T) {
	testRouteOK(consts.MethodGet, t)
	testRouteOK(consts.MethodPost, t)
	testRouteOK(consts.MethodPut, t)
	testRouteOK(consts.MethodPatch, t)
	testRouteOK(consts.MethodHead, t)
	testRouteOK(consts.MethodOptions, t)
	testRouteOK(consts.MethodDelete, t)
	testRouteOK(consts.MethodConnect, t)
	testRouteOK(consts.MethodTrace, t)
}

func TestRouteNotOK1(t *testing.T) {
	testRouteNotOK(consts.MethodGet, t)
	testRouteNotOK(consts.MethodPost, t)
	testRouteNotOK(consts.MethodPut, t)
	testRouteNotOK(consts.MethodPatch, t)
	testRouteNotOK(consts.MethodHead, t)
	testRouteNotOK(consts.MethodOptions, t)
	testRouteNotOK(consts.MethodDelete, t)
	testRouteNotOK(consts.MethodConnect, t)
	testRouteNotOK(consts.MethodTrace, t)
}

func TestRouteNotOK2(t *testing.T) {
	testRouteNotOK2(consts.MethodGet, t)
	testRouteNotOK2(consts.MethodPost, t)
	testRouteNotOK2(consts.MethodPut, t)
	testRouteNotOK2(consts.MethodPatch, t)
	testRouteNotOK2(consts.MethodHead, t)
	testRouteNotOK2(consts.MethodOptions, t)
	testRouteNotOK2(consts.MethodDelete, t)
	testRouteNotOK2(consts.MethodConnect, t)
	testRouteNotOK2(consts.MethodTrace, t)
}

func TestRouteNotOK3(t *testing.T) {
	testRouteNotOK3(consts.MethodGet, t)
	testRouteNotOK3(consts.MethodPost, t)
	testRouteNotOK3(consts.MethodPut, t)
	testRouteNotOK3(consts.MethodPatch, t)
	testRouteNotOK3(consts.MethodHead, t)
	testRouteNotOK3(consts.MethodOptions, t)
	testRouteNotOK3(consts.MethodDelete, t)
	testRouteNotOK3(consts.MethodConnect, t)
	testRouteNotOK3(consts.MethodTrace, t)
}

func TestRouteRedirectTrailingSlash(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.options.RedirectFixedPath = false
	router.options.RedirectTrailingSlash = true
	router.GET("/path", func(c context.Context, ctx *app.RequestContext) {})
	router.GET("/path2/", func(c context.Context, ctx *app.RequestContext) {})
	router.POST("/path3", func(c context.Context, ctx *app.RequestContext) {})
	router.PUT("/path4/", func(c context.Context, ctx *app.RequestContext) {})

	w := performRequest(router, consts.MethodGet, "/path/")
	assert.DeepEqual(t, "/path", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusMovedPermanently, w.Code)

	w = performRequest(router, consts.MethodGet, "/path2")
	assert.DeepEqual(t, "/path2/", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusMovedPermanently, w.Code)

	w = performRequest(router, consts.MethodPost, "/path3/")
	assert.DeepEqual(t, "/path3", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusTemporaryRedirect, w.Code)

	w = performRequest(router, consts.MethodPut, "/path4")
	assert.DeepEqual(t, "/path4/", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusTemporaryRedirect, w.Code)

	w = performRequest(router, consts.MethodGet, "/path")
	assert.DeepEqual(t, consts.StatusOK, w.Code)

	w = performRequest(router, consts.MethodGet, "/path2/")
	assert.DeepEqual(t, consts.StatusOK, w.Code)

	w = performRequest(router, consts.MethodPost, "/path3")
	assert.DeepEqual(t, consts.StatusOK, w.Code)

	w = performRequest(router, consts.MethodPut, "/path4/")
	assert.DeepEqual(t, consts.StatusOK, w.Code)

	w = performRequest(router, consts.MethodGet, "/path2", header{Key: "X-Forwarded-Prefix", Value: "/api"})
	assert.DeepEqual(t, "/api/path2/", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusMovedPermanently, w.Code)

	w = performRequest(router, consts.MethodGet, "/path2/", header{Key: "X-Forwarded-Prefix", Value: "/api/"})
	assert.DeepEqual(t, consts.StatusOK, w.Code)

	router.options.RedirectTrailingSlash = false

	w = performRequest(router, consts.MethodGet, "/path/")
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
	w = performRequest(router, consts.MethodGet, "/path2")
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
	w = performRequest(router, consts.MethodPost, "/path3/")
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
	w = performRequest(router, consts.MethodPut, "/path4")
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
}

func TestRouteRedirectTrailingSlashWithQuery(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.options.RedirectFixedPath = false
	router.options.RedirectTrailingSlash = true
	router.GET("/path", func(c context.Context, ctx *app.RequestContext) {})
	router.GET("/path2/", func(c context.Context, ctx *app.RequestContext) {})

	w := performRequest(router, consts.MethodGet, "/path/?offset=2")
	assert.DeepEqual(t, "/path?offset=2", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusMovedPermanently, w.Code)

	w = performRequest(router, consts.MethodGet, "/path2?offset=2")
	assert.DeepEqual(t, "/path2/?offset=2", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusMovedPermanently, w.Code)
}

func TestRouteRedirectFixedPath(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.options.RedirectFixedPath = true
	router.options.RedirectTrailingSlash = false

	router.GET("/path", func(c context.Context, ctx *app.RequestContext) {})
	router.GET("/Path2", func(c context.Context, ctx *app.RequestContext) {})
	router.POST("/PATH3", func(c context.Context, ctx *app.RequestContext) {})
	router.POST("/Path4/", func(c context.Context, ctx *app.RequestContext) {})

	w := performRequest(router, consts.MethodGet, "/PATH")
	assert.DeepEqual(t, "/path", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusMovedPermanently, w.Code)

	w = performRequest(router, consts.MethodGet, "/path2")
	assert.DeepEqual(t, "/Path2", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusMovedPermanently, w.Code)

	w = performRequest(router, consts.MethodPost, "/path3")
	assert.DeepEqual(t, "/PATH3", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusTemporaryRedirect, w.Code)

	w = performRequest(router, consts.MethodPost, "/path4")
	assert.DeepEqual(t, "/Path4/", w.Header().Get("Location"))
	assert.DeepEqual(t, consts.StatusTemporaryRedirect, w.Code)
}

// TestContextParamsGet tests that a parameter can be parsed from the URL.
func TestRouteParamsByName(t *testing.T) {
	name := ""
	lastName := ""
	wild := ""
	router := NewEngine(config.NewOptions(nil))
	router.GET("/test/:name/:last_name/*wild", func(c context.Context, ctx *app.RequestContext) {
		name = ctx.Params.ByName("name")
		lastName = ctx.Params.ByName("last_name")
		var ok bool
		wild, ok = ctx.Params.Get("wild")

		assert.DeepEqual(t, true, ok)
		assert.DeepEqual(t, name, ctx.Param("name"))
		assert.DeepEqual(t, name, ctx.Param("name"))
		assert.DeepEqual(t, lastName, ctx.Param("last_name"))

		assert.DeepEqual(t, "", ctx.Param("wtf"))
		assert.DeepEqual(t, "", ctx.Params.ByName("wtf"))

		wtf, ok := ctx.Params.Get("wtf")
		assert.DeepEqual(t, "", wtf)
		assert.False(t, ok)
	})

	w := performRequest(router, consts.MethodGet, "/test/john/smith/is/super/great")

	assert.DeepEqual(t, consts.StatusOK, w.Code)
	assert.DeepEqual(t, "john", name)
	assert.DeepEqual(t, "smith", lastName)
	assert.DeepEqual(t, "is/super/great", wild)
}

// TestContextParamsGet tests that a parameter can be parsed from the URL even with extra slashes.
func TestRouteParamsByNameWithExtraSlash(t *testing.T) {
	name := ""
	lastName := ""
	wild := ""
	router := NewEngine(config.NewOptions(nil))
	router.options.RemoveExtraSlash = true
	router.GET("/test/:name/:last_name/*wild", func(c context.Context, ctx *app.RequestContext) {
		name = ctx.Params.ByName("name")
		lastName = ctx.Params.ByName("last_name")
		var ok bool
		wild, ok = ctx.Params.Get("wild")

		assert.True(t, ok)
		assert.DeepEqual(t, name, ctx.Param("name"))
		assert.DeepEqual(t, name, ctx.Param("name"))
		assert.DeepEqual(t, lastName, ctx.Param("last_name"))

		assert.DeepEqual(t, "", ctx.Param("wtf"))
		assert.DeepEqual(t, "", ctx.Params.ByName("wtf"))

		wtf, ok := ctx.Params.Get("wtf")
		assert.DeepEqual(t, "", wtf)
		assert.False(t, ok)
	})

	w := performRequest(router, consts.MethodGet, "/test//john//smith//is//super//great")

	assert.DeepEqual(t, consts.StatusOK, w.Code)
	assert.DeepEqual(t, "john", name)
	assert.DeepEqual(t, "smith", lastName)
	assert.DeepEqual(t, "is/super/great", wild)
}

// TestHandleStaticFile - ensure the static file handles properly
func TestRouteStaticFile(t *testing.T) {
	// SETUP file
	testRoot, _ := os.Getwd()
	f, err := ioutil.TempFile(testRoot, "")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString("Hertz Web Framework")
	assert.Nil(t, err)
	f.Close()

	dir, filename := filepath.Split(f.Name())

	// SETUP engine
	router := NewEngine(config.NewOptions(nil))
	router.StaticFS("/using_static", &app.FS{Root: dir, AcceptByteRange: true, PathRewrite: app.NewPathSlashesStripper(1)})
	router.StaticFile("/result", f.Name())

	w := performRequest(router, consts.MethodGet, "/using_static/"+filename)
	w2 := performRequest(router, consts.MethodGet, "/result")

	assert.DeepEqual(t, w, w2)
	assert.DeepEqual(t, consts.StatusOK, w.Code)
	assert.DeepEqual(t, "Hertz Web Framework", w.Body.String())
	assert.DeepEqual(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

	w3 := performRequest(router, consts.MethodHead, "/using_static/"+filename)
	w4 := performRequest(router, consts.MethodHead, "/result")

	assert.DeepEqual(t, w3, w4)
	assert.DeepEqual(t, consts.StatusOK, w3.Code)
}

// TestHandleStaticDir - ensure the root/sub dir handles properly
func TestRouteStaticListingDir(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.StaticFS("/", &app.FS{Root: "./", GenerateIndexPages: true})

	w := performRequest(router, consts.MethodGet, "/")
	assert.DeepEqual(t, consts.StatusOK, w.Code)

	assert.True(t, strings.Contains(w.Body.String(), "engine.go"))
	assert.DeepEqual(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

// TestHandleHeadToDir - ensure the root/sub dir handles properly
func TestRouteStaticNoListing(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.Static("/", "./")

	w := performRequest(router, consts.MethodGet, "/")

	assert.DeepEqual(t, http.StatusForbidden, w.Code)
	assert.False(t, strings.Contains(w.Body.String(), "engine.go"))
}

func TestRouterMiddlewareAndStatic(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	static := router.Group("/", func(c context.Context, ctx *app.RequestContext) {
		ctx.Response.Header.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 MST")
		ctx.Response.Header.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 MST")
		ctx.Response.Header.Set("Expires", "Mon, 02 Jan 2006 15:04:05 MST")
		ctx.Response.Header.Set("X-Hertz", "Hertz Framework")
	})
	static.Static("/", "./")

	w := performRequest(router, consts.MethodGet, "/engine.go")

	assert.DeepEqual(t, consts.StatusOK, w.Code)
	assert.True(t, strings.Contains(w.Body.String(), "package route"))
	// when Go version <= 1.16, mime.TypeByExtension will return Content-Type='text/plain; charset=utf-8',
	// otherwise it will return Content-Type='text/x-go; charset=utf-8'
	assert.NotEqual(t, "", w.Header().Get("Content-Type"))
	assert.NotEqual(t, w.Header().Get("Last-Modified"), "Mon, 02 Jan 2006 15:04:05 MST")
	assert.DeepEqual(t, "Mon, 02 Jan 2006 15:04:05 MST", w.Header().Get("Expires"))
	assert.DeepEqual(t, "Hertz Framework", w.Header().Get("x-Hertz"))
}

func TestRouteNotAllowedEnabled(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.options.HandleMethodNotAllowed = true
	router.POST("/path", func(c context.Context, ctx *app.RequestContext) {})
	w := performRequest(router, consts.MethodGet, "/path")
	assert.DeepEqual(t, consts.StatusMethodNotAllowed, w.Code)

	router.NoMethod(func(c context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusTeapot, "responseText")
	})
	w = performRequest(router, consts.MethodGet, "/path")
	assert.DeepEqual(t, "responseText", w.Body.String())
	assert.DeepEqual(t, http.StatusTeapot, w.Code)
}

func TestRouteNotAllowedEnabled2(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.options.HandleMethodNotAllowed = true
	// add one methodTree to trees
	router.addRoute(consts.MethodPost, "/", app.HandlersChain{func(_ context.Context, _ *app.RequestContext) {}})
	router.GET("/path2", func(c context.Context, ctx *app.RequestContext) {})
	w := performRequest(router, consts.MethodPost, "/path2")
	assert.DeepEqual(t, consts.StatusMethodNotAllowed, w.Code)
}

func TestRouteNotAllowedDisabled(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.options.HandleMethodNotAllowed = false
	router.POST("/path", func(c context.Context, ctx *app.RequestContext) {})
	w := performRequest(router, consts.MethodGet, "/path")
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)

	router.NoMethod(func(c context.Context, ctx *app.RequestContext) {
		ctx.String(http.StatusTeapot, "responseText")
	})
	w = performRequest(router, consts.MethodGet, "/path")
	assert.DeepEqual(t, "404 page not found", w.Body.String())
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
}

func TestRouterNotFoundWithRemoveExtraSlash(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.options.RemoveExtraSlash = true
	router.GET("/path", func(c context.Context, ctx *app.RequestContext) {})
	router.GET("/", func(c context.Context, ctx *app.RequestContext) {})

	testRoutes := []struct {
		route    string
		code     int
		location string
	}{
		{"/../path", consts.StatusOK, ""},    // CleanPath
		{"/nope", consts.StatusNotFound, ""}, // NotFound
	}
	for _, tr := range testRoutes {
		w := performRequest(router, "GET", tr.route)
		assert.DeepEqual(t, tr.code, w.Code)
		if w.Code != consts.StatusNotFound {
			assert.DeepEqual(t, tr.location, fmt.Sprint(w.Header().Get("Location")))
		}
	}
}

func TestRouterNotFound(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.options.RedirectFixedPath = true
	router.GET("/path", func(c context.Context, ctx *app.RequestContext) {})
	router.GET("/dir/", func(c context.Context, ctx *app.RequestContext) {})
	router.GET("/", func(c context.Context, ctx *app.RequestContext) {})

	testRoutes := []struct {
		route    string
		code     int
		location string
	}{
		{"/path/", consts.StatusMovedPermanently, "/path"},    // TSR -/
		{"/dir", consts.StatusMovedPermanently, "/dir/"},      // TSR +/
		{"/PATH", consts.StatusMovedPermanently, "/path"},     // Fixed Case
		{"/DIR/", consts.StatusMovedPermanently, "/dir/"},     // Fixed Case
		{"/PATH/", consts.StatusMovedPermanently, "/path"},    // Fixed Case -/
		{"/DIR", consts.StatusMovedPermanently, "/dir/"},      // Fixed Case +/
		{"/../path/", consts.StatusMovedPermanently, "/path"}, // Without CleanPath
		{"/nope", consts.StatusNotFound, ""},                  // NotFound
	}
	for _, tr := range testRoutes {
		w := performRequest(router, consts.MethodGet, tr.route)
		assert.DeepEqual(t, tr.code, w.Code)
		if w.Code != consts.StatusNotFound {
			assert.DeepEqual(t, tr.location, fmt.Sprint(w.Header().Get("Location")))
		}
	}

	// Test custom not found handler
	var notFound bool
	router.NoRoute(func(c context.Context, ctx *app.RequestContext) {
		ctx.AbortWithStatus(consts.StatusNotFound)
		notFound = true
	})
	w := performRequest(router, consts.MethodGet, "/nope")
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
	assert.True(t, notFound)

	// Test other method than GET (want 307 instead of 301)
	router.PATCH("/path", func(c context.Context, ctx *app.RequestContext) {})
	w = performRequest(router, consts.MethodPatch, "/path/")
	assert.DeepEqual(t, consts.StatusTemporaryRedirect, w.Code)
	assert.DeepEqual(t, "map[Content-Type:[text/plain; charset=utf-8] Location:[/path]]", fmt.Sprint(w.Header()))

	// Test special case where no node for the prefix "/" exists
	router = NewEngine(config.NewOptions(nil))
	router.GET("/a", func(c context.Context, ctx *app.RequestContext) {})
	w = performRequest(router, consts.MethodGet, "/")
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
}

func TestRouterStaticFSNotFound(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))
	router.StaticFS("/", &app.FS{Root: "/thisreallydoesntexist/"})
	router.NoRoute(func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusNotFound, "non existent")
	})

	w := performRequest(router, consts.MethodGet, "/nonexistent")
	assert.DeepEqual(t, "Cannot open requested path", w.Body.String())

	w = performRequest(router, consts.MethodHead, "/nonexistent")
	assert.DeepEqual(t, "Cannot open requested path", w.Body.String())
}

func TestRouterStaticFSFileNotFound(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))

	router.StaticFS("/", &app.FS{Root: "."})

	assert.NotPanic(t, func() {
		performRequest(router, consts.MethodGet, "/nonexistent")
	})
}

func TestMiddlewareCalledOnceByRouterStaticFSNotFound(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))

	// Middleware must be called just only once by per request.
	middlewareCalledNum := 0
	router.Use(func(c context.Context, ctx *app.RequestContext) {
		middlewareCalledNum++
	})

	router.StaticFS("/", &app.FS{Root: "/thisreallydoesntexist/"})

	// First access
	performRequest(router, consts.MethodGet, "/nonexistent")
	assert.DeepEqual(t, 1, middlewareCalledNum)

	// Second access
	performRequest(router, consts.MethodHead, "/nonexistent")
	assert.DeepEqual(t, 2, middlewareCalledNum)
}

func TestRouteRawPath(t *testing.T) {
	route := NewEngine(config.NewOptions(nil))
	route.options.UseRawPath = true

	route.POST("/project/:name/build/:num", func(c context.Context, ctx *app.RequestContext) {
		name := ctx.Params.ByName("name")
		num := ctx.Params.ByName("num")

		assert.DeepEqual(t, name, ctx.Param("name"))
		assert.DeepEqual(t, num, ctx.Param("num"))

		assert.DeepEqual(t, "Some/Other/Project", name)
		assert.DeepEqual(t, "222", num)
	})

	w := performRequest(route, consts.MethodPost, "/project/Some%2FOther%2FProject/build/222")
	assert.DeepEqual(t, consts.StatusOK, w.Code)
}

func TestRouteRawPathNoUnescape(t *testing.T) {
	route := NewEngine(config.NewOptions(nil))
	route.options.UseRawPath = true
	route.options.UnescapePathValues = false

	route.POST("/project/:name/build/:num", func(c context.Context, ctx *app.RequestContext) {
		name := ctx.Params.ByName("name")
		num := ctx.Params.ByName("num")

		assert.DeepEqual(t, name, ctx.Param("name"))
		assert.DeepEqual(t, num, ctx.Param("num"))

		assert.DeepEqual(t, "Some%2FOther%2FProject", name)
		assert.DeepEqual(t, "333", num)
	})

	w := performRequest(route, consts.MethodPost, "/project/Some%2FOther%2FProject/build/333")
	assert.DeepEqual(t, consts.StatusOK, w.Code)
}

func TestRouteServeErrorWithWriteHeader(t *testing.T) {
	route := NewEngine(config.NewOptions(nil))
	route.Use(func(c context.Context, ctx *app.RequestContext) {
		ctx.SetStatusCode(421)
		ctx.Next(c)
	})

	w := performRequest(route, consts.MethodGet, "/NotFound")
	assert.DeepEqual(t, 421, w.Code)
	assert.DeepEqual(t, 0, w.Body.Len())
}

func TestRouteContextHoldsFullPath(t *testing.T) {
	router := NewEngine(config.NewOptions(nil))

	// Test routes
	routes := []string{
		"/simple",
		"/project/:name",
		"/",
		"/news/home",
		"/news",
		"/simple-two/one",
		"/simple-two/one-two",
		"/project/:name/build/*params",
		"/project/:name/bui",
		"/user/:id/status",
		"/user/:id",
		"/user/:id/profile",
	}

	for _, route := range routes {
		actualRoute := route
		router.GET(route, func(c context.Context, ctx *app.RequestContext) {
			// For each defined route context should contain its full path
			assert.DeepEqual(t, actualRoute, ctx.FullPath())
			ctx.AbortWithStatus(consts.StatusOK)
		})
	}

	for _, route := range routes {
		w := performRequest(router, consts.MethodGet, route)
		assert.DeepEqual(t, consts.StatusOK, w.Code)
	}

	// Test not found
	router.Use(func(c context.Context, ctx *app.RequestContext) {
		// For not found routes full path is empty
		assert.DeepEqual(t, "", ctx.FullPath())
	})

	w := performRequest(router, consts.MethodGet, "/not-found")
	assert.DeepEqual(t, consts.StatusNotFound, w.Code)
}

func checkUnusedParamValues(t *testing.T, ctx *app.RequestContext, expectParam map[string]string) {
	for _, p := range ctx.Params {
		if expectParam == nil {
			t.Errorf("pValue '%+v' is set for param name '%v' but we are not expecting it", p.Value, p.Key)
		} else if val, ok := expectParam[p.Key]; !ok || val != p.Value {
			t.Errorf("'%+v' is set for param name '%v' but we are expecting it with expectParam '%+v'", p.Value, p.Key, val)
		}
	}
}

var handlerFunc = func(route string) app.HandlersChain {
	return app.HandlersChain{func(c context.Context, ctx *app.RequestContext) {
		ctx.Set("path", route)
	}}
}

var handlerHelper = func(route, key string, value int) app.HandlersChain {
	return app.HandlersChain{func(c context.Context, ctx *app.RequestContext) {
		ctx.Set(key, value)
		ctx.Set("path", route)
	}}
}

func getHelper(c *app.RequestContext, key string) interface{} {
	p, _ := c.Get(key)
	return p
}

func TestRouterStatic(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	path := "/folders/a/files/hertz.gif"
	e.addRoute(consts.MethodGet, path, handlerFunc(path))
	c := e.NewContext()
	c.Request.SetRequestURI(path)
	c.Request.Header.SetMethod(consts.MethodGet)
	e.ServeHTTP(context.Background(), c)
	assert.DeepEqual(t, path, getHelper(c, "path"))
}

func TestRouterParam(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.addRoute(consts.MethodGet, "/users/:id", handlerFunc("/users/:id"))

	testCases := []struct {
		name        string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
	}{
		{
			name:        "route /users/1 to /users/:id",
			whenURL:     "/users/1",
			expectRoute: "/users/:id",
			expectParam: map[string]string{"id": "1"},
		},
		{
			name:        "route /users/1/ to /users/:id",
			whenURL:     "/users/1/",
			expectRoute: nil,
			expectParam: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := e.NewContext()
			c.Request.SetRequestURI(tc.whenURL)
			c.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), c)
			assert.DeepEqual(t, tc.expectRoute, getHelper(c, "path"))
			checkUnusedParamValues(t, c, tc.expectParam)
		})
	}
}

func TestRouterTwoParam(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.addRoute(consts.MethodGet, "/users/:uid/files/:fid", handlerFunc("/users/:uid/files/:fid"))
	ctx := e.NewContext()
	ctx.Request.SetRequestURI("/users/1/files/1")
	ctx.Request.Header.SetMethod(consts.MethodGet)
	e.ServeHTTP(context.Background(), ctx)

	assert.DeepEqual(t, "1", ctx.Param("uid"))
	assert.DeepEqual(t, "1", ctx.Param("fid"))
}

func TestRouterParamWithSlash(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	e.addRoute(consts.MethodGet, "/a/:b/c/d/:e", handlerFunc("/a/:b/c/d/:e"))
	e.addRoute(consts.MethodGet, "/a/:b/c/:d/:f", handlerFunc("/a/:b/c/:d/:f"))

	ctx := e.NewContext()
	ctx.Request.SetRequestURI("/a/1/c/d/2/3")
	ctx.Request.Header.SetMethod(consts.MethodGet)
	e.ServeHTTP(context.Background(), ctx)
	assert.Nil(t, getHelper(ctx, "path"))
	assert.DeepEqual(t, consts.StatusNotFound, ctx.Response.StatusCode())
}

func TestRouteMultiLevelBacktracking(t *testing.T) {
	testCases := []struct {
		name        string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
	}{
		{
			name:        "route /a/c/df to /a/c/df",
			whenURL:     "/a/c/df",
			expectRoute: "/a/c/df",
		},
		{
			name:        "route /a/x/df to /a/:b/c",
			whenURL:     "/a/x/c",
			expectRoute: "/a/:b/c",
			expectParam: map[string]string{"b": "x"},
		},
		// {
		// 	name:        "route /a/x/f to /a/*/f",
		// 	whenURL:     "/a/x/f",
		// 	expectRoute: "/a/*/f",
		// 	expectParam: map[string]string{"x": "x/f"}, // NOTE: `x` would be probably more suitable
		// },
		{
			name:        "route /b/c/f to /:e/c/f",
			whenURL:     "/b/c/f",
			expectRoute: "/:e/c/f",
			expectParam: map[string]string{"e": "b"},
		},
		{
			name:        "route /b/c/c to /*x",
			whenURL:     "/b/c/c",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "b/c/c"},
		},
	}

	e := NewEngine(config.NewOptions(nil))

	e.addRoute(consts.MethodGet, "/a/:b/c", handlerHelper("/a/:b/c", "case", 1))
	e.addRoute(consts.MethodGet, "/a/c/d", handlerHelper("/a/c/d", "case", 2))
	e.addRoute(consts.MethodGet, "/a/c/df", handlerHelper("/a/c/df", "case", 3))
	// e.addRoute(consts.MethodGet, "/a/*/f", handlerHelper("case", 4))
	e.addRoute(consts.MethodGet, "/:e/c/f", handlerHelper("/:e/c/f", "case", 5))
	e.addRoute(consts.MethodGet, "/*x", handlerHelper("/*x", "case", 6))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouteMultiLevelBacktracking2(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.addRoute(consts.MethodGet, "/a/:b/c", handlerFunc("/a/:b/c"))
	e.addRoute(consts.MethodGet, "/a/c/d", handlerFunc("/a/c/d"))
	e.addRoute(consts.MethodGet, "/a/c/df", handlerFunc("/a/c/df"))
	e.addRoute(consts.MethodGet, "/:e/c/f", handlerFunc("/:e/c/f"))
	e.addRoute(consts.MethodGet, "/*x", handlerFunc("/*x"))

	testCases := []struct {
		name        string
		whenURL     string
		expectRoute string
		expectParam map[string]string
	}{
		{
			name:        "route /a/c/df to /a/c/df",
			whenURL:     "/a/c/df",
			expectRoute: "/a/c/df",
		},
		{
			name:        "route /a/x/df to /a/:b/c",
			whenURL:     "/a/x/c",
			expectRoute: "/a/:b/c",
			expectParam: map[string]string{"b": "x"},
		},
		{
			name:        "route /a/c/f to /:e/c/f",
			whenURL:     "/a/c/f",
			expectRoute: "/:e/c/f",
			expectParam: map[string]string{"e": "a"},
		},
		{
			name:        "route /b/c/f to /:e/c/f",
			whenURL:     "/b/c/f",
			expectRoute: "/:e/c/f",
			expectParam: map[string]string{"e": "b"},
		},
		{
			name:        "route /b/c/c to /*",
			whenURL:     "/b/c/c",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "b/c/c"},
		},
		{ // this traverses `/a/:b/c` and `/:e/c/f` branches and eventually backtracks to `/*`
			name:        "route /a/c/cf to /*",
			whenURL:     "/a/c/cf",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "a/c/cf"},
		},
		{
			name:        "route /anyMatch to /*",
			whenURL:     "/anyMatch",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "anyMatch"},
		},
		{
			name:        "route /anyMatch/withSlash to /*",
			whenURL:     "/anyMatch/withSlash",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "anyMatch/withSlash"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterBacktrackingFromMultipleParamKinds(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.addRoute(consts.MethodGet, "/*x", handlerFunc("/*x")) // this can match only path that does not have slash in it
	e.addRoute(consts.MethodGet, "/:1/second", handlerFunc("/:1/second"))
	e.addRoute(consts.MethodGet, "/:1/:2", handlerFunc("/:1/:2")) // this acts as match ANY for all routes that have at least one slash
	e.addRoute(consts.MethodGet, "/:1/:2/third", handlerFunc("/:1/:2/third"))
	e.addRoute(consts.MethodGet, "/:1/:2/:3/fourth", handlerFunc("/:1/:2/:3/fourth"))
	e.addRoute(consts.MethodGet, "/:1/:2/:3/:4/fifth", handlerFunc("/:1/:2/:3/:4/fifth"))

	testCases := []struct {
		name        string
		whenURL     string
		expectRoute string
		expectParam map[string]string
	}{
		{
			name:        "route /first to /*",
			whenURL:     "/first",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "first"},
		},
		{
			name:        "route /first/second to /:1/second",
			whenURL:     "/first/second",
			expectRoute: "/:1/second",
			expectParam: map[string]string{"1": "first"},
		},
		{
			name:        "route /first/second-new to /:1/:2",
			whenURL:     "/first/second-new",
			expectRoute: "/:1/:2",
			expectParam: map[string]string{
				"1": "first",
				"2": "second-new",
			},
		},
		{
			name:        "route /first/second/ to /:1/:2",
			whenURL:     "/first/second/",
			expectRoute: "/*x", // "/:1/:2",
			expectParam: map[string]string{"x": "first/second/"},
		},
		{
			name:        "route /first/second/third/fourth/fifth/nope to /:1/:2",
			whenURL:     "/first/second/third/fourth/fifth/nope",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "first/second/third/fourth/fifth/nope"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterParamStaticConflict(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	g := e.Group("/g")
	g.GET("/skills", handlerFunc("/g/skills")...)
	g.GET("/status", handlerFunc("/g/status")...)
	g.GET("/:name", handlerFunc("/g/:name")...)

	testCases := []struct {
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
	}{
		{
			whenURL:     "/g/s",
			expectRoute: "/g/:name",
			expectParam: map[string]string{"name": "s"},
		},
		{
			whenURL:     "/g/status",
			expectRoute: "/g/status",
			expectParam: map[string]string{"name": ""},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()

			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)
			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterMatchAny(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Routes
	e.addRoute(consts.MethodGet, "/", handlerFunc("/"))
	e.addRoute(consts.MethodGet, "/*x", handlerFunc("/*x"))
	e.addRoute(consts.MethodGet, "/users/*x", handlerFunc("/users/*x"))

	testCases := []struct {
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
	}{
		{
			whenURL:     "/",
			expectRoute: "/",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/download",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "download"},
		},
		{
			whenURL:     "/users/joe",
			expectRoute: "/users/*x",
			expectParam: map[string]string{"x": "joe"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()

			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

// NOTE: This is to document current implementation. Last added route with `*` asterisk is always the match and no
// backtracking or more precise matching is done to find more suitable match.
//
// Current behaviour might not be correct or expected.
// But this is where we are without well defined requirements/rules how (multiple) asterisks work in route
func TestRouterAnyMatchesLastAddedAnyRoute(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	e.addRoute(consts.MethodGet, "/users/*x", handlerHelper("/users/*x", "case", 1))
	// e.addRoute(consts.MethodGet, "/users/*x/action*y", handlerHelper("/users/*x/action*y", "case", 2))

	ctx := e.NewContext()

	ctx.Request.SetRequestURI("/users/xxx/action/sea")
	ctx.Request.Header.SetMethod(consts.MethodGet)
	e.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, "/users/*x", getHelper(ctx, "path"))
	assert.DeepEqual(t, "xxx/action/sea", ctx.Param("x"))

	// if we add another route then it is the last added and so it is matched
	// e.addRoute(consts.MethodGet, "/users/*x/action/search", handlerHelper("/users/*x/action/search", "case", 3))

	// c.Request.SetRequestURI("/users/xxx/action/sea")
	// c.Request.Header.SetMethod(consts.MethodGet)
	// e.ServeHTTP(context.Background(), c)
	// test.DeepEqual(t, "/users/*x/action/search", getHelper(c, "path"))
	// test.DeepEqual(t, "xxx/action/sea", ctx.Param("x"))
}

func TestRouterMatchAnyPrefixIssue(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Routes
	e.addRoute(consts.MethodGet, "/*x", handlerFunc("/*x"))
	e.addRoute(consts.MethodGet, "/users/*x", handlerFunc("/users/*x"))

	testCases := []struct {
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
	}{
		{
			whenURL:     "/",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/users",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "users"},
		},
		{
			whenURL:     "/users/",
			expectRoute: "/users/*x",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/users_prefix",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "users_prefix"},
		},
		{
			whenURL:     "/users_prefix/",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "users_prefix/"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()

			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

// TestRouterMatchAnySlash shall verify finding the best route
// for any routes with trailing slash requests
func TestRouterMatchAnySlash(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Routes
	e.addRoute(consts.MethodGet, "/users", handlerFunc("/users"))
	e.addRoute(consts.MethodGet, "/users/*x", handlerFunc("/users/*x"))
	e.addRoute(consts.MethodGet, "/img/*x", handlerFunc("/img/*x"))
	e.addRoute(consts.MethodGet, "/img/load", handlerFunc("/img/load"))
	e.addRoute(consts.MethodGet, "/img/load/*x", handlerFunc("/img/load/*x"))
	e.addRoute(consts.MethodGet, "/assets/*x", handlerFunc("/assets/*x"))

	testCases := []struct {
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{
			whenURL:     "/",
			expectRoute: nil,
			expectParam: map[string]string{"x": ""},
		},
		{ // Test trailing slash request for simple any route (see #1526)
			whenURL:     "/users/",
			expectRoute: "/users/*x",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/users/joe",
			expectRoute: "/users/*x",
			expectParam: map[string]string{"x": "joe"},
		},
		// Test trailing slash request for nested any route (see #1526)
		{
			whenURL:     "/img/load",
			expectRoute: "/img/load",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/img/load/",
			expectRoute: "/img/load/*x",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/img/load/ben",
			expectRoute: "/img/load/*x",
			expectParam: map[string]string{"x": "ben"},
		},
		// Test /assets/*x any route
		{ // ... without trailing slash must not match
			whenURL:     "/assets",
			expectRoute: nil,
			expectParam: map[string]string{"x": ""},
		},

		{ // ... with trailing slash must match
			whenURL:     "/assets/",
			expectRoute: "/assets/*x",
			expectParam: map[string]string{"x": ""},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()

			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterMatchAnyMultiLevel(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Routes
	e.addRoute(consts.MethodGet, "/api/users/jack", handlerFunc("/api/users/jack"))
	e.addRoute(consts.MethodGet, "/api/users/jill", handlerFunc("/api/users/jill"))
	e.addRoute(consts.MethodGet, "/api/users/*x", handlerFunc("/api/users/*x"))
	e.addRoute(consts.MethodGet, "/api/*x", handlerFunc("/api/*x"))
	e.addRoute(consts.MethodGet, "/other/*x", handlerFunc("/other/*x"))
	e.addRoute(consts.MethodGet, "/*x", handlerFunc("/*x"))

	testCases := []struct {
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{
			whenURL:     "/api/users/jack",
			expectRoute: "/api/users/jack",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/api/users/jill",
			expectRoute: "/api/users/jill",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/api/users/joe",
			expectRoute: "/api/users/*x",
			expectParam: map[string]string{"x": "joe"},
		},
		{
			whenURL:     "/api/nousers/joe",
			expectRoute: "/api/*x",
			expectParam: map[string]string{"x": "nousers/joe"},
		},
		{
			whenURL:     "/api/none",
			expectRoute: "/api/*x",
			expectParam: map[string]string{"x": "none"},
		},
		{
			whenURL:     "/api/none",
			expectRoute: "/api/*x",
			expectParam: map[string]string{"x": "none"},
		},
		{
			whenURL:     "/noapi/users/jim",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "noapi/users/jim"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()

			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterMatchAnyMultiLevelWithPost(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Routes
	e.POST("/api/auth/login", handlerFunc("/api/auth/login")...)
	e.POST("/api/auth/forgotPassword", handlerFunc("/api/auth/forgotPassword")...)
	e.Any("/api/*x", handlerFunc("/api/*x")...)
	e.Any("/*x", handlerFunc("/*x")...)

	testCases := []struct {
		whenMethod  string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{ // POST /api/auth/login shall choose login method
			whenURL:     "/api/auth/login",
			whenMethod:  consts.MethodPost,
			expectRoute: "/api/auth/login",
			expectParam: map[string]string{"x": ""},
		},
		{ // POST /api/auth/logout shall choose nearest any route
			whenURL:     "/api/auth/logout",
			whenMethod:  consts.MethodPost,
			expectRoute: "/api/*x",
			expectParam: map[string]string{"x": "auth/logout"},
		},
		{ // POST to /api/other/test shall choose nearest any route
			whenURL:     "/api/other/test",
			whenMethod:  consts.MethodPost,
			expectRoute: "/api/*x",
			expectParam: map[string]string{"x": "other/test"},
		},
		{ // GET to /api/other/test shall choose nearest any route
			whenURL:     "/api/other/test",
			whenMethod:  consts.MethodGet,
			expectRoute: "/api/*x",
			expectParam: map[string]string{"x": "other/test"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(tc.whenMethod)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterMicroParam(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.addRoute(consts.MethodGet, "/:a/:b/:c", handlerFunc("/:a/:b/:c"))
	ctx := e.NewContext()
	ctx.Request.SetRequestURI("/1/2/3")
	ctx.Request.Header.SetMethod(consts.MethodGet)
	e.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, "1", ctx.Param("a"))
	assert.DeepEqual(t, "2", ctx.Param("b"))
	assert.DeepEqual(t, "3", ctx.Param("c"))
}

func TestRouterMixParamMatchAny(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Route
	e.addRoute(consts.MethodGet, "/users/:id/*x", handlerFunc("/users/:id/*x"))
	ctx := e.NewContext()

	ctx.Request.SetRequestURI("/users/joe/comments")
	ctx.Request.Header.SetMethod(consts.MethodGet)
	e.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, "joe", ctx.Param("id"))
}

func TestRouterMultiRoute(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Routes
	e.addRoute(consts.MethodGet, "/users", handlerFunc("/users"))
	e.addRoute(consts.MethodGet, "/users/:id", handlerFunc("/users/:id"))

	testCases := []struct {
		whenMethod  string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{
			whenURL:     "/users",
			expectRoute: "/users",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/users/1",
			expectRoute: "/users/:id",
			expectParam: map[string]string{"id": "1"},
		},
		{
			whenURL:     "/user",
			expectRoute: nil,
			expectParam: map[string]string{"x": ""},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterPriority(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Routes
	e.addRoute(consts.MethodGet, "/users", handlerFunc("/users"))
	e.addRoute(consts.MethodGet, "/users/new", handlerFunc("/users/new"))
	e.addRoute(consts.MethodGet, "/users/:id", handlerFunc("/users/:id"))
	e.addRoute(consts.MethodGet, "/users/dew", handlerFunc("/users/dew"))
	e.addRoute(consts.MethodGet, "/users/:id/files", handlerFunc("/users/:id/files"))
	e.addRoute(consts.MethodGet, "/users/newsee", handlerFunc("/users/newsee"))
	e.addRoute(consts.MethodGet, "/users/*x", handlerFunc("/users/*x"))
	e.addRoute(consts.MethodGet, "/users/new/*x", handlerFunc("/users/new/*x"))
	e.addRoute(consts.MethodGet, "/*x", handlerFunc("/*x"))

	testCases := []struct {
		whenMethod  string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{
			whenURL:     "/users",
			expectRoute: "/users",
		},
		{
			whenURL:     "/users/new",
			expectRoute: "/users/new",
		},
		{
			whenURL:     "/users/1",
			expectRoute: "/users/:id",
			expectParam: map[string]string{"id": "1"},
		},
		{
			whenURL:     "/users/dew",
			expectRoute: "/users/dew",
		},
		{
			whenURL:     "/users/1/files",
			expectRoute: "/users/:id/files",
			expectParam: map[string]string{"id": "1"},
		},
		{
			whenURL:     "/users/new",
			expectRoute: "/users/new",
		},
		{
			whenURL:     "/users/news",
			expectRoute: "/users/:id",
			expectParam: map[string]string{"id": "news"},
		},
		{
			whenURL:     "/users/newsee",
			expectRoute: "/users/newsee",
		},
		{
			whenURL:     "/users/joe/books",
			expectRoute: "/users/*x",
			expectParam: map[string]string{"x": "joe/books"},
		},
		{
			whenURL:     "/users/new/someone",
			expectRoute: "/users/new/*x",
			expectParam: map[string]string{"x": "someone"},
		},
		{
			whenURL:     "/users/dew/someone",
			expectRoute: "/users/*x",
			expectParam: map[string]string{"x": "dew/someone"},
		},
		{ // Route > /users/*x should be matched although /users/dew exists
			whenURL:     "/users/notexists/someone",
			expectRoute: "/users/*x",
			expectParam: map[string]string{"x": "notexists/someone"},
		},
		{
			whenURL:     "/nousers",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "nousers"},
		},
		{
			whenURL:     "/nousers/new",
			expectRoute: "/*x",
			expectParam: map[string]string{"x": "nousers/new"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterIssue1348(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	e.addRoute(consts.MethodGet, "/:lang/", handlerFunc("/:lang/"))
	e.addRoute(consts.MethodGet, "/:lang/dupa", handlerFunc("/:lang/dupa"))
}

func TestRouterPriorityNotFound(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Add
	e.addRoute(consts.MethodGet, "/a/foo", handlerFunc("/a/foo"))
	e.addRoute(consts.MethodGet, "/a/bar", handlerFunc("/a/bar"))

	testCases := []struct {
		whenMethod  string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{
			whenURL:     "/a/foo",
			expectRoute: "/a/foo",
		},
		{
			whenURL:     "/a/bar",
			expectRoute: "/a/bar",
		},
		{
			whenURL:     "/abc/def",
			expectRoute: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterParamNames(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Routes
	e.addRoute(consts.MethodGet, "/users", handlerFunc("/users"))
	e.addRoute(consts.MethodGet, "/users/:id", handlerFunc("/users/:id"))
	e.addRoute(consts.MethodGet, "/users/:uid/files/:fid", handlerFunc("/users/:uid/files/:fid"))

	testCases := []struct {
		whenMethod  string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{
			whenURL:     "/users",
			expectRoute: "/users",
		},
		{
			whenURL:     "/users/1",
			expectRoute: "/users/:id",
			expectParam: map[string]string{"id": "1"},
		},
		{
			whenURL:     "/users/1/files/1",
			expectRoute: "/users/:uid/files/:fid",
			expectParam: map[string]string{
				"uid": "1",
				"fid": "1",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterStaticDynamicConflict(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	e.addRoute(consts.MethodGet, "/dictionary/skills", handlerHelper("/dictionary/skills", "a", 1))
	e.addRoute(consts.MethodGet, "/dictionary/:name", handlerHelper("/dictionary/:name", "b", 2))
	e.addRoute(consts.MethodGet, "/users/new", handlerHelper("/users/new", "d", 4))
	e.addRoute(consts.MethodGet, "/users/:name", handlerHelper("/users/:name", "e", 5))
	e.addRoute(consts.MethodGet, "/server", handlerHelper("/server", "c", 3))
	e.addRoute(consts.MethodGet, "/", handlerHelper("/", "f", 6))

	testCases := []struct {
		whenMethod  string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{
			whenURL:     "/dictionary/skills",
			expectRoute: "/dictionary/skills",
			expectParam: map[string]string{"x": ""},
		},
		{
			whenURL:     "/dictionary/skillsnot",
			expectRoute: "/dictionary/:name",
			expectParam: map[string]string{"name": "skillsnot"},
		},
		{
			whenURL:     "/dictionary/type",
			expectRoute: "/dictionary/:name",
			expectParam: map[string]string{"name": "type"},
		},
		{
			whenURL:     "/server",
			expectRoute: "/server",
		},
		{
			whenURL:     "/users/new",
			expectRoute: "/users/new",
		},
		{
			whenURL:     "/users/new2",
			expectRoute: "/users/:name",
			expectParam: map[string]string{"name": "new2"},
		},
		{
			whenURL:     "/",
			expectRoute: "/",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.whenURL, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}

func TestRouterParamBacktraceNotFound(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	// Add
	e.addRoute(consts.MethodGet, "/:param1", handlerFunc("/:param1"))
	e.addRoute(consts.MethodGet, "/:param1/foo", handlerFunc("/:param1/foo"))
	e.addRoute(consts.MethodGet, "/:param1/bar", handlerFunc("/:param1/bar"))
	e.addRoute(consts.MethodGet, "/:param1/bar/:param2", handlerFunc("/:param1/bar/:param2"))

	testCases := []struct {
		name        string
		whenMethod  string
		whenURL     string
		expectRoute interface{}
		expectParam map[string]string
		expectError error
	}{
		{
			name:        "route /a to /:param1",
			whenURL:     "/a",
			expectRoute: "/:param1",
			expectParam: map[string]string{"param1": "a"},
		},
		{
			name:        "route /a/foo to /:param1/foo",
			whenURL:     "/a/foo",
			expectRoute: "/:param1/foo",
			expectParam: map[string]string{"param1": "a"},
		},
		{
			name:        "route /a/bar to /:param1/bar",
			whenURL:     "/a/bar",
			expectRoute: "/:param1/bar",
			expectParam: map[string]string{"param1": "a"},
		},
		{
			name:        "route /a/bar/b to /:param1/bar/:param2",
			whenURL:     "/a/bar/b",
			expectRoute: "/:param1/bar/:param2",
			expectParam: map[string]string{
				"param1": "a",
				"param2": "b",
			},
		},
		{
			name:        "route /a/bbbbb should return 404",
			whenURL:     "/a/bbbbb",
			expectRoute: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := e.NewContext()
			ctx.Request.SetRequestURI(tc.whenURL)
			ctx.Request.Header.SetMethod(consts.MethodGet)
			e.ServeHTTP(context.Background(), ctx)

			assert.DeepEqual(t, tc.expectRoute, getHelper(ctx, "path"))
			for param, expectedValue := range tc.expectParam {
				assert.DeepEqual(t, expectedValue, ctx.Param(param))
			}
			checkUnusedParamValues(t, ctx, tc.expectParam)
		})
	}
}
