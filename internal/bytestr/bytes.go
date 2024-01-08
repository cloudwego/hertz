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

// Package bytestr defines some common bytes
package bytestr

import (
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	DefaultServerName  = []byte("hertz")
	DefaultUserAgent   = []byte("hertz")
	DefaultContentType = []byte("text/plain; charset=utf-8")
)

var (
	StrBackSlash        = []byte("\\")
	StrSlash            = []byte("/")
	StrSlashSlash       = []byte("//")
	StrSlashDotDot      = []byte("/..")
	StrSlashDotSlash    = []byte("/./")
	StrSlashDotDotSlash = []byte("/../")
	StrCRLF             = []byte("\r\n")
	StrHTTP             = []byte("http")
	StrHTTPS            = []byte("https")
	StrHTTP11           = []byte("HTTP/1.1")
	StrColon            = []byte(":")
	StrStar             = []byte("*")
	StrColonSlashSlash  = []byte("://")
	StrColonSpace       = []byte(": ")
	StrCommaSpace       = []byte(", ")
	StrAt               = []byte("@")
	StrSD               = []byte("sd")

	StrResponseContinue = []byte("HTTP/1.1 100 Continue\r\n\r\n")

	StrGet     = []byte(consts.MethodGet)
	StrHead    = []byte(consts.MethodHead)
	StrPost    = []byte(consts.MethodPost)
	StrPut     = []byte(consts.MethodPut)
	StrDelete  = []byte(consts.MethodDelete)
	StrConnect = []byte(consts.MethodConnect)
	StrOptions = []byte(consts.MethodOptions)
	StrTrace   = []byte(consts.MethodTrace)
	StrPatch   = []byte(consts.MethodPatch)

	StrExpect           = []byte(consts.HeaderExpect)
	StrConnection       = []byte(consts.HeaderConnection)
	StrContentLength    = []byte(consts.HeaderContentLength)
	StrContentType      = []byte(consts.HeaderContentType)
	StrDate             = []byte(consts.HeaderDate)
	StrHost             = []byte(consts.HeaderHost)
	StrServer           = []byte(consts.HeaderServer)
	StrTransferEncoding = []byte(consts.HeaderTransferEncoding)

	StrUserAgent          = []byte(consts.HeaderUserAgent)
	StrCookie             = []byte(consts.HeaderCookie)
	StrLocation           = []byte(consts.HeaderLocation)
	StrContentRange       = []byte(consts.HeaderContentRange)
	StrContentEncoding    = []byte(consts.HeaderContentEncoding)
	StrAcceptEncoding     = []byte(consts.HeaderAcceptEncoding)
	StrSetCookie          = []byte(consts.HeaderSetCookie)
	StrAuthorization      = []byte(consts.HeaderAuthorization)
	StrRange              = []byte(consts.HeaderRange)
	StrLastModified       = []byte(consts.HeaderLastModified)
	StrAcceptRanges       = []byte(consts.HeaderAcceptRanges)
	StrIfModifiedSince    = []byte(consts.HeaderIfModifiedSince)
	StrTE                 = []byte(consts.HeaderTE)
	StrTrailer            = []byte(consts.HeaderTrailer)
	StrMaxForwards        = []byte(consts.HeaderMaxForwards)
	StrProxyConnection    = []byte(consts.HeaderProxyConnection)
	StrProxyAuthenticate  = []byte(consts.HeaderProxyAuthenticate)
	StrProxyAuthorization = []byte(consts.HeaderProxyAuthorization)
	StrWWWAuthenticate    = []byte(consts.HeaderWWWAuthenticate)

	StrCookieExpires        = []byte("expires")
	StrCookieDomain         = []byte("domain")
	StrCookiePath           = []byte("path")
	StrCookieHTTPOnly       = []byte("HttpOnly")
	StrCookieSecure         = []byte("secure")
	StrCookieMaxAge         = []byte("max-age")
	StrCookieSameSite       = []byte("SameSite")
	StrCookieSameSiteLax    = []byte("Lax")
	StrCookieSameSiteStrict = []byte("Strict")
	StrCookieSameSiteNone   = []byte("None")
	StrCookiePartitioned    = []byte("Partitioned")

	StrClose               = []byte("close")
	StrGzip                = []byte("gzip")
	StrDeflate             = []byte("deflate")
	StrKeepAlive           = []byte("keep-alive")
	StrUpgrade             = []byte("Upgrade")
	StrChunked             = []byte("chunked")
	StrIdentity            = []byte("identity")
	Str100Continue         = []byte("100-continue")
	StrPostArgsContentType = []byte("application/x-www-form-urlencoded")
	StrMultipartFormData   = []byte("multipart/form-data")
	StrBoundary            = []byte("boundary")
	StrBytes               = []byte("bytes")
	StrTextSlash           = []byte("text/")
	StrApplicationSlash    = []byte("application/")
	StrBasicSpace          = []byte("Basic ")

	// http2
	StrClientPreface = []byte(consts.ClientPreface)
)
