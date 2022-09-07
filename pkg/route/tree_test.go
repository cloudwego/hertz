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
 *
 * Copyright 2013 Julien Schmidt. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be found
 * at https://github.com/julienschmidt/httprouter/blob/master/LICENSE
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package route

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
)

// Used as a workaround since we can't compare functions or their addresses
var fakeHandlerValue string

func fakeHandler(val string) app.HandlersChain {
	return app.HandlersChain{func(c context.Context, ctx *app.RequestContext) {
		fakeHandlerValue = val
	}}
}

type testRequests []struct {
	path       string
	nilHandler bool
	route      string
	ps         param.Params
}

func getParams() *param.Params {
	ps := make(param.Params, 0, 20)
	return &ps
}

func checkRequests(t *testing.T, tree *router, requests testRequests, unescapes ...bool) {
	unescape := false
	if len(unescapes) >= 1 {
		unescape = unescapes[0]
	}

	for _, request := range requests {
		params := getParams()
		value := tree.find(request.path, params, unescape)

		if value.handlers == nil {
			if !request.nilHandler {
				t.Errorf("handle mismatch for route '%s': Expected non-nil handle", request.path)
			}
		} else if request.nilHandler {
			t.Errorf("handle mismatch for route '%s': Expected nil handle", request.path)
		} else {
			value.handlers[0](context.Background(), nil)
			if fakeHandlerValue != request.route {
				t.Errorf("handle mismatch for route '%s': Wrong handle (%s != %s)", request.path, fakeHandlerValue, request.route)
			}
		}
		for _, item := range request.ps {
			if item.Value != (*params).ByName(item.Key) {
				t.Errorf("mismatch params. path: %s, key: %s, expected value: %s, actual value: %s", request.path, item.Key, item.Value, (*params).ByName(item.Key))
			}
		}
	}
}

func TestCountParams(t *testing.T) {
	if countParams("/path/:param1/static/*catch-all") != 2 {
		t.Fail()
	}
	if countParams(strings.Repeat("/:param", 256)) != 256 {
		t.Fail()
	}
}

func TestEmptyPath(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"",
		"user",
		":user",
		"*user",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})
		if recv == nil {
			t.Fatalf("no panic while inserting route with empty path '%s", route)
		}
	}
}

func TestTreeAddAndGet(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/hi",
		"/contact",
		"/co",
		"/c",
		"/a",
		"/ab",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/α",
		"/β",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}

	checkRequests(t, tree, testRequests{
		{"", true, "", nil},
		{"a", true, "", nil},
		{"/a", false, "/a", nil},
		{"/", true, "", nil},
		{"/hi", false, "/hi", nil},
		{"/contact", false, "/contact", nil},
		{"/co", false, "/co", nil},
		{"/con", true, "", nil},  // key mismatch
		{"/cona", true, "", nil}, // key mismatch
		{"/no", true, "", nil},   // no matching child
		{"/ab", false, "/ab", nil},
		{"/α", false, "/α", nil},
		{"/β", false, "/β", nil},
	})
}

func TestTreeWildcard(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/",
		"/cmd/:tool/:sub",
		"/cmd/:tool/",
		"/cmd/xxx/",
		"/src/*filepath",
		"/search/",
		"/search/:query",
		"/user_:name",
		"/user_:name/about",
		"/files/:dir/*filepath",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/info/:user/public",
		"/info/:user/project/:project",
		"/a/b/:c",
		"/a/:b/c/d",
		"/a/*b",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/cmd/test/", false, "/cmd/:tool/", param.Params{param.Param{Key: "tool", Value: "test"}}},
		{"/cmd/test", true, "", nil},
		{"/cmd/test/3", false, "/cmd/:tool/:sub", param.Params{param.Param{Key: "tool", Value: "test"}, param.Param{Key: "sub", Value: "3"}}},
		{"/src/", false, "/src/*filepath", param.Params{param.Param{Key: "filepath", Value: ""}}},
		{"/src/some/file.png", false, "/src/*filepath", param.Params{param.Param{Key: "filepath", Value: "some/file.png"}}},
		{"/search/", false, "/search/", nil},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", param.Params{param.Param{Key: "query", Value: "someth!ng+in+ünìcodé"}}},
		{"/search/someth!ng+in+ünìcodé/", true, "", nil},
		{"/user_gopher", false, "/user_:name", param.Params{param.Param{Key: "name", Value: "gopher"}}},
		{"/user_gopher/about", false, "/user_:name/about", param.Params{param.Param{Key: "name", Value: "gopher"}}},
		{"/files/js/inc/framework.js", false, "/files/:dir/*filepath", param.Params{param.Param{Key: "dir", Value: "js"}, param.Param{Key: "filepath", Value: "inc/framework.js"}}},
		{"/info/gordon/public", false, "/info/:user/public", param.Params{param.Param{Key: "user", Value: "gordon"}}},
		{"/info/gordon/project/go", false, "/info/:user/project/:project", param.Params{param.Param{Key: "user", Value: "gordon"}, param.Param{Key: "project", Value: "go"}}},
		{"/a/b/c", false, "/a/b/:c", param.Params{param.Param{Key: "c", Value: "c"}}},
		{"/a/b/c/d", false, "/a/:b/c/d", param.Params{param.Param{Key: "b", Value: "b"}}},
		{"/a/b", false, "/a/*b", param.Params{param.Param{Key: "b", Value: "b"}}},
	})
}

func TestUnescapeParameters(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/",
		"/cmd/:tool/:sub",
		"/cmd/:tool/",
		"/src/*filepath",
		"/search/:query",
		"/files/:dir/*filepath",
		"/info/:user/project/:project",
		"/info/:user",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}

	unescape := true
	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/cmd/test/", false, "/cmd/:tool/", param.Params{param.Param{Key: "tool", Value: "test"}}},
		{"/cmd/test", true, "", nil},
		{"/src/some/file.png", false, "/src/*filepath", param.Params{param.Param{Key: "filepath", Value: "some/file.png"}}},
		{"/src/some/file+test.png", false, "/src/*filepath", param.Params{param.Param{Key: "filepath", Value: "some/file test.png"}}},
		{"/src/some/file++++%%%%test.png", false, "/src/*filepath", param.Params{param.Param{Key: "filepath", Value: "some/file++++%%%%test.png"}}},
		{"/src/some/file%2Ftest.png", false, "/src/*filepath", param.Params{param.Param{Key: "filepath", Value: "some/file/test.png"}}},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", param.Params{param.Param{Key: "query", Value: "someth!ng in ünìcodé"}}},
		{"/info/gordon/project/go", false, "/info/:user/project/:project", param.Params{param.Param{Key: "user", Value: "gordon"}, param.Param{Key: "project", Value: "go"}}},
		{"/info/slash%2Fgordon", false, "/info/:user", param.Params{param.Param{Key: "user", Value: "slash/gordon"}}},
		{"/info/slash%2Fgordon/project/Project%20%231", false, "/info/:user/project/:project", param.Params{param.Param{Key: "user", Value: "slash/gordon"}, param.Param{Key: "project", Value: "Project #1"}}},
		{"/info/slash%%%%", false, "/info/:user", param.Params{param.Param{Key: "user", Value: "slash%%%%"}}},
		{"/info/slash%%%%2Fgordon/project/Project%%%%20%231", false, "/info/:user/project/:project", param.Params{param.Param{Key: "user", Value: "slash%%%%2Fgordon"}, param.Param{Key: "project", Value: "Project%%%%20%231"}}},
	}, unescape)
}

func catchPanic(testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
	}()

	testFunc()
	return
}

type testRoute struct {
	path     string
	conflict bool
}

func testRoutes(t *testing.T, routes []testRoute) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route.path, []app.HandlerFunc{
				func(c context.Context, ctx *app.RequestContext) {
					fmt.Println("test")
				},
			})
		})

		if route.conflict {
			if recv == nil {
				t.Errorf("no panic for conflicting route '%s'", route.path)
			}
		} else if recv != nil {
			t.Errorf("unexpected panic for route '%s': %v", route.path, recv)
		}
	}
}

func TestTreeWildcardConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/vet", false},
		{"/cmd/:tool/:sub", false},
		{"/src/*filepath", false},
		{"/src/*filepathx", true},
		{"/src/", false},
		{"/src1/", false},
		{"/src1/*filepath", false},
		{"/src2*filepath", true},
		{"/search/:query", false},
		{"/search/invalid", false},
		{"/user_:name", false},
		{"/user_x", false},
		{"/user_:name", true},
		{"/id:id", false},
		{"/id/:id", false},
	}
	testRoutes(t, routes)
}

func TestTreeChildConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/vet", false},
		{"/cmd/:tool/:sub", false},
		{"/src/AUTHORS", false},
		{"/src/*filepath", false},
		{"/user_x", false},
		{"/user_:name", false},
		{"/id/:id", false},
		{"/id:id", false},
		{"/:id", false},
		{"/*filepath", false},
	}
	testRoutes(t, routes)
}

func TestTreeDuplicatePath(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/",
		"/doc/",
		"/src/*filepath",
		"/search/:query",
		"/user_:name",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}

		// Add again
		recv = catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		if recv == nil {
			t.Fatalf("no panic while inserting duplicate route '%s", route)
		}
	}

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/doc/", false, "/doc/", nil},
		{"/src/some/file.png", false, "/src/*filepath", param.Params{param.Param{Key: "filepath", Value: "some/file.png"}}},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", param.Params{param.Param{Key: "query", Value: "someth!ng+in+ünìcodé"}}},
		{"/user_gopher", false, "/user_:name", param.Params{param.Param{Key: "name", Value: "gopher"}}},
	})
}

func TestEmptyWildcardName(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/user:",
		"/user:/",
		"/cmd/:/",
		"/src/*",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})
		if recv == nil {
			t.Fatalf("no panic while inserting route with empty wildcard name '%s", route)
		}
	}
}

func TestTreeCatchAllConflict(t *testing.T) {
	routes := []testRoute{
		{"/src/*filepath/x", true},
		{"/src2/", false},
		{"/src2/*filepath/x", true},
		{"/src3/*filepath", false},
		{"/src3/*filepath/x", true},
	}
	testRoutes(t, routes)
}

func TestTreeCatchMaxParams(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}
	route := "/cmd/*filepath"
	tree.addRoute(route, fakeHandler(route))
}

func TestTreeDoubleWildcard(t *testing.T) {
	const panicMsg = "only one wildcard per path segment is allowed"

	routes := [...]string{
		"/:foo:bar",
		"/:foo:bar/",
		"/:foo*bar",
	}

	for _, route := range routes {
		tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})

		if rs, ok := recv.(string); !ok || !strings.HasPrefix(rs, panicMsg) {
			t.Fatalf(`"Expected panic "%s" for route '%s', got "%v"`, panicMsg, route, recv)
		}
	}
}

func TestTreeTrailingSlashRedirect2(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/api/:version/seller/locales/get",
		"/api/v:version/seller/permissions/get",
		"/api/v:version/seller/university/entrance_knowledge_list/get",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}
	v := make(param.Params, 0, 1)

	tsrRoutes := [...]string{
		"/api/v:version/seller/permissions/get/",
		"/api/version/seller/permissions/get/",
	}

	for _, route := range tsrRoutes {
		value := tree.find(route, &v, false)
		if value.handlers != nil {
			t.Fatalf("non-nil handler for TSR route '%s", route)
		} else if !value.tsr {
			t.Errorf("expected TSR recommendation for route '%s'", route)
		}
	}

	noTsrRoutes := [...]string{
		"/api/v:version/seller/permissions/get/a",
	}
	for _, route := range noTsrRoutes {
		value := tree.find(route, &v, false)
		if value.handlers != nil {
			t.Fatalf("non-nil handler for No-TSR route '%s", route)
		} else if value.tsr {
			t.Errorf("expected no TSR recommendation for route '%s'", route)
		}
	}
}

func TestTreeTrailingSlashRedirect(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/hi",
		"/b/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/admin",
		"/admin/:category",
		"/admin/:category/:page",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/no/a",
		"/no/b",
		"/api/hello/:name",
		"/user/:name/*id",
		"/resource",
		"/r/*id",
		"/book/biz/:name",
		"/book/biz/abc",
		"/book/biz/abc/bar",
		"/book/:page/:name",
		"/book/hello/:name/biz/",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	tsrRoutes := [...]string{
		"/hi/",
		"/b",
		"/search/gopher/",
		"/cmd/vet",
		"/src",
		"/x/",
		"/y",
		"/0/go/",
		"/1/go",
		"/a",
		"/admin/",
		"/admin/config/",
		"/admin/config/permissions/",
		"/doc/",
		"/user/name",
		"/r",
		"/book/hello/a/biz",
		"/book/biz/foo/",
		"/book/biz/abc/bar/",
	}
	v := make(param.Params, 0, 10)
	for _, route := range tsrRoutes {
		value := tree.find(route, &v, false)
		if value.handlers != nil {
			t.Fatalf("non-nil handler for TSR route '%s", route)
		} else if !value.tsr {
			t.Errorf("expected TSR recommendation for route '%s'", route)
		}
	}

	noTsrRoutes := [...]string{
		"/",
		"/no",
		"/no/",
		"/_",
		"/_/",
		"/api/world/abc",
		"/book",
		"/book/",
		"/book/hello/a/abc",
		"/book/biz/abc/biz",
	}
	for _, route := range noTsrRoutes {
		value := tree.find(route, &v, false)
		if value.handlers != nil {
			t.Fatalf("non-nil handler for No-TSR route '%s", route)
		} else if value.tsr {
			t.Errorf("expected no TSR recommendation for route '%s'", route)
		}
	}
}

func TestTreeRootTrailingSlashRedirect(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	recv := catchPanic(func() {
		tree.addRoute("/:test", fakeHandler("/:test"))
	})
	if recv != nil {
		t.Fatalf("panic inserting test route: %v", recv)
	}

	value := tree.find("/", nil, false)
	if value.handlers != nil {
		t.Fatalf("non-nil handler")
	} else if value.tsr {
		t.Errorf("expected no TSR recommendation")
	}
}

func TestTreeFindCaseInsensitivePath(t *testing.T) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	longPath := "/l" + strings.Repeat("o", 128) + "ng"
	lOngPath := "/l" + strings.Repeat("O", 128) + "ng/"

	routes := [...]string{
		"/hi",
		"/b/",
		"/ABC/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/doc/go/away",
		"/no/a",
		"/no/b",
		"/z/:id/2",
		"/z/:id/:age",
		"/x/:id/3/",
		"/x/:id/3/4",
		"/x/:id/:age/5",
		longPath,
	}

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	// Check out == in for all registered routes
	// With fixTrailingSlash = true
	for _, route := range routes {
		out, found := tree.root.findCaseInsensitivePath(route, true)
		if !found {
			t.Errorf("Route '%s' not found!", route)
		} else if string(out) != route {
			t.Errorf("Wrong result for route '%s': %s", route, string(out))
		}
	}
	// With fixTrailingSlash = false
	for _, route := range routes {
		out, found := tree.root.findCaseInsensitivePath(route, false)
		if !found {
			t.Errorf("Route '%s' not found!", route)
		} else if string(out) != route {
			t.Errorf("Wrong result for route '%s': %s", route, string(out))
		}
	}

	tests := []struct {
		in    string
		out   string
		found bool
		slash bool
	}{
		{"/HI", "/hi", true, false},
		{"/HI/", "/hi", true, true},
		{"/B", "/b/", true, true},
		{"/B/", "/b/", true, false},
		{"/abc", "/ABC/", true, true},
		{"/abc/", "/ABC/", true, false},
		{"/aBc", "/ABC/", true, true},
		{"/aBc/", "/ABC/", true, false},
		{"/abC", "/ABC/", true, true},
		{"/abC/", "/ABC/", true, false},
		{"/SEARCH/QUERY", "/search/QUERY", true, false},
		{"/SEARCH/QUERY/", "/search/QUERY", true, true},
		{"/CMD/TOOL/", "/cmd/TOOL/", true, false},
		{"/CMD/TOOL", "/cmd/TOOL/", true, true},
		{"/SRC/FILE/PATH", "/src/FILE/PATH", true, false},
		{"/x/Y", "/x/y", true, false},
		{"/x/Y/", "/x/y", true, true},
		{"/X/y", "/x/y", true, false},
		{"/X/y/", "/x/y", true, true},
		{"/X/Y", "/x/y", true, false},
		{"/X/Y/", "/x/y", true, true},
		{"/Y/", "/y/", true, false},
		{"/Y", "/y/", true, true},
		{"/Y/z", "/y/z", true, false},
		{"/Y/z/", "/y/z", true, true},
		{"/Y/Z", "/y/z", true, false},
		{"/Y/Z/", "/y/z", true, true},
		{"/y/Z", "/y/z", true, false},
		{"/y/Z/", "/y/z", true, true},
		{"/Aa", "/aa", true, false},
		{"/Aa/", "/aa", true, true},
		{"/AA", "/aa", true, false},
		{"/AA/", "/aa", true, true},
		{"/aA", "/aa", true, false},
		{"/aA/", "/aa", true, true},
		{"/A/", "/a/", true, false},
		{"/A", "/a/", true, true},
		{"/DOC", "/doc", true, false},
		{"/DOC/", "/doc", true, true},
		{"/NO", "", false, true},
		{"/DOC/GO", "", false, true},
		{"/Z/1/2", "/z/1/2", true, false},
		{"/Z/1/3", "/z/1/3", true, false},
		{"/Z/1/2/", "/z/1/2", true, true},
		{"/Z/1/3/", "/z/1/3", true, true},
		{"/X/1/3", "/x/1/3/", true, true},
		{"/X/1/3/5", "/x/1/3/5", true, false},
		{lOngPath, longPath, true, true},
	}
	// With fixTrailingSlash = true
	for _, test := range tests {
		out, found := tree.root.findCaseInsensitivePath(test.in, true)
		if found != test.found || (found && (string(out) != test.out)) {
			t.Errorf("Wrong result for '%s': got %s, %t; want %s, %t",
				test.in, string(out), found, test.out, test.found)
			return
		}
	}
	// With fixTrailingSlash = false
	for _, test := range tests {
		out, found := tree.root.findCaseInsensitivePath(test.in, false)
		if test.slash {
			if found { // test needs a trailingSlash fix. It must not be found!
				t.Errorf("Found without fixTrailingSlash: %s; got %s", test.in, string(out))
			}
		} else {
			if found != test.found || (found && (string(out) != test.out)) {
				t.Errorf("Wrong result for '%s': got %s, %t; want %s, %t",
					test.in, string(out), found, test.out, test.found)
				return
			}
		}
	}
}
