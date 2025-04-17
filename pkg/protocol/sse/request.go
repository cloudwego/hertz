/*
 * Copyright 2025 CloudWeGo Authors
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

package sse

import (
	"bytes"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// AddAcceptMIME adds `text/event-stream` to http `Accept` header.
//
// This is NOT required as per spec:
// * User agents MAY set (`Accept`, `text/event-stream`) in request's header list.
func AddAcceptMIME(req *protocol.Request) {
	v := req.Header.Peek("Accept")
	if len(v) > 0 {
		if bytes.Contains(v, bytestr.MIMETextEventStream) {
			return
		}
		// for better compatibility, only use one Accept header value
		// append `text/event-stream` to the end of the value
		req.Header.Set("Accept", string(v)+", "+string(bytestr.MIMETextEventStream))
	} else {
		req.Header.Set("Accept", string(bytestr.MIMETextEventStream))
	}
}
