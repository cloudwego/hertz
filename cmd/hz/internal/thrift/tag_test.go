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
	"io/ioutil"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/cmd/hz/internal/config"
	"github.com/cloudwego/thriftgo/plugin"
)

func TestInsertTag(t *testing.T) {
	data, err := ioutil.ReadFile("./test_data/thrift_tag_test.out")
	if err != nil {
		t.Fatal(err)
	}
	req, err := plugin.UnmarshalRequest(data)
	if err != nil {
		t.Fatal(err)
	}

	plu := new(Plugin)
	plu.req = req
	plu.args = new(config.Argument)

	type TagStruct struct {
		Annotation   string
		GeneratedTag string
		ActualTag    string
	}

	tagList := []TagStruct{
		{
			Annotation:   "query",
			GeneratedTag: "json:\"DefaultQueryTag\" query:\"query\"",
		},
		{
			Annotation:   "raw_body",
			GeneratedTag: "json:\"RawBodyTag\" raw_body:\"raw_body\"",
		},
		{
			Annotation:   "path",
			GeneratedTag: "json:\"PathTag\" path:\"path\"",
		},
		{
			Annotation:   "form",
			GeneratedTag: "form:\"form\" json:\"FormTag\"",
		},
		{
			Annotation:   "cookie",
			GeneratedTag: "cookie:\"cookie\" json:\"CookieTag\"",
		},
		{
			Annotation:   "header",
			GeneratedTag: "header:\"header\" json:\"HeaderTag\"",
		},
		{
			Annotation:   "body",
			GeneratedTag: "form:\"body\" json:\"body\"",
		},
		{
			Annotation:   "go.tag",
			GeneratedTag: "",
		},
		{
			Annotation:   "vd",
			GeneratedTag: "form:\"VdTag\" json:\"VdTag\" query:\"VdTag\" vd:\"$!='?'\"",
		},
		{
			Annotation:   "non",
			GeneratedTag: "form:\"DefaultTag\" json:\"DefaultTag\" query:\"DefaultTag\"",
		},
		{
			Annotation:   "query required",
			GeneratedTag: "json:\"ReqQuery,required\" query:\"query,required\"",
		},
		{
			Annotation:   "query optional",
			GeneratedTag: "json:\"OptQuery,omitempty\" query:\"query\"",
		},
		{
			Annotation:   "body required",
			GeneratedTag: "form:\"body,required\" json:\"body,required\"",
		},
		{
			Annotation:   "body optional",
			GeneratedTag: "form:\"body\" json:\"body,omitempty\"",
		},
		{
			Annotation:   "go.tag required",
			GeneratedTag: "form:\"ReqGoTag,required\" query:\"ReqGoTag,required\"",
		},
		{
			Annotation:   "go.tag optional",
			GeneratedTag: "form:\"OptGoTag\" query:\"OptGoTag\"",
		},
		{
			Annotation:   "go tag cover query",
			GeneratedTag: "form:\"QueryGoTag,required\" json:\"QueryGoTag,required\"",
		},
	}

	tags, err := plu.InsertTag()
	if err != nil {
		t.Fatal(err)
	}
	for i, tag := range tags {
		tagList[i].ActualTag = tag.Content
		if !strings.Contains(tagList[i].ActualTag, tagList[i].GeneratedTag) {
			t.Fatalf("expected tag: '%s', but autual tag: '%s'", tagList[i].GeneratedTag, tagList[i].ActualTag)
		}
	}
}
