// Copyright 2023 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

//go:build gjson || !(amd64 && (linux || windows || darwin))
// +build gjson !amd64 !linux,!windows,!darwin

package decoder

import (
	"strings"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/tidwall/gjson"
)

func checkRequireJSON(req *protocol.Request, tagInfo TagInfo) bool {
	if !tagInfo.Required {
		return true
	}
	ct := bytesconv.B2s(req.Header.ContentType())
	if !strings.EqualFold(utils.FilterContentType(ct), consts.MIMEApplicationJSON) {
		return false
	}
	result := gjson.GetBytes(req.Body(), tagInfo.JSONName)
	if !result.Exists() {
		idx := strings.LastIndex(tagInfo.JSONName, ".")
		// There should be a superior if it is empty, it will report 'true' for required
		if idx > 0 && !gjson.GetBytes(req.Body(), tagInfo.JSONName[:idx]).Exists() {
			return true
		}
		return false
	}
	return true
}
