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
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestFieldInfo_checkJSON(t *testing.T) {
	f := &fieldInfo{}
	f.tagInfos = []*TagInfo{{
		Tag:      "json",
		Required: true,
	}}

	req := &protocol.Request{}
	req.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationJSON))
	update_rawbody(req, []byte(`{"a": {"b":1}}`))

	// case ok
	f.jsonName = "b"
	f.jsonKeys = []any{"a", "b"}
	ok, err := f.checkJSON(req)
	assert.Assert(t, ok == true, ok)
	assert.Assert(t, err == nil, err)

	// case Content-Type not json
	req.Header.SetContentTypeBytes(nil)
	ok, err = f.checkJSON(req)
	assert.Assert(t, ok == false, ok)
	assert.DeepEqual(t, `JSON field "a.b" is required, request body is not JSON`, err.Error())
	req.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationJSON))

	// case parent exists but field not
	f.jsonName = "c"
	f.jsonKeys = []any{"a", "c"}
	ok, err = f.checkJSON(req)
	assert.Assert(t, ok == false, ok)
	assert.DeepEqual(t, `JSON field "a.c" is required, but it's not set`, err.Error())

	// case parent not exists
	f.jsonName = "c"
	f.jsonKeys = []any{"d", "c"}
	ok, err = f.checkJSON(req)
	assert.Assert(t, ok == false, ok)
	assert.Assert(t, err == nil, err)

}
