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

const LastEventIDHeader = "Last-Event-ID"

// GetLastEventID returns the value of the Last-Event-ID header.
func GetLastEventID(req *protocol.Request) string {
	return string(req.Header.Peek(LastEventIDHeader))
}

// SetLastEventID sets the Last-Event-ID header.
func SetLastEventID(req *protocol.Request, id string) {
	req.Header.Set(LastEventIDHeader, id)
}

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

// https://html.spec.whatwg.org/multipage/server-sent-events.html#parsing-an-event-stream
// end-of-line   = ( cr lf / cr / lf )
func scanEOL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	i := bytes.IndexByte(data, '\r')
	j := bytes.IndexByte(data, '\n')
	if i >= 0 {
		if i+1 == j { // \r\n
			return i + 2, data[0:i], nil
		}
		if j >= 0 { // choose the nearer \r or \n as EOL
			if i < j {
				return i + 1, data[0:i], nil // \r
			}
			return j + 1, data[0:j], nil // \n
		}
		// if ends with '\r', we need to check the next char is NOT '\n' as per spec
		// this may cause unexpected blocks on reading more data.
		if i < len(data)-1 || atEOF {
			return i + 1, data[0:i], nil
		}
	} else if j >= 0 {
		return j + 1, data[0:j], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil // more data
}
