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
	"go/format"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/util"
)

type File struct {
	Path        string
	Content     string
	NoRepeat    bool
	FileTplName string
}

// Lint is used to statically analyze and format go code
func (file *File) Lint() error {
	name := filepath.Base(file.Path)
	if strings.HasSuffix(name, ".go") {
		out, err := format.Source(util.Str2Bytes(file.Content))
		if err != nil {
			return fmt.Errorf("lint file '%s' failed, err: %v", name, err.Error())
		}
		file.Content = util.Bytes2Str(out)
	}
	return nil
}
