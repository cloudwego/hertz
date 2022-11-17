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

package util

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/ast/astutil"
)

func TestAddImport(t *testing.T) {
	inserts := [][]string{
		{
			"ctx",
			"context",
		},
		{
			"",
			"context",
		},
	}
	files := [][]string{
		{
			`package foo

import (
	"fmt"
	"time"
)
`,
			`package foo

import (
	ctx "context"
	"fmt"
	"time"
)
`,
		},
		{
			`package foo

import (
	"fmt"
	"time"
)
`,
			`package foo

import (
	"context"
	"fmt"
	"time"
)
`,
		},
	}
	for idx, file := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "", file[0], parser.ImportsOnly)
		if err != nil {
			t.Fatalf("can not parse ast for file")
		}
		astutil.AddNamedImport(fset, f, inserts[idx][0], inserts[idx][1])
		var output []byte
		buffer := bytes.NewBuffer(output)
		err = format.Node(buffer, fset, f)
		if err != nil {
			t.Fatalf("can add import for file")
		}
		if buffer.String() != file[1] {
			t.Fatalf("insert import fialed")
		}
	}
}
