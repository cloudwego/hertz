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

package protobuf

import (
	"io/ioutil"
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestTagGenerate(t *testing.T) {
	type TagStruct struct {
		Annotation   string
		GeneratedTag string
		ActualTag    string
	}

	tagList := []TagStruct{
		{
			Annotation:   "query",
			GeneratedTag: "protobuf:\"bytes,1,opt,name=QueryTag\" json:\"QueryTag,omitempty\" query:\"query\"",
		},
		{
			Annotation:   "raw_body",
			GeneratedTag: "protobuf:\"bytes,2,opt,name=RawBodyTag\" json:\"RawBodyTag,omitempty\" raw_body:\"raw_body\"",
		},
		{
			Annotation:   "path",
			GeneratedTag: "protobuf:\"bytes,3,opt,name=PathTag\" json:\"PathTag,omitempty\" path:\"path\"",
		},
		{
			Annotation:   "form",
			GeneratedTag: "protobuf:\"bytes,4,opt,name=FormTag\" json:\"FormTag,omitempty\" form:\"form\"",
		},
		{
			Annotation:   "cookie",
			GeneratedTag: "protobuf:\"bytes,5,opt,name=CookieTag\" json:\"CookieTag,omitempty\" cookie:\"cookie\"",
		},
		{
			Annotation:   "header",
			GeneratedTag: "protobuf:\"bytes,6,opt,name=HeaderTag\" json:\"HeaderTag,omitempty\" header:\"header\"",
		},
		{
			Annotation:   "body",
			GeneratedTag: "bytes,7,opt,name=BodyTag\" form:\"body\" json:\"body,omitempty\"",
		},
		{
			Annotation:   "go.tag",
			GeneratedTag: "bytes,8,opt,name=GoTag\" json:\"json\" form:\"form\" goTag:\"tag\" header:\"header\" query:\"query\"",
		},
		{
			Annotation:   "vd",
			GeneratedTag: "bytes,9,opt,name=VdTag\" json:\"VdTag,omitempty\" form:\"VdTag\" query:\"VdTag\" vd:\"$!='?'\"",
		},
		{
			Annotation:   "non",
			GeneratedTag: "bytes,10,opt,name=DefaultTag\" json:\"DefaultTag,omitempty\" form:\"DefaultTag\" query:\"DefaultTag\"",
		},
		{
			Annotation:   "query required",
			GeneratedTag: "bytes,11,req,name=ReqQuery\" json:\"ReqQuery,required\" query:\"query,required\"",
		},
		{
			Annotation:   "query optional",
			GeneratedTag: "bytes,12,opt,name=OptQuery\" json:\"OptQuery,omitempty\" query:\"query\"",
		},
		{
			Annotation:   "body required",
			GeneratedTag: "protobuf:\"bytes,13,req,name=ReqBody\" form:\"body,required\" json:\"body,required\"",
		},
		{
			Annotation:   "body optional",
			GeneratedTag: "protobuf:\"bytes,14,opt,name=OptBody\" form:\"body\" json:\"body,omitempty\"",
		},
		{
			Annotation:   "go.tag required",
			GeneratedTag: "protobuf:\"bytes,15,req,name=ReqGoTag\" query:\"ReqGoTag,required\" form:\"ReqGoTag,required\" json:\"json\"",
		},
		{
			Annotation:   "go.tag optional",
			GeneratedTag: "bytes,16,opt,name=OptGoTag\" query:\"OptGoTag\" form:\"OptGoTag\" json:\"json\"",
		},
		{
			Annotation:   "go tag cover query",
			GeneratedTag: "bytes,17,req,name=QueryGoTag\" json:\"QueryGoTag,required\" query:\"queryTag\"",
		},
	}

	in, err := ioutil.ReadFile("./test_data/protobuf_tag_test.out")
	if err != nil {
		t.Fatal(err)
	}

	req := &pluginpb.CodeGeneratorRequest{}
	err = proto.Unmarshal(in, req)
	if err != nil {
		t.Fatalf("unmarshal stdin request error: %v", err)
	}

	opts := protogen.Options{}
	gen, err := opts.New(req)

	for _, f := range gen.Files {
		if f.Proto.GetName() == "test_tag.proto" {
			fileInfo := newFileInfo(f)
			for _, message := range fileInfo.allMessages {
				for idx, field := range message.Fields {
					tags := structTags{
						{"protobuf", fieldProtobufTagValue(field)},
					}
					err = injectTagsToStructTags(field.Desc, &tags, true)
					if err != nil {
						t.Fatal(err)
					}
					var actualTag string
					for i, tag := range tags {
						if i == 0 {
							actualTag = tag[0] + ":" + "\"" + tag[1] + "\""
						} else {
							actualTag = actualTag + " " + tag[0] + ":" + "\"" + tag[1] + "\""
						}
					}
					tagList[idx].ActualTag = actualTag

				}
			}
		}
	}

	for i := range tagList {
		if !strings.Contains(tagList[i].ActualTag, tagList[i].GeneratedTag) {
			t.Fatalf("expected tag: '%s', but autual tag: '%s'", tagList[i].GeneratedTag, tagList[i].ActualTag)
		}
	}
}
