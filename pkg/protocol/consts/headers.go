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

package consts

const (
	HeaderAuthorization = "Authorization"
	HeaderHost          = "Host"
	HeaderReferer       = "Referer"
	HeaderContentType   = "Content-Type"
	HeaderUserAgent     = "User-Agent"
	HeaderExpect        = "Expect"
	HeaderConnection    = "Connection"
	HeaderContentLength = "Content-Length"
	HeaderCookie        = "Cookie"
	HeaderAltSvc        = "Alt-Svc"

	HeaderServer           = "Server"
	HeaderServerLower      = "server"
	HeaderSetCookie        = "Set-Cookie"
	HeaderSetCookieLower   = "set-cookie"
	HeaderTransferEncoding = "Transfer-Encoding"
	HeaderDate             = "Date"

	HeaderRange        = "Range"
	HeaderAcceptRanges = "Accept-Ranges"
	HeaderContentRange = "Content-Range"

	HeaderIfModifiedSince = "If-Modified-Since"
	HeaderLastModified    = "Last-Modified"

	// Message body information
	HeaderContentEncoding = "Content-Encoding"
	HeaderAcceptEncoding  = "Accept-Encoding"

	// Redirects
	HeaderLocation = "Location"

	// Protocol
	HTTP11 = "HTTP/1.1"
	HTTP10 = "HTTP/1.0"
	HTTP20 = "HTTP/2.0"
)
