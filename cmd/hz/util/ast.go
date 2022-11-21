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
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/ast/astutil"
)

func AddImport(file, alias, impt string) (string, error) {
	fset := token.NewFileSet()
	path, _ := filepath.Abs(file)
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("can not parse ast for file: %s", path)
	}
	added := astutil.AddNamedImport(fset, f, alias, impt)
	if !added {
		return "", fmt.Errorf("can not add import \"%s\" for file: %s", impt, path)
	}
	var output []byte
	buffer := bytes.NewBuffer(output)
	err = format.Node(buffer, fset, f)
	if err != nil {
		return "", fmt.Errorf("can add import for file: %s", path)
	}

	return buffer.String(), nil
}
