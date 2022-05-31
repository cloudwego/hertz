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
	method string
	path   string
}

func BenchmarkTree_FindStatic(b *testing.B) {
	tree := &router{method: "GET", root: &node{}, hasTsrHandler: make(map[string]bool)}

	static := []*Route{
		{"GET", "/"},
		{"GET", "/cmd.html"},
		{"GET", "/code.html"},
		{"GET", "/contrib.html"},
		{"GET", "/contribute.html"},
		{"GET", "/debugging_with_gdb.html"},
		{"GET", "/docs.html"},
		{"GET", "/effective_go.html"},
		{"GET", "/files.log"},
		{"GET", "/gccgo_contribute.html"},
		{"GET", "/gccgo_install.html"},
		{"GET", "/go-logo-black.png"},
		{"GET", "/go-logo-blue.png"},
		{"GET", "/go-logo-white.png"},
		{"GET", "/go1.1.html"},
		{"GET", "/go1.2.html"},
		{"GET", "/go1.html"},
		{"GET", "/go1compat.html"},
		{"GET", "/go_faq.html"},
		{"GET", "/go_mem.html"},
		{"GET", "/go_spec.html"},
		{"GET", "/help.html"},
		{"GET", "/ie.css"},
		{"GET", "/install-source.html"},
		{"GET", "/install.html"},
		{"GET", "/logo-153x55.png"},
		{"GET", "/Makefile"},
		{"GET", "/root.html"},
		{"GET", "/share.png"},
		{"GET", "/sieve.gif"},
		{"GET", "/tos.html"},
		{"GET", "/articles/"},
		{"GET", "/articles/go_command.html"},
		{"GET", "/articles/index.html"},
		{"GET", "/articles/wiki/"},
		{"GET", "/articles/wiki/edit.html"},
		{"GET", "/articles/wiki/final-noclosure.go"},
		{"GET", "/articles/wiki/final-noerror.go"},
		{"GET", "/articles/wiki/final-parsetemplate.go"},
		{"GET", "/articles/wiki/final-template.go"},
		{"GET", "/articles/wiki/final.go"},
		{"GET", "/articles/wiki/get.go"},
		{"GET", "/articles/wiki/http-sample.go"},
		{"GET", "/articles/wiki/index.html"},
		{"GET", "/articles/wiki/Makefile"},
		{"GET", "/articles/wiki/notemplate.go"},
		{"GET", "/articles/wiki/part1-noerror.go"},
		{"GET", "/articles/wiki/part1.go"},
		{"GET", "/articles/wiki/part2.go"},
		{"GET", "/articles/wiki/part3-errorhandling.go"},
		{"GET", "/articles/wiki/part3.go"},
		{"GET", "/articles/wiki/test.bash"},
		{"GET", "/articles/wiki/test_edit.good"},
		{"GET", "/articles/wiki/test_Test.txt.good"},
		{"GET", "/articles/wiki/test_view.good"},
		{"GET", "/articles/wiki/view.html"},
		{"GET", "/codewalk/"},
		{"GET", "/codewalk/codewalk.css"},
		{"GET", "/codewalk/codewalk.js"},
		{"GET", "/codewalk/codewalk.xml"},
		{"GET", "/codewalk/functions.xml"},
		{"GET", "/codewalk/markov.go"},
		{"GET", "/codewalk/markov.xml"},
		{"GET", "/codewalk/pig.go"},
		{"GET", "/codewalk/popout.png"},
		{"GET", "/codewalk/run"},
		{"GET", "/codewalk/sharemem.xml"},
		{"GET", "/codewalk/urlpoll.go"},
		{"GET", "/devel/"},
		{"GET", "/devel/release.html"},
		{"GET", "/devel/weekly.html"},
		{"GET", "/gopher/"},
		{"GET", "/gopher/appenginegopher.jpg"},
		{"GET", "/gopher/appenginegophercolor.jpg"},
		{"GET", "/gopher/appenginelogo.gif"},
		{"GET", "/gopher/bumper.png"},
		{"GET", "/gopher/bumper192x108.png"},
		{"GET", "/gopher/bumper320x180.png"},
		{"GET", "/gopher/bumper480x270.png"},
		{"GET", "/gopher/bumper640x360.png"},
		{"GET", "/gopher/doc.png"},
		{"GET", "/gopher/frontpage.png"},
		{"GET", "/gopher/gopherbw.png"},
		{"GET", "/gopher/gophercolor.png"},
		{"GET", "/gopher/gophercolor16x16.png"},
		{"GET", "/gopher/help.png"},
		{"GET", "/gopher/pkg.png"},
		{"GET", "/gopher/project.png"},
		{"GET", "/gopher/ref.png"},
		{"GET", "/gopher/run.png"},
		{"GET", "/gopher/talks.png"},
		{"GET", "/gopher/pencil/"},
		{"GET", "/gopher/pencil/gopherhat.jpg"},
		{"GET", "/gopher/pencil/gopherhelmet.jpg"},
		{"GET", "/gopher/pencil/gophermega.jpg"},
		{"GET", "/gopher/pencil/gopherrunning.jpg"},
		{"GET", "/gopher/pencil/gopherswim.jpg"},
		{"GET", "/gopher/pencil/gopherswrench.jpg"},
		{"GET", "/play/"},
		{"GET", "/play/fib.go"},
		{"GET", "/play/hello.go"},
		{"GET", "/play/life.go"},
		{"GET", "/play/peano.go"},
		{"GET", "/play/pi.go"},
		{"GET", "/play/sieve.go"},
		{"GET", "/play/solitaire.go"},
		{"GET", "/play/tree.go"},
		{"GET", "/progs/"},
		{"GET", "/progs/cgo1.go"},
		{"GET", "/progs/cgo2.go"},
		{"GET", "/progs/cgo3.go"},
		{"GET", "/progs/cgo4.go"},
		{"GET", "/progs/defer.go"},
		{"GET", "/progs/defer.out"},
		{"GET", "/progs/defer2.go"},
		{"GET", "/progs/defer2.out"},
		{"GET", "/progs/eff_bytesize.go"},
		{"GET", "/progs/eff_bytesize.out"},
		{"GET", "/progs/eff_qr.go"},
		{"GET", "/progs/eff_sequence.go"},
		{"GET", "/progs/eff_sequence.out"},
		{"GET", "/progs/eff_unused1.go"},
		{"GET", "/progs/eff_unused2.go"},
		{"GET", "/progs/error.go"},
		{"GET", "/progs/error2.go"},
		{"GET", "/progs/error3.go"},
		{"GET", "/progs/error4.go"},
		{"GET", "/progs/go1.go"},
		{"GET", "/progs/gobs1.go"},
		{"GET", "/progs/gobs2.go"},
		{"GET", "/progs/image_draw.go"},
		{"GET", "/progs/image_package1.go"},
		{"GET", "/progs/image_package1.out"},
		{"GET", "/progs/image_package2.go"},
		{"GET", "/progs/image_package2.out"},
		{"GET", "/progs/image_package3.go"},
		{"GET", "/progs/image_package3.out"},
		{"GET", "/progs/image_package4.go"},
		{"GET", "/progs/image_package4.out"},
		{"GET", "/progs/image_package5.go"},
		{"GET", "/progs/image_package5.out"},
		{"GET", "/progs/image_package6.go"},
		{"GET", "/progs/image_package6.out"},
		{"GET", "/progs/interface.go"},
		{"GET", "/progs/interface2.go"},
		{"GET", "/progs/interface2.out"},
		{"GET", "/progs/json1.go"},
		{"GET", "/progs/json2.go"},
		{"GET", "/progs/json2.out"},
		{"GET", "/progs/json3.go"},
		{"GET", "/progs/json4.go"},
		{"GET", "/progs/json5.go"},
		{"GET", "/progs/run"},
		{"GET", "/progs/slices.go"},
		{"GET", "/progs/timeout1.go"},
		{"GET", "/progs/timeout2.go"},
		{"GET", "/progs/update.bash"},
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
		{"GET", "/authorizations"},
		{"GET", "/authorizations/:id"},
		//{"POST", "/authorizations"},
		//{"PUT", "/authorizations/clients/:client_id"},
		//{"PATCH", "/authorizations/:id"},
		//{"DELETE", "/authorizations/:id"},
		{"GET", "/applications/:client_id/tokens/:access_token"},
		{"DELETE", "/applications/:client_id/tokens"},
		//{"DELETE", "/applications/:client_id/tokens/:access_token"},

		// Activity
		{"GET", "/events"},
		{"GET", "/repos/:owner/:repo/events"},
		{"GET", "/networks/:owner/:repo/events"},
		{"GET", "/orgs/:org/events"},
		{"GET", "/users/:user/received_events"},
		{"GET", "/users/:user/received_events/public"},
		{"GET", "/users/:user/events"},
		{"GET", "/users/:user/events/public"},
		{"GET", "/users/:user/events/orgs/:org"},
		{"GET", "/feeds"},
		//{"GET", "/notifications"},
		{"GET", "/repos/:owner/:repo/notifications"},
		{"PUT", "/notifications"},
		//{"PUT", "/repos/:owner/:repo/notifications"},
		{"GET", "/notifications/threads/:id"},
		//{"PATCH", "/notifications/threads/:id"},
		{"GET", "/notifications/threads/:id/subscription"},
		//{"PUT", "/notifications/threads/:id/subscription"},
		//{"DELETE", "/notifications/threads/:id/subscription"},
		{"GET", "/repos/:owner/:repo/stargazers"},
		{"GET", "/users/:user/starred"},
		{"GET", "/user/starred"},
		{"GET", "/user/starred/:owner/:repo"},
		//{"PUT", "/user/starred/:owner/:repo"},
		//{"DELETE", "/user/starred/:owner/:repo"},
		{"GET", "/repos/:owner/:repo/subscribers"},
		{"GET", "/users/:user/subscriptions"},
		{"GET", "/user/subscriptions"},
		{"GET", "/repos/:owner/:repo/subscription"},
		//{"PUT", "/repos/:owner/:repo/subscription"},
		//{"DELETE", "/repos/:owner/:repo/subscription"},
		{"GET", "/user/subscriptions/:owner/:repo"},
		//{"PUT", "/user/subscriptions/:owner/:repo"},
		//{"DELETE", "/user/subscriptions/:owner/:repo"},

		// Gists
		{"GET", "/users/:user/gists"},
		{"GET", "/gists"},
		//{"GET", "/gists/public"},
		//{"GET", "/gists/starred"},
		{"GET", "/gists/:id"},
		//{"POST", "/gists"},
		//{"PATCH", "/gists/:id"},
		{"PUT", "/gists/:id/star"},
		//{"DELETE", "/gists/:id/star"},
		//{"GET", "/gists/:id/star"},
		{"POST", "/gists/:id/forks"},
		//{"DELETE", "/gists/:id"},

		// Git Data
		{"GET", "/repos/:owner/:repo/git/blobs/:sha"},
		{"POST", "/repos/:owner/:repo/git/blobs"},
		{"GET", "/repos/:owner/:repo/git/commits/:sha"},
		{"POST", "/repos/:owner/:repo/git/commits"},
		//{"GET", "/repos/:owner/:repo/git/refs/*ref"},
		{"GET", "/repos/:owner/:repo/git/refs"},
		//{"POST", "/repos/:owner/:repo/git/refs"},
		//{"PATCH", "/repos/:owner/:repo/git/refs/*ref"},
		//{"DELETE", "/repos/:owner/:repo/git/refs/*ref"},
		{"GET", "/repos/:owner/:repo/git/tags/:sha"},
		{"POST", "/repos/:owner/:repo/git/tags"},
		{"GET", "/repos/:owner/:repo/git/trees/:sha"},
		{"POST", "/repos/:owner/:repo/git/trees"},

		{"GET", "/issues"},
		{"GET", "/user/issues"},
		{"GET", "/orgs/:org/issues"},
		{"GET", "/repos/:owner/:repo/issues"},
		{"GET", "/repos/:owner/:repo/issues/:number"},
		//{"POST", "/repos/:owner/:repo/issues"},
		//{"PATCH", "/repos/:owner/:repo/issues/:number"},
		{"GET", "/repos/:owner/:repo/assignees"},
		{"GET", "/repos/:owner/:repo/assignees/:assignee"},
		{"GET", "/repos/:owner/:repo/issues/:number/comments"},
		//{"GET", "/repos/:owner/:repo/issues/comments"},
		//{"GET", "/repos/:owner/:repo/issues/comments/:id"},
		//{"POST", "/repos/:owner/:repo/issues/:number/comments"},
		//{"PATCH", "/repos/:owner/:repo/issues/comments/:id"},
		//{"DELETE", "/repos/:owner/:repo/issues/comments/:id"},
		{"GET", "/repos/:owner/:repo/issues/:number/events"},
		//{"GET", "/repos/:owner/:repo/issues/events"},
		//{"GET", "/repos/:owner/:repo/issues/events/:id"},
		{"GET", "/repos/:owner/:repo/labels"},
		{"GET", "/repos/:owner/:repo/labels/:name"},
		//{"POST", "/repos/:owner/:repo/labels"},
		//{"PATCH", "/repos/:owner/:repo/labels/:name"},
		//{"DELETE", "/repos/:owner/:repo/labels/:name"},
		{"GET", "/repos/:owner/:repo/issues/:number/labels"},
		//{"POST", "/repos/:owner/:repo/issues/:number/labels"},
		//{"DELETE", "/repos/:owner/:repo/issues/:number/labels/:name"},
		//{"PUT", "/repos/:owner/:repo/issues/:number/labels"},
		//{"DELETE", "/repos/:owner/:repo/issues/:number/labels"},
		{"GET", "/repos/:owner/:repo/milestones/:number/labels"},
		{"GET", "/repos/:owner/:repo/milestones"},
		{"GET", "/repos/:owner/:repo/milestones/:number"},
		//{"POST", "/repos/:owner/:repo/milestones"},
		//{"PATCH", "/repos/:owner/:repo/milestones/:number"},
		//{"DELETE", "/repos/:owner/:repo/milestones/:number"},

		// Miscellaneous
		{"GET", "/emojis"},
		{"GET", "/gitignore/templates"},
		{"GET", "/gitignore/templates/:name"},
		{"POST", "/markdown"},
		{"POST", "/markdown/raw"},
		{"GET", "/meta"},
		{"GET", "/rate_limit"},

		// Organizations
		{"GET", "/users/:user/orgs"},
		{"GET", "/user/orgs"},
		{"GET", "/orgs/:org"},
		//{"PATCH", "/orgs/:org"},
		{"GET", "/orgs/:org/members"},
		{"GET", "/orgs/:org/members/:user"},
		//{"DELETE", "/orgs/:org/members/:user"},
		{"GET", "/orgs/:org/public_members"},
		{"GET", "/orgs/:org/public_members/:user"},
		//{"PUT", "/orgs/:org/public_members/:user"},
		//{"DELETE", "/orgs/:org/public_members/:user"},
		{"GET", "/orgs/:org/teams"},
		{"GET", "/teams/:id"},
		//{"POST", "/orgs/:org/teams"},
		//{"PATCH", "/teams/:id"},
		//{"DELETE", "/teams/:id"},
		{"GET", "/teams/:id/members"},
		{"GET", "/teams/:id/members/:user"},
		//{"PUT", "/teams/:id/members/:user"},
		//{"DELETE", "/teams/:id/members/:user"},
		{"GET", "/teams/:id/repos"},
		{"GET", "/teams/:id/repos/:owner/:repo"},
		//{"PUT", "/teams/:id/repos/:owner/:repo"},
		//{"DELETE", "/teams/:id/repos/:owner/:repo"},
		{"GET", "/user/teams"},

		// Pull Requests
		{"GET", "/repos/:owner/:repo/pulls"},
		{"GET", "/repos/:owner/:repo/pulls/:number"},
		//{"POST", "/repos/:owner/:repo/pulls"},
		//{"PATCH", "/repos/:owner/:repo/pulls/:number"},
		{"GET", "/repos/:owner/:repo/pulls/:number/commits"},
		{"GET", "/repos/:owner/:repo/pulls/:number/files"},
		{"GET", "/repos/:owner/:repo/pulls/:number/merge"},
		//{"PUT", "/repos/:owner/:repo/pulls/:number/merge"},
		{"GET", "/repos/:owner/:repo/pulls/:number/comments"},
		//{"GET", "/repos/:owner/:repo/pulls/comments"},
		//{"GET", "/repos/:owner/:repo/pulls/comments/:number"},
		//{"PUT", "/repos/:owner/:repo/pulls/:number/comments"},
		//{"PATCH", "/repos/:owner/:repo/pulls/comments/:number"},
		//{"DELETE", "/repos/:owner/:repo/pulls/comments/:number"},

		// Repositories
		{"GET", "/user/repos"},
		{"GET", "/users/:user/repos"},
		{"GET", "/orgs/:org/repos"},
		{"GET", "/repositories"},
		//{"POST", "/user/repos"},
		//{"POST", "/orgs/:org/repos"},
		{"GET", "/repos/:owner/:repo"},
		//{"PATCH", "/repos/:owner/:repo"},
		{"GET", "/repos/:owner/:repo/contributors"},
		{"GET", "/repos/:owner/:repo/languages"},
		{"GET", "/repos/:owner/:repo/teams"},
		{"GET", "/repos/:owner/:repo/tags"},
		{"GET", "/repos/:owner/:repo/branches"},
		{"GET", "/repos/:owner/:repo/branches/:branch"},
		//{"DELETE", "/repos/:owner/:repo"},
		{"GET", "/repos/:owner/:repo/collaborators"},
		{"GET", "/repos/:owner/:repo/collaborators/:user"},
		//{"PUT", "/repos/:owner/:repo/collaborators/:user"},
		//{"DELETE", "/repos/:owner/:repo/collaborators/:user"},
		{"GET", "/repos/:owner/:repo/comments"},
		{"GET", "/repos/:owner/:repo/commits/:sha/comments"},
		//{"POST", "/repos/:owner/:repo/commits/:sha/comments"},
		{"GET", "/repos/:owner/:repo/comments/:id"},
		//{"PATCH", "/repos/:owner/:repo/comments/:id"},
		//{"DELETE", "/repos/:owner/:repo/comments/:id"},
		{"GET", "/repos/:owner/:repo/commits"},
		{"GET", "/repos/:owner/:repo/commits/:sha"},
		{"GET", "/repos/:owner/:repo/readme"},
		//{"GET", "/repos/:owner/:repo/contents/*path"},
		//{"PUT", "/repos/:owner/:repo/contents/*path"},
		//{"DELETE", "/repos/:owner/:repo/contents/*path"},
		//{"GET", "/repos/:owner/:repo/:archive_format/:ref"},
		{"GET", "/repos/:owner/:repo/keys"},
		{"GET", "/repos/:owner/:repo/keys/:id"},
		//{"POST", "/repos/:owner/:repo/keys"},
		//{"PATCH", "/repos/:owner/:repo/keys/:id"},
		//{"DELETE", "/repos/:owner/:repo/keys/:id"},
		{"GET", "/repos/:owner/:repo/downloads"},
		{"GET", "/repos/:owner/:repo/downloads/:id"},
		//{"DELETE", "/repos/:owner/:repo/downloads/:id"},
		{"GET", "/repos/:owner/:repo/forks"},
		//{"POST", "/repos/:owner/:repo/forks"},
		{"GET", "/repos/:owner/:repo/hooks"},
		{"GET", "/repos/:owner/:repo/hooks/:id"},
		//{"POST", "/repos/:owner/:repo/hooks"},
		//{"PATCH", "/repos/:owner/:repo/hooks/:id"},
		//{"POST", "/repos/:owner/:repo/hooks/:id/tests"},
		//{"DELETE", "/repos/:owner/:repo/hooks/:id"},
		//{"POST", "/repos/:owner/:repo/merges"},
		{"GET", "/repos/:owner/:repo/releases"},
		{"GET", "/repos/:owner/:repo/releases/:id"},
		//{"POST", "/repos/:owner/:repo/releases"},
		//{"PATCH", "/repos/:owner/:repo/releases/:id"},
		//{"DELETE", "/repos/:owner/:repo/releases/:id"},
		{"GET", "/repos/:owner/:repo/releases/:id/assets"},
		{"GET", "/repos/:owner/:repo/stats/contributors"},
		{"GET", "/repos/:owner/:repo/stats/commit_activity"},
		{"GET", "/repos/:owner/:repo/stats/code_frequency"},
		{"GET", "/repos/:owner/:repo/stats/participation"},
		{"GET", "/repos/:owner/:repo/stats/punch_card"},
		{"GET", "/repos/:owner/:repo/statuses/:ref"},
		//{"POST", "/repos/:owner/:repo/statuses/:ref"},

		// Search
		{"GET", "/search/repositories"},
		{"GET", "/search/code"},
		{"GET", "/search/issues"},
		{"GET", "/search/users"},
		{"GET", "/legacy/issues/search/:owner/:repository/:state/:keyword"},
		{"GET", "/legacy/repos/search/:keyword"},
		{"GET", "/legacy/user/search/:keyword"},
		{"GET", "/legacy/user/email/:email"},

		// Users
		{"GET", "/users/:user"},
		{"GET", "/user"},
		//{"PATCH", "/user"},
		{"GET", "/users"},
		{"GET", "/user/emails"},
		//{"POST", "/user/emails"},
		//{"DELETE", "/user/emails"},
		{"GET", "/users/:user/followers"},
		{"GET", "/user/followers"},
		{"GET", "/users/:user/following"},
		{"GET", "/user/following"},
		{"GET", "/user/following/:user"},
		{"GET", "/users/:user/following/:target_user"},
		//{"PUT", "/user/following/:user"},
		//{"DELETE", "/user/following/:user"},
		{"GET", "/users/:user/keys"},
		{"GET", "/user/keys"},
		{"GET", "/user/keys/:id"},
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
