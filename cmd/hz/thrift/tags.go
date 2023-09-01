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

package thrift

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/cmd/hz/config"
	"github.com/cloudwego/hertz/cmd/hz/generator"
	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/util"
	"github.com/cloudwego/thriftgo/parser"
)

const (
	AnnotationQuery    = "api.query"
	AnnotationForm     = "api.form"
	AnnotationPath     = "api.path"
	AnnotationHeader   = "api.header"
	AnnotationCookie   = "api.cookie"
	AnnotationBody     = "api.body"
	AnnotationRawBody  = "api.raw_body"
	AnnotationJsConv   = "api.js_conv"
	AnnotationNone     = "api.none"
	AnnotationFileName = "api.file_name"

	AnnotationValidator = "api.vd"

	AnnotationGoTag = "go.tag"
)

const (
	ApiGet        = "api.get"
	ApiPost       = "api.post"
	ApiPut        = "api.put"
	ApiPatch      = "api.patch"
	ApiDelete     = "api.delete"
	ApiOptions    = "api.options"
	ApiHEAD       = "api.head"
	ApiAny        = "api.any"
	ApiPath       = "api.path"
	ApiSerializer = "api.serializer"
	ApiGenPath    = "api.handler_path"
)

const (
	ApiBaseDomain    = "api.base_domain"
	ApiServiceGroup  = "api.service_group"
	ApiServiceGenDir = "api.service_gen_dir" // handler_dir for handler_by_service
	ApiServicePath   = "api.service_path"    // declare the path to the service's handler according to this annotation for handler_by_method
)

var (
	HttpMethodAnnotations = map[string]string{
		ApiGet:     "GET",
		ApiPost:    "POST",
		ApiPut:     "PUT",
		ApiPatch:   "PATCH",
		ApiDelete:  "DELETE",
		ApiOptions: "OPTIONS",
		ApiHEAD:    "HEAD",
		ApiAny:     "ANY",
	}

	HttpMethodOptionAnnotations = map[string]string{
		ApiGenPath: "handler_path",
	}

	BindingTags = map[string]string{
		AnnotationPath:    "path",
		AnnotationQuery:   "query",
		AnnotationHeader:  "header",
		AnnotationCookie:  "cookie",
		AnnotationBody:    "json",
		AnnotationForm:    "form",
		AnnotationRawBody: "raw_body",
	}

	SerializerTags = map[string]string{
		ApiSerializer: "serializer",
	}

	ValidatorTags = map[string]string{AnnotationValidator: "vd"}
)

var (
	jsonSnakeName  = false
	unsetOmitempty = false
)

func CheckTagOption(args *config.Argument) []generator.Option {
	var ret []generator.Option
	if args == nil {
		return ret
	}
	if args.SnakeName {
		jsonSnakeName = true
	}
	if args.UnsetOmitempty {
		unsetOmitempty = true
	}
	if args.JSONEnumStr {
		ret = append(ret, generator.OptionMarshalEnumToText)
	}
	return ret
}

func checkSnakeName(name string) string {
	if jsonSnakeName {
		name = util.ToSnakeCase(name)
	}
	return name
}

func getAnnotation(input parser.Annotations, target string) []string {
	if len(input) == 0 {
		return nil
	}
	for _, anno := range input {
		if strings.ToLower(anno.Key) == target {
			return anno.Values
		}
	}

	return []string{}
}

type httpAnnotation struct {
	method string
	path   []string
}

type httpAnnotations []httpAnnotation

func (s httpAnnotations) Len() int {
	return len(s)
}

func (s httpAnnotations) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s httpAnnotations) Less(i, j int) bool {
	return s[i].method < s[j].method
}

func getAnnotations(input parser.Annotations, targets map[string]string) map[string][]string {
	if len(input) == 0 || len(targets) == 0 {
		return nil
	}
	out := map[string][]string{}
	for k, t := range targets {
		var ret *parser.Annotation
		for _, anno := range input {
			if strings.ToLower(anno.Key) == k {
				ret = anno
				break
			}
		}
		if ret == nil {
			continue
		}
		out[t] = ret.Values
	}
	return out
}

func defaultBindingTags(f *parser.Field) []model.Tag {
	out := make([]model.Tag, 3)
	bindingTags := []string{
		AnnotationQuery,
		AnnotationForm,
		AnnotationPath,
		AnnotationHeader,
		AnnotationCookie,
		AnnotationBody,
		AnnotationRawBody,
	}

	for _, tag := range bindingTags {
		if v := getAnnotation(f.Annotations, tag); len(v) > 0 {
			out[0] = jsonTag(f)
			return out[:1]
		}
	}

	if v := getAnnotation(f.Annotations, AnnotationBody); len(v) > 0 {
		val := getJsonValue(f, v[0])
		out[0] = tag("json", val)
	} else {
		out[0] = jsonTag(f)
	}
	if v := getAnnotation(f.Annotations, AnnotationQuery); len(v) > 0 {
		val := checkRequire(f, v[0])
		out[1] = tag(BindingTags[AnnotationQuery], val)
	} else {
		val := checkRequire(f, checkSnakeName(f.Name))
		out[1] = tag(BindingTags[AnnotationQuery], val)
	}
	if v := getAnnotation(f.Annotations, AnnotationForm); len(v) > 0 {
		val := checkRequire(f, v[0])
		out[2] = tag(BindingTags[AnnotationForm], val)
	} else {
		val := checkRequire(f, checkSnakeName(f.Name))
		out[2] = tag(BindingTags[AnnotationForm], val)
	}
	return out
}

func jsonTag(f *parser.Field) (ret model.Tag) {
	ret.Key = "json"
	ret.Value = checkSnakeName(f.Name)

	if v := getAnnotation(f.Annotations, AnnotationJsConv); len(v) > 0 {
		ret.Value += ",string"
	}
	if !unsetOmitempty && f.Requiredness == parser.FieldType_Optional {
		ret.Value += ",omitempty"
	} else if f.Requiredness == parser.FieldType_Required {
		ret.Value += ",required"
	}
	return
}

func tag(k, v string) model.Tag {
	return model.Tag{
		Key:   k,
		Value: v,
	}
}

func annotationToTags(as parser.Annotations, targets map[string]string) (tags []model.Tag) {
	rets := getAnnotations(as, targets)
	for k, v := range rets {
		for _, vv := range v {
			tags = append(tags, model.Tag{
				Key:   k,
				Value: vv,
			})
		}
	}
	return
}

func injectTags(f *parser.Field, gf *model.Field, needDefault, needGoTag bool) error {
	as := f.Annotations
	if as == nil {
		as = parser.Annotations{}
	}
	tags := gf.Tags
	if tags == nil {
		tags = make([]model.Tag, 0, len(as))
	}

	if needDefault {
		tags = append(tags, defaultBindingTags(f)...)
	}

	// binding tags
	bts := annotationToTags(as, BindingTags)
	for _, t := range bts {
		key := t.Key
		tags.Remove(key)
		if key == "json" {
			formVal := t.Value
			t.Value = getJsonValue(f, t.Value)
			formVal = checkRequire(f, formVal)
			tags = append(tags, tag("form", formVal))
		} else {
			t.Value = checkRequire(f, t.Value)
		}
		tags = append(tags, t)
	}

	// validator tags
	tags = append(tags, annotationToTags(as, ValidatorTags)...)

	// the tag defined by gotag with higher priority
	checkGoTag(as, &tags)

	// go.tags for compiler mode
	if needGoTag {
		rets := getAnnotation(as, AnnotationGoTag)
		for _, v := range rets {
			gts := util.SplitGoTags(v)
			for _, gt := range gts {
				sp := strings.SplitN(gt, ":", 2)
				if len(sp) != 2 {
					return fmt.Errorf("invalid go tag: %s", v)
				}
				vv, err := strconv.Unquote(sp[1])
				if err != nil {
					return fmt.Errorf("invalid go.tag value: %s, err: %v", sp[1], err.Error())
				}
				key := sp[0]
				tags.Remove(key)
				tags = append(tags, model.Tag{
					Key:   key,
					Value: vv,
				})
			}
		}
	}

	sort.Sort(tags)
	gf.Tags = tags
	return nil
}

func getJsonValue(f *parser.Field, val string) string {
	if v := getAnnotation(f.Annotations, AnnotationJsConv); len(v) > 0 {
		val += ",string"
	}
	if !unsetOmitempty && f.Requiredness == parser.FieldType_Optional {
		val += ",omitempty"
	} else if f.Requiredness == parser.FieldType_Required {
		val += ",required"
	}

	return val
}

func checkRequire(f *parser.Field, val string) string {
	if f.Requiredness == parser.FieldType_Required {
		val += ",required"
	}

	return val
}

// checkGoTag removes the tag defined in gotag
func checkGoTag(as parser.Annotations, tags *model.Tags) error {
	rets := getAnnotation(as, AnnotationGoTag)
	for _, v := range rets {
		gts := util.SplitGoTags(v)
		for _, gt := range gts {
			sp := strings.SplitN(gt, ":", 2)
			if len(sp) != 2 {
				return fmt.Errorf("invalid go tag: %s", v)
			}
			key := sp[0]
			tags.Remove(key)
		}
	}

	return nil
}
