/*
 * Copyright 2023 CloudWeGo Authors
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

package decoder

import (
	"errors"
	"reflect"
	"strings"
)

const (
	pathTag     = "path"
	formTag     = "form"
	queryTag    = "query"
	cookieTag   = "cookie"
	headerTag   = "header"
	jsonTag     = "json"
	rawBodyTag  = "raw_body"
	fileNameTag = "file_name"
)

const (
	defaultTag = "default"
)

const (
	requiredTagOpt = "required"
)

type TagInfo struct {
	Tag         string
	Name        string
	Required    bool
	Options     []string
	Getter      getter
	SliceGetter sliceGetter
}

func NewTagInfo(tag, name string) *TagInfo {
	ti := &TagInfo{Tag: tag, Name: name}
	ti.Getter = tag2getter[ti.Tag]
	ti.SliceGetter = tag2sliceGetter[ti.Tag]
	return ti
}

func (ti *TagInfo) IsJSON() bool {
	return ti.Tag == jsonTag
}

var errSkipped = errors.New("skipped")

func (ti *TagInfo) Parse(tagvalue string) error {
	tagname, opts, _ := strings.Cut(tagvalue, ",")
	if len(tagname) > 0 {
		ti.Name = tagname
	}
	ti.Options = ti.Options[:0]
	for opts != "" {
		o := ""
		o, opts, _ = strings.Cut(opts, ",")
		if o == requiredTagOpt {
			ti.Required = true
		}
		ti.Options = append(ti.Options, o)
	}
	return nil
}

func (ti *TagInfo) Skip() bool { return ti.Name == "-" }

func lookupFieldTags(field reflect.StructField, config *DecodeConfig) []*TagInfo {
	var tagInfos []*TagInfo
	for _, tag := range []string{pathTag, formTag, queryTag, cookieTag, headerTag, jsonTag, rawBodyTag, fileNameTag} {
		tagv, ok := field.Tag.Lookup(tag)
		if !ok {
			continue
		}

		ti := NewTagInfo(tag, field.Name)
		if ti.Parse(tagv) == errSkipped {
			continue
		}
		tagInfos = append(tagInfos, ti)
	}
	return tagInfos
}

func getDefaultFieldTags(f reflect.StructField) (ret []*TagInfo) {
	for _, tag := range []string{pathTag, formTag, queryTag, cookieTag, headerTag, jsonTag, fileNameTag} {
		ti := NewTagInfo(tag, f.Name)
		ret = append(ret, ti)
	}
	return
}

func getFieldTagInfoByTag(field reflect.StructField, tag string) *TagInfo {
	ti := NewTagInfo(tag, field.Name)
	v, ok := field.Tag.Lookup(tag)
	if !ok {
		return ti
	}
	ti.Parse(v)
	return ti
}
