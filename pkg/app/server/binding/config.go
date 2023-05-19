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

package binding

import (
	standardJson "encoding/json"
	"reflect"

	"github.com/cloudwego/hertz/pkg/app/server/binding/decoder"
	path1 "github.com/cloudwego/hertz/pkg/app/server/binding/path"
	hjson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// ResetJSONUnmarshaler reset the JSON Unmarshal function.
func ResetJSONUnmarshaler(fn func(data []byte, v interface{}) error) {
	hjson.Unmarshal = fn
}

// ResetStdJSONUnmarshaler uses "encoding/json" as the JSON Unmarshal function.
func ResetStdJSONUnmarshaler() {
	ResetJSONUnmarshaler(standardJson.Unmarshal)
}

// EnableDefaultTag is used to enable or disable adding default tags to a field when it has no tag, it is true by default.
// If is true, the field with no tag will be added default tags, for more automated parameter binding. But there may be additional overhead
func EnableDefaultTag(b bool) {
	decoder.EnableDefaultTag = b
}

// EnableStructFieldResolve to enable or disable the generation of a separate decoder for a struct, it is false by default.
// If is true, the 'struct' field will get a single decoder.structTypeFieldTextDecoder, and use json.Unmarshal for decode it.
func EnableStructFieldResolve(b bool) {
	decoder.EnableStructFieldResolve = b
}

// RegTypeUnmarshal registers customized type unmarshaler.
func RegTypeUnmarshal(t reflect.Type, fn func(req *protocol.Request, params path1.PathParam, text string) (reflect.Value, error)) error {
	return decoder.RegTypeUnmarshal(t, fn)
}

// MustRegTypeUnmarshal registers customized type unmarshaler. It will panic if exist error.
func MustRegTypeUnmarshal(t reflect.Type, fn func(req *protocol.Request, params path1.PathParam, text string) (reflect.Value, error)) {
	decoder.MustRegTypeUnmarshal(t, fn)
}
