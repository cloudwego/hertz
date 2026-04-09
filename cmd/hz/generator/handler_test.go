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
	"fmt"
	"regexp"
	"testing"
)

func TestHandlerMethodExistsMatch(t *testing.T) {
	methodName := "CreateUser"
	re := regexp.MustCompile(fmt.Sprintf(`func\s+(\([^)]*\)\s+)?%s\s*\(`, regexp.QuoteMeta(methodName)))

	tests := []struct {
		name  string
		file  string
		match bool
	}{
		{
			name:  "plain function",
			file:  `func CreateUser(ctx context.Context, c *app.RequestContext) {`,
			match: true,
		},
		{
			name:  "receiver method pointer",
			file:  `func (handler *APIHandler) CreateUser(ctx context.Context, c *app.RequestContext) {`,
			match: true,
		},
		{
			name:  "receiver method value",
			file:  `func (h APIHandler) CreateUser(ctx context.Context, c *app.RequestContext) {`,
			match: true,
		},
		{
			name:  "different method",
			file:  `func CreateRole(ctx context.Context, c *app.RequestContext) {`,
			match: false,
		},
		{
			name:  "prefix mismatch",
			file:  `func CreateUserV2(ctx context.Context, c *app.RequestContext) {`,
			match: false,
		},
		{
			name:  "in comment",
			file:  `// func CreateUser is deprecated`,
			match: false,
		},
		{
			name:  "multiline file with receiver",
			file:  "package handler\n\nfunc (s *Service) CreateUser(ctx context.Context) {\n}\n",
			match: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := re.Match([]byte(tt.file))
			if got != tt.match {
				t.Errorf("match = %v, want %v for input: %s", got, tt.match, tt.file)
			}
		})
	}
}
