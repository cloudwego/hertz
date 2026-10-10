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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func Test_checkDupRegister(t *testing.T) {
	type args struct {
		file      []byte
		insertReg string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "dup tab",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app\"\n)\n\nfunc register() {\n\tapp.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: true,
		},
		{
			name: "dup space",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app\"\n)\n\nfunc register() {\n   app.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: true,
		},
		{
			name: "not dup prefix",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/app_2\"\n)\n\nfunc register() {\n\tapp_2.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: false,
		},
		{
			name: "not dup subfix",
			args: args{
				file:      []byte("package main\n\nimport (\n\t\"hertz.io/hertz/pkg/xapp\"\n)\n\nfunc register() {\n xapp.Register(r)\n}"),
				insertReg: "app.Register(r)\n",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkDupRegister(tt.args.file, tt.args.insertReg); got != tt.want {
				t.Errorf("checkDupRegister() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateMiddlewareRegCustomNames(t *testing.T) {
	const rawName = "{{if ne .MiddleWare \"root\"}}_{{end}}{{untitle .RawName}}Mw"
	tests := []struct {
		name         string
		templateName string
		insertKey    string
		snakeStyle   bool
		router       Router
		functions    []string
	}{
		{
			name: "default", templateName: "{{.MiddleWare}}Mw",
			router: Router{Router: &RouterNode{
				Path: "/:collectionUid", GroupMiddleware: "_collectionuid",
				HandlerMiddleware: "_getllmresponse", Handler: "handler.GetLLMResponse",
				Children: childrenRouterInfo{{
					Path: "/duplicate", HandlerMiddleware: "_getllmresponse0",
					Handler: "handler.GetLLMResponse",
				}},
			}},
			functions: []string{"_collectionuidMw", "_getllmresponseMw", "_getllmresponse0Mw"},
		},
		{
			name: "custom", templateName: rawName, insertKey: "func " + rawName + "(",
			router: Router{Router: &RouterNode{
				Path: "/:collectionUid", GroupMiddleware: "_collectionuid",
				HandlerMiddleware: "_getllmresponse", Handler: "handler.GetLLMResponse",
				Children: childrenRouterInfo{{
					Path: "/duplicate", HandlerMiddleware: "_getllmresponse0",
					Handler: "handler.GetLLMResponse",
				}},
			}},
			functions: []string{"_collectionUidMw", "_getLLMResponseMw", "_getLLMResponse0Mw"},
		},
		{
			name: "v2 custom", templateName: rawName, insertKey: "func " + rawName + "(", snakeStyle: true,
			router: Router{Router: &RouterNode{
				Path: "/marketplace", PathPrefix: "_v1_skills_marketplace",
				GroupMiddleware: "_v1_skills_marketplace", HandlerMiddleware: "_GetLLMResponse",
				Handler: "handler.GetLLMResponse", Children: childrenRouterInfo{{
					Path: "/duplicate", HandlerMiddleware: "_GetLLMResponse0",
					Handler: "handler.GetLLMResponse",
				}},
			}},
			functions: []string{"_v1_skills_marketplaceMw", "_getLLMResponseMw", "_getLLMResponse0Mw"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &Template{}
			info.UpdateBehavior.InsertKey = tt.insertKey
			gen := &HttpPackageGenerator{SnakeStyleMiddleware: tt.snakeStyle}
			gen.tplsInfo = map[string]*Template{middlewareTplName: {}, middlewareSingleTplName: info}
			gen.tpls = map[string]*template.Template{
				middlewareSingleTplName: template.Must(template.New("single").Funcs(funcMap).Parse("\nfunc " + tt.templateName + "() []app.HandlerFunc { return nil }\n")),
			}
			path := filepath.Join(t.TempDir(), "middleware.go")
			source := "package service\nfunc " + tt.functions[1] + "() []app.HandlerFunc { return authenticate() }\n"
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := gen.updateMiddlewareReg(tt.router, middlewareTplName, path); err != nil {
					t.Fatal(err)
				}
				got := gen.files[len(gen.files)-1].Content
				if !strings.HasPrefix(got, source) {
					t.Fatal("existing middleware changed")
				}
				for _, fn := range tt.functions {
					if strings.Count(got, "func "+fn+"()") != 1 {
						t.Fatalf("missing or duplicate %s: %s", fn, got)
					}
				}
				if err := os.WriteFile(path, []byte(got), 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRawGroupName(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/", "root"},
		{"/:collectionUid", "collectionUid"},
		{"/:1b", "1b"},
		{"/userID", "userID"},
		{"/*filePath", "_2AfilePath"},
	} {
		node := &RouterNode{Path: tc.path}
		if got := node.RawGroupName(); got != tc.want {
			t.Errorf("RawGroupName(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestRawHandlerName(t *testing.T) {
	for _, tc := range []struct{ handler, want string }{
		{"handler.GetLLMResponse", "GetLLMResponse"},
		{"GetLLMResponse", "GetLLMResponse"},
	} {
		node := &RouterNode{Handler: tc.handler}
		if got := node.RawHandlerName(); got != tc.want {
			t.Errorf("RawHandlerName(%q) = %q, want %q", tc.handler, got, tc.want)
		}
	}
}

func TestRawHandlerMiddleware(t *testing.T) {
	for _, tc := range []struct{ handler, middleware, want string }{
		{"handler.GetLLMResponse", "_getllmresponse", "GetLLMResponse"},
		{"handler.GetLLMResponse", "_getllmresponse0", "GetLLMResponse0"},
		{"handler.GetLLMResponse", "_GetLLMResponse0", "GetLLMResponse0"},
	} {
		node := &RouterNode{Handler: tc.handler, HandlerMiddleware: tc.middleware}
		if got := node.RawHandlerMiddleware(); got != tc.want {
			t.Errorf("RawHandlerMiddleware(%q, %q) = %q, want %q",
				tc.handler, tc.middleware, got, tc.want)
		}
	}
}

func TestRawGroupMiddleware(t *testing.T) {
	for _, tc := range []struct{ path, pathPrefix, middleware, want string }{
		{"/", "", "root", "root"},
		{"/:collectionUid", "", "_collectionuid", "collectionUid"},
		{"/:collectionUid", "", "_collectionuid0", "collectionUid0"},
		{"/:1b", "", "_1b", "1b"},
		{"/:1b", "", "_1b0", "1b0"},
		{"/:1TestA", "", "_1testa", "1TestA"},
		{"/:1TestA", "", "_1testa0", "1TestA0"},
		{"/marketplace", "_v1_skills_marketplace", "_v1_skills_marketplace", "v1_skills_marketplace"},
		{"/marketplace", "_v1_skills_marketplace", "_v1_skills_marketplace0", "v1_skills_marketplace0"},
	} {
		node := &RouterNode{Path: tc.path, PathPrefix: tc.pathPrefix, GroupMiddleware: tc.middleware}
		if got := node.RawGroupMiddleware(); got != tc.want {
			t.Errorf("RawGroupMiddleware(%q, %q, %q) = %q, want %q",
				tc.path, tc.pathPrefix, tc.middleware, got, tc.want)
		}
	}
}
