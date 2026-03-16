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

func TestOneofFieldCustomBodyTag(t *testing.T) {
	in, err := ioutil.ReadFile("./test_data/oneof_tag_test.out")
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
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		_, err = generateFile(gen, f, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	resp := gen.Response()
	var content string
	for _, f := range resp.File {
		if strings.HasSuffix(f.GetName(), "oneof_tag.pb.go") {
			content = f.GetContent()
			break
		}
	}
	if content == "" {
		t.Fatal("generated oneof_tag.pb.go not found")
	}

	wants := []string{
		"Question *QuestionnaireQuestion `protobuf:\"bytes,2,opt,name=question,proto3,oneof\" form:\"content\" json:\"content,omitempty\"`",
		"System *QuestionnaireSystem `protobuf:\"bytes,3,opt,name=system,proto3,oneof\" form:\"content\" json:\"content,omitempty\"`",
	}
	for _, want := range wants {
		if !strings.Contains(content, want) {
			t.Fatalf("expected oneof wrapper field tag in generated file: %s", want)
		}
	}
}
