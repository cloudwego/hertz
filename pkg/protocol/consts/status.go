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
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik Dubbelboer, FastHTTP Authors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package consts

import (
	"fmt"
	"sync/atomic"
)

const (
	statusMessageMin = 100
	statusMessageMax = 511
)

// HTTP status codes were stolen from net/http.
const (
	StatusContinue           = 100 // RFC 7231, 6.2.1
	StatusSwitchingProtocols = 101 // RFC 7231, 6.2.2
	StatusProcessing         = 102 // RFC 2518, 10.1

	StatusOK                   = 200 // RFC 7231, 6.3.1
	StatusCreated              = 201 // RFC 7231, 6.3.2
	StatusAccepted             = 202 // RFC 7231, 6.3.3
	StatusNonAuthoritativeInfo = 203 // RFC 7231, 6.3.4
	StatusNoContent            = 204 // RFC 7231, 6.3.5
	StatusResetContent         = 205 // RFC 7231, 6.3.6
	StatusPartialContent       = 206 // RFC 7233, 4.1
	StatusMultiStatus          = 207 // RFC 4918, 11.1
	StatusAlreadyReported      = 208 // RFC 5842, 7.1
	StatusIMUsed               = 226 // RFC 3229, 10.4.1

	StatusMultipleChoices   = 300 // RFC 7231, 6.4.1
	StatusMovedPermanently  = 301 // RFC 7231, 6.4.2
	StatusFound             = 302 // RFC 7231, 6.4.3
	StatusSeeOther          = 303 // RFC 7231, 6.4.4
	StatusNotModified       = 304 // RFC 7232, 4.1
	StatusUseProxy          = 305 // RFC 7231, 6.4.5
	_                       = 306 // RFC 7231, 6.4.6 (Unused)
	StatusTemporaryRedirect = 307 // RFC 7231, 6.4.7
	StatusPermanentRedirect = 308 // RFC 7538, 3

	StatusBadRequest                   = 400 // RFC 7231, 6.5.1
	StatusUnauthorized                 = 401 // RFC 7235, 3.1
	StatusPaymentRequired              = 402 // RFC 7231, 6.5.2
	StatusForbidden                    = 403 // RFC 7231, 6.5.3
	StatusNotFound                     = 404 // RFC 7231, 6.5.4
	StatusMethodNotAllowed             = 405 // RFC 7231, 6.5.5
	StatusNotAcceptable                = 406 // RFC 7231, 6.5.6
	StatusProxyAuthRequired            = 407 // RFC 7235, 3.2
	StatusRequestTimeout               = 408 // RFC 7231, 6.5.7
	StatusConflict                     = 409 // RFC 7231, 6.5.8
	StatusGone                         = 410 // RFC 7231, 6.5.9
	StatusLengthRequired               = 411 // RFC 7231, 6.5.10
	StatusPreconditionFailed           = 412 // RFC 7232, 4.2
	StatusRequestEntityTooLarge        = 413 // RFC 7231, 6.5.11
	StatusRequestURITooLong            = 414 // RFC 7231, 6.5.12
	StatusUnsupportedMediaType         = 415 // RFC 7231, 6.5.13
	StatusRequestedRangeNotSatisfiable = 416 // RFC 7233, 4.4
	StatusExpectationFailed            = 417 // RFC 7231, 6.5.14
	StatusTeapot                       = 418 // RFC 7168, 2.3.3
	StatusUnprocessableEntity          = 422 // RFC 4918, 11.2
	StatusLocked                       = 423 // RFC 4918, 11.3
	StatusFailedDependency             = 424 // RFC 4918, 11.4
	StatusUpgradeRequired              = 426 // RFC 7231, 6.5.15
	StatusPreconditionRequired         = 428 // RFC 6585, 3
	StatusTooManyRequests              = 429 // RFC 6585, 4
	StatusRequestHeaderFieldsTooLarge  = 431 // RFC 6585, 5
	StatusUnavailableForLegalReasons   = 451 // RFC 7725, 3

	StatusInternalServerError           = 500 // RFC 7231, 6.6.1
	StatusNotImplemented                = 501 // RFC 7231, 6.6.2
	StatusBadGateway                    = 502 // RFC 7231, 6.6.3
	StatusServiceUnavailable            = 503 // RFC 7231, 6.6.4
	StatusGatewayTimeout                = 504 // RFC 7231, 6.6.5
	StatusHTTPVersionNotSupported       = 505 // RFC 7231, 6.6.6
	StatusVariantAlsoNegotiates         = 506 // RFC 2295, 8.1
	StatusInsufficientStorage           = 507 // RFC 4918, 11.5
	StatusLoopDetected                  = 508 // RFC 5842, 7.2
	StatusNotExtended                   = 510 // RFC 2774, 7
	StatusNetworkAuthenticationRequired = 511 // RFC 6585, 6
)

var (
	statusLines atomic.Value

	statusMessages = map[int]string{
		StatusContinue:           "Continue",
		StatusSwitchingProtocols: "Switching Protocols",
		StatusProcessing:         "Processing",

		StatusOK:                   "OK",
		StatusCreated:              "Created",
		StatusAccepted:             "Accepted",
		StatusNonAuthoritativeInfo: "Non-Authoritative Information",
		StatusNoContent:            "No Content",
		StatusResetContent:         "Reset Content",
		StatusPartialContent:       "Partial Content",
		StatusMultiStatus:          "Multi-Status",
		StatusAlreadyReported:      "Already Reported",
		StatusIMUsed:               "IM Used",

		StatusMultipleChoices:   "Multiple Choices",
		StatusMovedPermanently:  "Moved Permanently",
		StatusFound:             "Found",
		StatusSeeOther:          "See Other",
		StatusNotModified:       "Not Modified",
		StatusUseProxy:          "Use Proxy",
		StatusTemporaryRedirect: "Temporary Redirect",
		StatusPermanentRedirect: "Permanent Redirect",

		StatusBadRequest:                   "Bad Request",
		StatusUnauthorized:                 "Unauthorized",
		StatusPaymentRequired:              "Payment Required",
		StatusForbidden:                    "Forbidden",
		StatusNotFound:                     "Not Found",
		StatusMethodNotAllowed:             "Method Not Allowed",
		StatusNotAcceptable:                "Not Acceptable",
		StatusProxyAuthRequired:            "Proxy Authentication Required",
		StatusRequestTimeout:               "Request Timeout",
		StatusConflict:                     "Conflict",
		StatusGone:                         "Gone",
		StatusLengthRequired:               "Length Required",
		StatusPreconditionFailed:           "Precondition Failed",
		StatusRequestEntityTooLarge:        "Request Entity Too Large",
		StatusRequestURITooLong:            "Request URI Too Long",
		StatusUnsupportedMediaType:         "Unsupported Media Type",
		StatusRequestedRangeNotSatisfiable: "Requested Range Not Satisfiable",
		StatusExpectationFailed:            "Expectation Failed",
		StatusTeapot:                       "I'm a teapot",
		StatusUnprocessableEntity:          "Unprocessable Entity",
		StatusLocked:                       "Locked",
		StatusFailedDependency:             "Failed Dependency",
		StatusUpgradeRequired:              "Upgrade Required",
		StatusPreconditionRequired:         "Precondition Required",
		StatusTooManyRequests:              "Too Many Requests",
		StatusRequestHeaderFieldsTooLarge:  "Request Header Fields Too Large",
		StatusUnavailableForLegalReasons:   "Unavailable For Legal Reasons",

		StatusInternalServerError:           "Internal Server Error",
		StatusNotImplemented:                "Not Implemented",
		StatusBadGateway:                    "Bad Gateway",
		StatusServiceUnavailable:            "Service Unavailable",
		StatusGatewayTimeout:                "Gateway Timeout",
		StatusHTTPVersionNotSupported:       "HTTP Version Not Supported",
		StatusVariantAlsoNegotiates:         "Variant Also Negotiates",
		StatusInsufficientStorage:           "Insufficient Storage",
		StatusLoopDetected:                  "Loop Detected",
		StatusNotExtended:                   "Not Extended",
		StatusNetworkAuthenticationRequired: "Network Authentication Required",
	}
)

// StatusMessage returns HTTP status message for the given status code.
func StatusMessage(statusCode int) string {
	if statusCode < statusMessageMin || statusCode > statusMessageMax {
		return "Unknown Status Code"
	}

	s := statusMessages[statusCode]
	if s == "" {
		s = "Unknown Status Code"
	}
	return s
}

func init() {
	statusLines.Store(make(map[int][]byte))
}

func StatusLine(statusCode int) []byte {
	m := statusLines.Load().(map[int][]byte)
	h := m[statusCode]
	if h != nil {
		return h
	}

	statusText := StatusMessage(statusCode)

	h = []byte(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, statusText))
	newM := make(map[int][]byte, len(m)+1)
	for k, v := range m {
		newM[k] = v
	}
	newM[statusCode] = h
	statusLines.Store(newM)
	return h
}
