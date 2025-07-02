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
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
)

type Route struct {
	path string
}

func BenchmarkTree_FindStatic(b *testing.B) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	static := []*Route{
		{"/"},
		{"/cmd.html"},
		{"/code.html"},
		{"/contrib.html"},
		{"/contribute.html"},
		{"/debugging_with_gdb.html"},
		{"/docs.html"},
		{"/effective_go.html"},
		{"/files.log"},
		{"/gccgo_contribute.html"},
		{"/gccgo_install.html"},
		{"/go-logo-black.png"},
		{"/go-logo-blue.png"},
		{"/go-logo-white.png"},
		{"/go1.1.html"},
		{"/go1.2.html"},
		{"/go1.html"},
		{"/go1compat.html"},
		{"/go_faq.html"},
		{"/go_mem.html"},
		{"/go_spec.html"},
		{"/help.html"},
		{"/ie.css"},
		{"/install-source.html"},
		{"/install.html"},
		{"/logo-153x55.png"},
		{"/Makefile"},
		{"/root.html"},
		{"/share.png"},
		{"/sieve.gif"},
		{"/tos.html"},
		{"/articles/"},
		{"/articles/go_command.html"},
		{"/articles/index.html"},
		{"/articles/wiki/"},
		{"/articles/wiki/edit.html"},
		{"/articles/wiki/final-noclosure.go"},
		{"/articles/wiki/final-noerror.go"},
		{"/articles/wiki/final-parsetemplate.go"},
		{"/articles/wiki/final-template.go"},
		{"/articles/wiki/final.go"},
		{"/articles/wiki/get.go"},
		{"/articles/wiki/http-sample.go"},
		{"/articles/wiki/index.html"},
		{"/articles/wiki/Makefile"},
		{"/articles/wiki/notemplate.go"},
		{"/articles/wiki/part1-noerror.go"},
		{"/articles/wiki/part1.go"},
		{"/articles/wiki/part2.go"},
		{"/articles/wiki/part3-errorhandling.go"},
		{"/articles/wiki/part3.go"},
		{"/articles/wiki/test.bash"},
		{"/articles/wiki/test_edit.good"},
		{"/articles/wiki/test_Test.txt.good"},
		{"/articles/wiki/test_view.good"},
		{"/articles/wiki/view.html"},
		{"/codewalk/"},
		{"/codewalk/codewalk.css"},
		{"/codewalk/codewalk.js"},
		{"/codewalk/codewalk.xml"},
		{"/codewalk/functions.xml"},
		{"/codewalk/markov.go"},
		{"/codewalk/markov.xml"},
		{"/codewalk/pig.go"},
		{"/codewalk/popout.png"},
		{"/codewalk/run"},
		{"/codewalk/sharemem.xml"},
		{"/codewalk/urlpoll.go"},
		{"/devel/"},
		{"/devel/release.html"},
		{"/devel/weekly.html"},
		{"/gopher/"},
		{"/gopher/appenginegopher.jpg"},
		{"/gopher/appenginegophercolor.jpg"},
		{"/gopher/appenginelogo.gif"},
		{"/gopher/bumper.png"},
		{"/gopher/bumper192x108.png"},
		{"/gopher/bumper320x180.png"},
		{"/gopher/bumper480x270.png"},
		{"/gopher/bumper640x360.png"},
		{"/gopher/doc.png"},
		{"/gopher/frontpage.png"},
		{"/gopher/gopherbw.png"},
		{"/gopher/gophercolor.png"},
		{"/gopher/gophercolor16x16.png"},
		{"/gopher/help.png"},
		{"/gopher/pkg.png"},
		{"/gopher/project.png"},
		{"/gopher/ref.png"},
		{"/gopher/run.png"},
		{"/gopher/talks.png"},
		{"/gopher/pencil/"},
		{"/gopher/pencil/gopherhat.jpg"},
		{"/gopher/pencil/gopherhelmet.jpg"},
		{"/gopher/pencil/gophermega.jpg"},
		{"/gopher/pencil/gopherrunning.jpg"},
		{"/gopher/pencil/gopherswim.jpg"},
		{"/gopher/pencil/gopherswrench.jpg"},
		{"/play/"},
		{"/play/fib.go"},
		{"/play/hello.go"},
		{"/play/life.go"},
		{"/play/peano.go"},
		{"/play/pi.go"},
		{"/play/sieve.go"},
		{"/play/solitaire.go"},
		{"/play/tree.go"},
		{"/progs/"},
		{"/progs/cgo1.go"},
		{"/progs/cgo2.go"},
		{"/progs/cgo3.go"},
		{"/progs/cgo4.go"},
		{"/progs/defer.go"},
		{"/progs/defer.out"},
		{"/progs/defer2.go"},
		{"/progs/defer2.out"},
		{"/progs/eff_bytesize.go"},
		{"/progs/eff_bytesize.out"},
		{"/progs/eff_qr.go"},
		{"/progs/eff_sequence.go"},
		{"/progs/eff_sequence.out"},
		{"/progs/eff_unused1.go"},
		{"/progs/eff_unused2.go"},
		{"/progs/error.go"},
		{"/progs/error2.go"},
		{"/progs/error3.go"},
		{"/progs/error4.go"},
		{"/progs/go1.go"},
		{"/progs/gobs1.go"},
		{"/progs/gobs2.go"},
		{"/progs/image_draw.go"},
		{"/progs/image_package1.go"},
		{"/progs/image_package1.out"},
		{"/progs/image_package2.go"},
		{"/progs/image_package2.out"},
		{"/progs/image_package3.go"},
		{"/progs/image_package3.out"},
		{"/progs/image_package4.go"},
		{"/progs/image_package4.out"},
		{"/progs/image_package5.go"},
		{"/progs/image_package5.out"},
		{"/progs/image_package6.go"},
		{"/progs/image_package6.out"},
		{"/progs/interface.go"},
		{"/progs/interface2.go"},
		{"/progs/interface2.out"},
		{"/progs/json1.go"},
		{"/progs/json2.go"},
		{"/progs/json2.out"},
		{"/progs/json3.go"},
		{"/progs/json4.go"},
		{"/progs/json5.go"},
		{"/progs/run"},
		{"/progs/slices.go"},
		{"/progs/timeout1.go"},
		{"/progs/timeout2.go"},
		{"/progs/update.bash"},
	}

	for _, route := range static {
		tree.addRoute(route.path, fakeHandler(route.path))
	}
	ps := getParams()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, request := range static {
			tree.find(request.path, ps, false)
		}
	}
}

func BenchmarkTree_FindGithub(b *testing.B) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	static := []*Route{
		// OAuth Authorizations
		{"/authorizations"},
		{"/authorizations/:id"},
		//{"/authorizations"},
		//{"/authorizations/clients/:client_id"},
		//{"/authorizations/:id"},
		//{"/authorizations/:id"},
		{"/applications/:client_id/tokens/:access_token"},
		{"/applications/:client_id/tokens"},
		//{"/applications/:client_id/tokens/:access_token"},

		// Activity
		{"/events"},
		{"/repos/:owner/:repo/events"},
		{"/networks/:owner/:repo/events"},
		{"/orgs/:org/events"},
		{"/users/:user/received_events"},
		{"/users/:user/received_events/public"},
		{"/users/:user/events"},
		{"/users/:user/events/public"},
		{"/users/:user/events/orgs/:org"},
		{"/feeds"},
		//{"/notifications"},
		{"/repos/:owner/:repo/notifications"},
		{"/notifications"},
		//{"/repos/:owner/:repo/notifications"},
		{"/notifications/threads/:id"},
		//{"/notifications/threads/:id"},
		{"/notifications/threads/:id/subscription"},
		//{"/notifications/threads/:id/subscription"},
		//{"/notifications/threads/:id/subscription"},
		{"/repos/:owner/:repo/stargazers"},
		{"/users/:user/starred"},
		{"/user/starred"},
		{"/user/starred/:owner/:repo"},
		//{"/user/starred/:owner/:repo"},
		//{"/user/starred/:owner/:repo"},
		{"/repos/:owner/:repo/subscribers"},
		{"/users/:user/subscriptions"},
		{"/user/subscriptions"},
		{"/repos/:owner/:repo/subscription"},
		//{"/repos/:owner/:repo/subscription"},
		//{"/repos/:owner/:repo/subscription"},
		{"/user/subscriptions/:owner/:repo"},
		//{"PUT", "/user/subscriptions/:owner/:repo"},
		//{"DELETE", "/user/subscriptions/:owner/:repo"},

		// Gists
		{"/users/:user/gists"},
		{"/gists"},
		//{"GET", "/gists/public"},
		//{"GET", "/gists/starred"},
		{"/gists/:id"},
		//{"POST", "/gists"},
		//{"PATCH", "/gists/:id"},
		{"/gists/:id/star"},
		//{"DELETE", "/gists/:id/star"},
		//{"GET", "/gists/:id/star"},
		{"/gists/:id/forks"},
		//{"DELETE", "/gists/:id"},

		// Git Data
		{"/repos/:owner/:repo/git/blobs/:sha"},
		{"/repos/:owner/:repo/git/blobs"},
		{"/repos/:owner/:repo/git/commits/:sha"},
		{"/repos/:owner/:repo/git/commits"},
		//{"GET", "/repos/:owner/:repo/git/refs/*ref"},
		{"/repos/:owner/:repo/git/refs"},
		//{"POST", "/repos/:owner/:repo/git/refs"},
		//{"PATCH", "/repos/:owner/:repo/git/refs/*ref"},
		//{"DELETE", "/repos/:owner/:repo/git/refs/*ref"},
		{"/repos/:owner/:repo/git/tags/:sha"},
		{"/repos/:owner/:repo/git/tags"},
		{"/repos/:owner/:repo/git/trees/:sha"},
		{"/repos/:owner/:repo/git/trees"},

		{"/issues"},
		{"/user/issues"},
		{"/orgs/:org/issues"},
		{"/repos/:owner/:repo/issues"},
		{"/repos/:owner/:repo/issues/:number"},
		//{"POST", "/repos/:owner/:repo/issues"},
		//{"PATCH", "/repos/:owner/:repo/issues/:number"},
		{"/repos/:owner/:repo/assignees"},
		{"/repos/:owner/:repo/assignees/:assignee"},
		{"/repos/:owner/:repo/issues/:number/comments"},
		//{"GET", "/repos/:owner/:repo/issues/comments"},
		//{"GET", "/repos/:owner/:repo/issues/comments/:id"},
		//{"POST", "/repos/:owner/:repo/issues/:number/comments"},
		//{"PATCH", "/repos/:owner/:repo/issues/comments/:id"},
		//{"DELETE", "/repos/:owner/:repo/issues/comments/:id"},
		{"/repos/:owner/:repo/issues/:number/events"},
		//{"GET", "/repos/:owner/:repo/issues/events"},
		//{"GET", "/repos/:owner/:repo/issues/events/:id"},
		{"/repos/:owner/:repo/labels"},
		{"/repos/:owner/:repo/labels/:name"},
		//{"POST", "/repos/:owner/:repo/labels"},
		//{"PATCH", "/repos/:owner/:repo/labels/:name"},
		//{"DELETE", "/repos/:owner/:repo/labels/:name"},
		{"/repos/:owner/:repo/issues/:number/labels"},
		//{"POST", "/repos/:owner/:repo/issues/:number/labels"},
		//{"DELETE", "/repos/:owner/:repo/issues/:number/labels/:name"},
		//{"PUT", "/repos/:owner/:repo/issues/:number/labels"},
		//{"DELETE", "/repos/:owner/:repo/issues/:number/labels"},
		{"/repos/:owner/:repo/milestones/:number/labels"},
		{"/repos/:owner/:repo/milestones"},
		{"/repos/:owner/:repo/milestones/:number"},
		//{"POST", "/repos/:owner/:repo/milestones"},
		//{"PATCH", "/repos/:owner/:repo/milestones/:number"},
		//{"DELETE", "/repos/:owner/:repo/milestones/:number"},

		// Miscellaneous
		{"/emojis"},
		{"/gitignore/templates"},
		{"/gitignore/templates/:name"},
		{"/markdown"},
		{"/markdown/raw"},
		{"/meta"},
		{"/rate_limit"},

		// Organizations
		{"/users/:user/orgs"},
		{"/user/orgs"},
		{"/orgs/:org"},
		//{"PATCH", "/orgs/:org"},
		{"/orgs/:org/members"},
		{"/orgs/:org/members/:user"},
		//{"DELETE", "/orgs/:org/members/:user"},
		{"/orgs/:org/public_members"},
		{"/orgs/:org/public_members/:user"},
		//{"PUT", "/orgs/:org/public_members/:user"},
		//{"DELETE", "/orgs/:org/public_members/:user"},
		{"/orgs/:org/teams"},
		{"/teams/:id"},
		//{"POST", "/orgs/:org/teams"},
		//{"PATCH", "/teams/:id"},
		//{"DELETE", "/teams/:id"},
		{"/teams/:id/members"},
		{"/teams/:id/members/:user"},
		//{"PUT", "/teams/:id/members/:user"},
		//{"DELETE", "/teams/:id/members/:user"},
		{"/teams/:id/repos"},
		{"/teams/:id/repos/:owner/:repo"},
		//{"PUT", "/teams/:id/repos/:owner/:repo"},
		//{"DELETE", "/teams/:id/repos/:owner/:repo"},
		{"/user/teams"},

		// Pull Requests
		{"/repos/:owner/:repo/pulls"},
		{"/repos/:owner/:repo/pulls/:number"},
		//{"POST", "/repos/:owner/:repo/pulls"},
		//{"PATCH", "/repos/:owner/:repo/pulls/:number"},
		{"/repos/:owner/:repo/pulls/:number/commits"},
		{"/repos/:owner/:repo/pulls/:number/files"},
		{"/repos/:owner/:repo/pulls/:number/merge"},
		//{"PUT", "/repos/:owner/:repo/pulls/:number/merge"},
		{"/repos/:owner/:repo/pulls/:number/comments"},
		//{"GET", "/repos/:owner/:repo/pulls/comments"},
		//{"GET", "/repos/:owner/:repo/pulls/comments/:number"},
		//{"PUT", "/repos/:owner/:repo/pulls/:number/comments"},
		//{"PATCH", "/repos/:owner/:repo/pulls/comments/:number"},
		//{"DELETE", "/repos/:owner/:repo/pulls/comments/:number"},

		// Repositories
		{"/user/repos"},
		{"/users/:user/repos"},
		{"/orgs/:org/repos"},
		{"/repositories"},
		//{"POST", "/user/repos"},
		//{"POST", "/orgs/:org/repos"},
		{"/repos/:owner/:repo"},
		//{"PATCH", "/repos/:owner/:repo"},
		{"/repos/:owner/:repo/contributors"},
		{"/repos/:owner/:repo/languages"},
		{"/repos/:owner/:repo/teams"},
		{"/repos/:owner/:repo/tags"},
		{"/repos/:owner/:repo/branches"},
		{"/repos/:owner/:repo/branches/:branch"},
		//{"DELETE", "/repos/:owner/:repo"},
		{"/repos/:owner/:repo/collaborators"},
		{"/repos/:owner/:repo/collaborators/:user"},
		//{"PUT", "/repos/:owner/:repo/collaborators/:user"},
		//{"DELETE", "/repos/:owner/:repo/collaborators/:user"},
		{"/repos/:owner/:repo/comments"},
		{"/repos/:owner/:repo/commits/:sha/comments"},
		//{"POST", "/repos/:owner/:repo/commits/:sha/comments"},
		{"/repos/:owner/:repo/comments/:id"},
		//{"PATCH", "/repos/:owner/:repo/comments/:id"},
		//{"DELETE", "/repos/:owner/:repo/comments/:id"},
		{"/repos/:owner/:repo/commits"},
		{"/repos/:owner/:repo/commits/:sha"},
		{"/repos/:owner/:repo/readme"},
		//{"GET", "/repos/:owner/:repo/contents/*path"},
		//{"PUT", "/repos/:owner/:repo/contents/*path"},
		//{"DELETE", "/repos/:owner/:repo/contents/*path"},
		//{"GET", "/repos/:owner/:repo/:archive_format/:ref"},
		{"/repos/:owner/:repo/keys"},
		{"/repos/:owner/:repo/keys/:id"},
		//{"POST", "/repos/:owner/:repo/keys"},
		//{"PATCH", "/repos/:owner/:repo/keys/:id"},
		//{"DELETE", "/repos/:owner/:repo/keys/:id"},
		{"/repos/:owner/:repo/downloads"},
		{"/repos/:owner/:repo/downloads/:id"},
		//{"DELETE", "/repos/:owner/:repo/downloads/:id"},
		{"/repos/:owner/:repo/forks"},
		//{"POST", "/repos/:owner/:repo/forks"},
		{"/repos/:owner/:repo/hooks"},
		{"/repos/:owner/:repo/hooks/:id"},
		//{"POST", "/repos/:owner/:repo/hooks"},
		//{"PATCH", "/repos/:owner/:repo/hooks/:id"},
		//{"POST", "/repos/:owner/:repo/hooks/:id/tests"},
		//{"DELETE", "/repos/:owner/:repo/hooks/:id"},
		//{"POST", "/repos/:owner/:repo/merges"},
		{"/repos/:owner/:repo/releases"},
		{"/repos/:owner/:repo/releases/:id"},
		//{"POST", "/repos/:owner/:repo/releases"},
		//{"PATCH", "/repos/:owner/:repo/releases/:id"},
		//{"DELETE", "/repos/:owner/:repo/releases/:id"},
		{"/repos/:owner/:repo/releases/:id/assets"},
		{"/repos/:owner/:repo/stats/contributors"},
		{"/repos/:owner/:repo/stats/commit_activity"},
		{"/repos/:owner/:repo/stats/code_frequency"},
		{"/repos/:owner/:repo/stats/participation"},
		{"/repos/:owner/:repo/stats/punch_card"},
		{"/repos/:owner/:repo/statuses/:ref"},
		//{"POST", "/repos/:owner/:repo/statuses/:ref"},

		// Search
		{"/search/repositories"},
		{"/search/code"},
		{"/search/issues"},
		{"/search/users"},
		{"/legacy/issues/search/:owner/:repository/:state/:keyword"},
		{"/legacy/repos/search/:keyword"},
		{"/legacy/user/search/:keyword"},
		{"/legacy/user/email/:email"},

		// Users
		{"/users/:user"},
		{"/user"},
		//{"PATCH", "/user"},
		{"/users"},
		{"/user/emails"},
		//{"POST", "/user/emails"},
		//{"DELETE", "/user/emails"},
		{"/users/:user/followers"},
		{"/user/followers"},
		{"/users/:user/following"},
		{"/user/following"},
		{"/user/following/:user"},
		{"/users/:user/following/:target_user"},
		//{"PUT", "/user/following/:user"},
		//{"DELETE", "/user/following/:user"},
		{"/users/:user/keys"},
		{"/user/keys"},
		{"/user/keys/:id"},
		//{"POST", "/user/keys"},
		//{"PATCH", "/user/keys/:id"},
		//{"DELETE", "/user/keys/:id"},
	}

	for _, route := range static {
		tree.addRoute(route.path, fakeHandler(route.path))
	}
	ps := getParams()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, request := range static {
			tree.find(request.path, ps, false)
		}
	}
}

func BenchmarkTree_FindStaticTsr(b *testing.B) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/doc/foo/go_faq.html/",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}
	tr := testRequests{
		{"/doc/foo/go_faq.html", false, "/doc/foo/go_faq.html", nil},
	}
	ps := getParams()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, request := range tr {
			tree.find(request.path, ps, false)
		}
	}
}

func BenchmarkTree_FindParam(b *testing.B) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/hi/:key1/foo/:key2",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}
	tr := testRequests{
		{"/hi/1/foo/2", false, "/hi", nil},
	}
	ps := getParams()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, request := range tr {
			tree.find(request.path, ps, false)
		}
	}
}

func BenchmarkTree_FindParamTsr(b *testing.B) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/hi/:key1/foo/:key2/",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}
	tr := testRequests{
		{"/hi/1/foo/2", false, "/hi", nil},
	}
	ps := getParams()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, request := range tr {
			tree.find(request.path, ps, false)
		}
	}
}

func BenchmarkTree_FindAny(b *testing.B) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/hi/*key1",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}
	tr := testRequests{
		{"/hi/foo", false, "/hi", nil},
	}
	ps := getParams()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, request := range tr {
			tree.find(request.path, ps, false)
		}
	}
}

func BenchmarkTree_FindAnyFallback(b *testing.B) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	routes := [...]string{
		"/hi/a/b/c/d/e/*key1",
		"/*key2",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}
	tr := testRequests{
		{"/hi/a/b/c/d/f", false, "/*key2", nil},
	}
	ps := getParams()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, request := range tr {
			tree.find(request.path, ps, false)
		}
	}
}

func BenchmarkRouteStatic(b *testing.B) {
	r := NewEngine(config.NewOptions(nil))
	r.GET("/hi/foo", func(c context.Context, ctx *app.RequestContext) {})
	ctx := r.NewContext()
	req := protocol.NewRequest("GET", "/hi/foo", nil)
	req.CopyTo(&ctx.Request)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(context.Background(), ctx)
		// ctx.index = -1
	}
}

func BenchmarkRouteParam(b *testing.B) {
	r := NewEngine(config.NewOptions(nil))
	r.GET("/hi/:user", func(c context.Context, ctx *app.RequestContext) {})
	ctx := r.NewContext()
	req := protocol.NewRequest("GET", "/hi/foo", nil)
	req.CopyTo(&ctx.Request)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(context.Background(), ctx)
		// ctx.index = -1
	}
}

func BenchmarkRouteAny(b *testing.B) {
	r := NewEngine(config.NewOptions(nil))
	r.GET("/hi/*user", func(c context.Context, ctx *app.RequestContext) {})
	ctx := r.NewContext()
	req := protocol.NewRequest("GET", "/hi/foo/dy", nil)
	req.CopyTo(&ctx.Request)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(context.Background(), ctx)
		// ctx.index = -1
	}
}
