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
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestPlugin_Handle(t *testing.T) {
	in, err := ioutil.ReadFile("../testdata/request_protoc.out")
	if err != nil {
		t.Fatal(err)
	}

	req := &pluginpb.CodeGeneratorRequest{}
	err = proto.Unmarshal(in, req)
	if err != nil {
		t.Fatalf("unmarshal stdin request error: %v", err)
	}

	// prepare args
	plu := &Plugin{}
	plu.setLogger()
	args, _ := plu.parseArgs(*req.Parameter)

	plu.Handle(req, args)
	plu.recvLogger()
}
