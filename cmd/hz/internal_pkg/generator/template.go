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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/meta"
	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/util"
	"github.com/cloudwego/hertz/cmd/hz/internal_pkg/util/logs"
)

var DefaultDelimiters = [2]string{"{{", "}}"}

type TemplateConfig struct {
	Layouts []Template `yaml:"layouts"`
}

type Template struct {
	Path   string    `yaml:"path"`   // The generated path and its filename, such as biz/handler/ping.go
	Delims [2]string `yaml:"delims"` // Template Action Instruction Identifier，default: "{{}}"
	Body   string    `yaml:"body"`   // Render template, currently only supports go template syntax
}

// TemplateGenerator contains information about the output template
type TemplateGenerator struct {
	OutputDir string
	Config    *TemplateConfig
	Excludes  []string
	tpls      map[string]*template.Template
	dirs      map[string]bool

	files         []File
	excludedFiles map[string]*File
}

func (tg *TemplateGenerator) Init() error {
	if tg.Config == nil {
		return errors.New("config not set yet")
	}

	tpls := tg.tpls
	if tpls == nil {
		tpls = make(map[string]*template.Template, len(tg.Config.Layouts))
	}
	dirs := tg.dirs
	if dirs == nil {
		dirs = make(map[string]bool)
	}

	for _, l := range tg.Config.Layouts {
		// check if is a directory
		var noFile bool
		if strings.HasSuffix(l.Path, string(filepath.Separator)) {
			noFile = true
		}
		path := l.Path
		if filepath.IsAbs(path) {
			return fmt.Errorf("absolute template path '%s' is not allowed", path)
		}
		dir := filepath.Dir(path)
		isExist, err := util.PathExist(filepath.Join(tg.OutputDir, dir))
		if err != nil {
			return fmt.Errorf("check directory '%s' failed, err: %v", dir, err.Error())
		}
		if isExist {
			dirs[dir] = true
		} else {
			dirs[dir] = false
		}

		if noFile {
			continue
		}

		// parse templates
		if _, ok := tpls[path]; ok {
			continue
		}
		delims := DefaultDelimiters
		if l.Delims[0] != "" && l.Delims[1] != "" {
			delims = l.Delims
		}
		tpl := template.New(path)
		tpl = tpl.Delims(delims[0], delims[1])
		if tpl, err = tpl.Parse(l.Body); err != nil {
			return fmt.Errorf("parse template '%s' failed, err: %v", path, err.Error())
		}

		tpls[path] = tpl
	}

	excludes := make(map[string]*File, len(tg.Excludes))
	for _, f := range tg.Excludes {
		excludes[f] = &File{}
	}

	tg.tpls = tpls
	tg.dirs = dirs
	tg.excludedFiles = excludes
	return nil
}

func (tg *TemplateGenerator) Generate(input interface{}, tplName, filepath string, noRepeat bool) error {
	// check if "*" (global scope) data exists, and stores it to all
	var all map[string]interface{}
	if data, ok := input.(map[string]interface{}); ok {
		ad, ok := data["*"]
		if ok {
			all = ad.(map[string]interface{})
		}
		if all == nil {
			all = map[string]interface{}{}
		}
		all["hzVersion"] = meta.Version
	}

	file := bytes.NewBuffer(nil)
	if tplName != "" {
		tpl := tg.tpls[tplName]
		if tpl == nil {
			return fmt.Errorf("tpl %s not found", tplName)
		}
		if err := tpl.Execute(file, input); err != nil {
			return fmt.Errorf("render template '%s' failed, err: %v", tplName, err.Error())
		}

		in := File{filepath, string(file.Bytes()), noRepeat, tplName}
		tg.files = append(tg.files, in)
		return nil
	}

	for path, tpl := range tg.tpls {
		file.Reset()
		var fd interface{}
		// search and merge rendering data
		if data, ok := input.(map[string]interface{}); ok {
			td := map[string]interface{}{}
			tmp, ok := data[path]
			if ok {
				td = tmp.(map[string]interface{})
			}
			for k, v := range all {
				td[k] = v
			}
			fd = td
		} else {
			fd = input
		}
		if err := tpl.Execute(file, fd); err != nil {
			return fmt.Errorf("render template '%s' failed, err: %v", path, err.Error())
		}

		in := File{path, string(file.Bytes()), noRepeat, tpl.Name()}
		tg.files = append(tg.files, in)
	}

	return nil
}

func (tg *TemplateGenerator) Persist() error {
	files := tg.files
	outPath := tg.OutputDir
	if !filepath.IsAbs(outPath) {
		outPath, _ = filepath.Abs(outPath)
	}

	for _, data := range files {
		// check for -E flags
		if _, ok := tg.excludedFiles[filepath.Join(data.Path)]; ok {
			continue
		}

		// lint file
		if err := data.Lint(); err != nil {
			return err
		}

		// create rendered file
		abPath := filepath.Join(outPath, data.Path)
		abDir := filepath.Dir(abPath)
		isExist, err := util.PathExist(abDir)
		if err != nil {
			return fmt.Errorf("check directory '%s' failed, err: %v", abDir, err.Error())
		}
		if !isExist {
			if err := os.MkdirAll(abDir, os.FileMode(0o744)); err != nil {
				return fmt.Errorf("mkdir %s failed, err: %v", abDir, err.Error())
			}
		}
		file, err := os.OpenFile(abPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.FileMode(0o755))
		defer file.Close()
		if err != nil {
			return fmt.Errorf("open file '%s' failed, err: %v", abPath, err.Error())
		}
		if _, err = file.WriteString(data.Content); err != nil {
			return fmt.Errorf("write file '%s' failed, err: %v", abPath, err.Error())
		}
	}

	tg.files = tg.files[:0]
	return nil
}

func (tg *TemplateGenerator) GetFormatAndExcludedFiles() ([]File, error) {
	var files []File
	outPath := tg.OutputDir
	if !filepath.IsAbs(outPath) {
		outPath, _ = filepath.Abs(outPath)
	}

	for _, data := range tg.Files() {
		if _, ok := tg.excludedFiles[filepath.Join(data.Path)]; ok {
			continue
		}

		// check repeat files
		logs.Infof("Write %s", data.Path)
		isExist, err := util.PathExist(filepath.Join(data.Path))
		if err != nil {
			return nil, fmt.Errorf("check file '%s' failed, err: %v", data.Path, err.Error())
		}
		if isExist && data.NoRepeat {
			if data.FileTplName == handlerTplName {
				logs.Warnf("Handler file(%s) has been generated.\n If you want to re-generate it, please copy and delete the file to prevent the already written code from being deleted.", data.Path)
			} else if data.FileTplName == routerTplName {
				logs.Warnf("Router file(%s) has been generated.\n If you want to re-generate it, please delete the file.", data.Path)
			} else {
				logs.Warnf("file '%s' already exists, so drop the generated file", data.Path)
			}
			continue
		}

		// lint file
		if err := data.Lint(); err != nil {
			logs.Warnf("Lint file: %s failed:\n %s\n", data.Path, data.Content)
		}
		files = append(files, data)
	}

	return files, nil
}

func (tg *TemplateGenerator) Files() []File {
	return tg.files
}

func (tg *TemplateGenerator) Degenerate() error {
	outPath := tg.OutputDir
	if !filepath.IsAbs(outPath) {
		outPath, _ = filepath.Abs(outPath)
	}
	for path := range tg.tpls {
		abPath := filepath.Join(outPath, path)
		if err := os.RemoveAll(abPath); err != nil {
			return fmt.Errorf("remove file '%s' failed, err: %v", path, err.Error())
		}
	}
	for dir, exist := range tg.dirs {
		if !exist {
			abDir := filepath.Join(outPath, dir)
			if err := os.RemoveAll(abDir); err != nil {
				return fmt.Errorf("remove directory '%s' failed, err: %v", dir, err.Error())
			}
		}
	}
	return nil
}
