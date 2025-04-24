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
	"github.com/cloudwego/hertz/pkg/app"
)

const LastEventIDHeader = "Last-Event-ID"

// GetLastEventID returns the value of the Last-Event-ID header.
func GetLastEventID(c *app.RequestContext) string {
	return string(c.GetHeader(LastEventIDHeader))
}

func sseEventType(v []byte) string {
	switch string(v) {
	case "message":
		return "message"
	}
	return string(v)
}

func hasCRLF(s string) bool {
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case '\r', '\n':
			return true
		}
	}
	return false
}
