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

package protocol

import (
	"bytes"
	"sync"
	"time"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/internal/nocopy"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils"
)

const (
	// CookieSameSiteDisabled removes the SameSite flag
	CookieSameSiteDisabled CookieSameSite = iota
	// CookieSameSiteDefaultMode sets the SameSite flag
	CookieSameSiteDefaultMode
	// CookieSameSiteLaxMode sets the SameSite flag with the "Lax" parameter
	CookieSameSiteLaxMode
	// CookieSameSiteStrictMode sets the SameSite flag with the "Strict" parameter
	CookieSameSiteStrictMode
	// CookieSameSiteNoneMode sets the SameSite flag with the "None" parameter
	// see https://tools.ietf.org/html/draft-west-cookie-incrementalism-00
	// third-party cookies are phasing out, use Partitioned cookies instead
	// see https://developers.google.com/privacy-sandbox/3pcd
	CookieSameSiteNoneMode
)

var zeroTime time.Time

var (
	errNoCookies = errors.NewPublic("no cookies found")

	// CookieExpireDelete may be set on Cookie.Expire for expiring the given cookie.
	CookieExpireDelete = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

	// CookieExpireUnlimited indicates that the cookie doesn't expire.
	CookieExpireUnlimited = zeroTime
)

// CookieSameSite is an enum for the mode in which the SameSite flag should be set for the given cookie.
// See https://tools.ietf.org/html/draft-ietf-httpbis-cookie-same-site-00 for details.
type CookieSameSite int

// Cookie represents HTTP response cookie.
//
// Do not copy Cookie objects. Create new object and use CopyTo instead.
//
// Cookie instance MUST NOT be used from concurrently running goroutines.
type Cookie struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	key    []byte
	value  []byte
	expire time.Time
	maxAge int
	domain []byte
	path   []byte

	httpOnly bool
	secure   bool
	// A partitioned third-party cookie is tied to the top-level site
	// where it's initially set and cannot be accessed from elsewhere.
	partitioned bool
	sameSite    CookieSameSite

	bufKV argsKV
	buf   []byte
}

var cookiePool = &sync.Pool{
	New: func() interface{} {
		return &Cookie{}
	},
}

// AcquireCookie returns an empty Cookie object from the pool.
//
// The returned object may be returned back to the pool with ReleaseCookie.
// This allows reducing GC load.
func AcquireCookie() *Cookie {
	return cookiePool.Get().(*Cookie)
}

// ReleaseCookie returns the Cookie object acquired with AcquireCookie back
// to the pool.
//
// Do not access released Cookie object, otherwise data races may occur.
func ReleaseCookie(c *Cookie) {
	c.Reset()
	cookiePool.Put(c)
}

// SetDomain sets cookie domain.
func (c *Cookie) SetDomain(domain string) {
	c.domain = append(c.domain[:0], domain...)
}

// SetPath sets cookie path.
func (c *Cookie) SetPath(path string) {
	c.buf = append(c.buf[:0], path...)
	c.path = normalizePath(c.path, c.buf)
}

// SetPathBytes sets cookie path.
func (c *Cookie) SetPathBytes(path []byte) {
	c.buf = append(c.buf[:0], path...)
	c.path = normalizePath(c.path, c.buf)
}

// SetExpire sets cookie expiration time.
//
// Set expiration time to CookieExpireDelete for expiring (deleting)
// the cookie on the client.
//
// By default cookie lifetime is limited by browser session.
func (c *Cookie) SetExpire(expire time.Time) {
	c.expire = expire
}

// SetKey sets cookie name.
func (c *Cookie) SetKey(key string) {
	c.key = append(c.key[:0], key...)
}

// SetKeyBytes sets cookie name.
func (c *Cookie) SetKeyBytes(key []byte) {
	c.key = append(c.key[:0], key...)
}

// SetValue sets cookie value.
func (c *Cookie) SetValue(value string) {
	warnIfInvalid(bytesconv.S2b(value))
	c.value = append(c.value[:0], value...)
}

// SetValueBytes sets cookie value.
func (c *Cookie) SetValueBytes(value []byte) {
	warnIfInvalid(value)
	c.value = append(c.value[:0], value...)
}

// AppendBytes appends cookie representation to dst and returns
// the extended dst.
func (c *Cookie) AppendBytes(dst []byte) []byte {
	if len(c.key) > 0 {
		dst = append(dst, c.key...)
		dst = append(dst, '=')
	}
	dst = append(dst, c.value...)

	if c.maxAge > 0 {
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookieMaxAge...)
		dst = append(dst, '=')
		dst = bytesconv.AppendUint(dst, c.maxAge)
	} else if !c.expire.IsZero() {
		c.bufKV.value = bytesconv.AppendHTTPDate(c.bufKV.value[:0], c.expire)
		dst = appendCookiePart(dst, bytestr.StrCookieExpires, c.bufKV.value)
	}
	if len(c.domain) > 0 {
		dst = appendCookiePart(dst, bytestr.StrCookieDomain, c.domain)
	}
	if len(c.path) > 0 {
		dst = appendCookiePart(dst, bytestr.StrCookiePath, c.path)
	}
	if c.httpOnly {
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookieHTTPOnly...)
	}
	if c.secure {
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookieSecure...)
	}
	switch c.sameSite {
	case CookieSameSiteDefaultMode:
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookieSameSite...)
	case CookieSameSiteLaxMode:
		dst = appendCookiePart(dst, bytestr.StrCookieSameSite, bytestr.StrCookieSameSiteLax)
	case CookieSameSiteStrictMode:
		dst = appendCookiePart(dst, bytestr.StrCookieSameSite, bytestr.StrCookieSameSiteStrict)
	case CookieSameSiteNoneMode:
		dst = appendCookiePart(dst, bytestr.StrCookieSameSite, bytestr.StrCookieSameSiteNone)
	}

	if c.partitioned {
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookiePartitioned...)
	}

	return dst
}

func appendCookiePart(dst, key, value []byte) []byte {
	dst = append(dst, ';', ' ')
	dst = append(dst, key...)
	dst = append(dst, '=')
	return append(dst, value...)
}

func appendRequestCookieBytes(dst []byte, cookies []argsKV) []byte {
	for i, n := 0, len(cookies); i < n; i++ {
		kv := &cookies[i]
		if len(kv.key) > 0 {
			dst = append(dst, kv.key...)
			dst = append(dst, '=')
		}
		dst = append(dst, kv.value...)
		if i+1 < n {
			dst = append(dst, ';', ' ')
		}
	}
	return dst
}

// For Response we can not use the above function as response cookies
// already contain the key= in the value.
func appendResponseCookieBytes(dst []byte, cookies []argsKV) []byte {
	for i, n := 0, len(cookies); i < n; i++ {
		kv := &cookies[i]
		dst = append(dst, kv.value...)
		if i+1 < n {
			dst = append(dst, ';', ' ')
		}
	}
	return dst
}

type cookieScanner struct {
	b []byte
}

func parseRequestCookies(cookies []argsKV, src []byte) []argsKV {
	var s cookieScanner
	s.b = src
	var kv *argsKV
	cookies, kv = allocArg(cookies)
	for s.next(kv) {
		if len(kv.key) > 0 || len(kv.value) > 0 {
			cookies, kv = allocArg(cookies)
		}
	}
	return releaseArg(cookies)
}

func (s *cookieScanner) next(kv *argsKV) bool {
	b := s.b
	if len(b) == 0 {
		return false
	}

	isKey := true
	k := 0
	for i, c := range b {
		switch c {
		case '=':
			if isKey {
				isKey = false
				kv.key = decodeCookieArg(kv.key, b[:i], false)
				k = i + 1
			}
		case ';':
			if isKey {
				kv.key = kv.key[:0]
			}
			kv.value = decodeCookieArg(kv.value, b[k:i], true)
			s.b = b[i+1:]
			return true
		}
	}

	if isKey {
		kv.key = kv.key[:0]
	}
	kv.value = decodeCookieArg(kv.value, b[k:], true)
	s.b = b[len(b):]
	return true
}

// Key returns cookie name.
//
// The returned value is valid until the next Cookie modification method call.
func (c *Cookie) Key() []byte {
	return c.key
}

// Cookie returns cookie representation.
//
// The returned value is valid until the next call to Cookie methods.
func (c *Cookie) Cookie() []byte {
	c.buf = c.AppendBytes(c.buf[:0])
	return c.buf
}

// Reset clears the cookie.
func (c *Cookie) Reset() {
	c.key = c.key[:0]
	c.value = c.value[:0]
	c.expire = zeroTime
	c.maxAge = 0
	c.domain = c.domain[:0]
	c.path = c.path[:0]
	c.httpOnly = false
	c.secure = false
	c.sameSite = CookieSameSiteDisabled
	c.partitioned = false
}

// Value returns cookie value.
//
// The returned value is valid until the next Cookie modification method call.
func (c *Cookie) Value() []byte {
	return c.value
}

// Parse parses Set-Cookie header.
func (c *Cookie) Parse(src string) error {
	c.buf = append(c.buf[:0], src...)
	return c.ParseBytes(c.buf)
}

// ParseBytes parses Set-Cookie header.
func (c *Cookie) ParseBytes(src []byte) error {
	c.Reset()

	var s cookieScanner
	s.b = src

	kv := &c.bufKV
	if !s.next(kv) {
		return errNoCookies
	}

	c.key = append(c.key[:0], kv.key...)
	c.value = append(c.value[:0], kv.value...)

	for s.next(kv) {
		if len(kv.key) != 0 {
			// Case-insensitive switch on first char
			switch kv.key[0] | 0x20 {
			case 'm':
				if utils.CaseInsensitiveCompare(bytestr.StrCookieMaxAge, kv.key) {
					maxAge, err := bytesconv.ParseUint(kv.value)
					if err != nil {
						return err
					}
					c.maxAge = maxAge
				}

			case 'e': // "expires"
				if utils.CaseInsensitiveCompare(bytestr.StrCookieExpires, kv.key) {
					v := bytesconv.B2s(kv.value)
					// Try the same two formats as net/http
					// See: https://github.com/golang/go/blob/00379be17e63a5b75b3237819392d2dc3b313a27/src/net/http/cookie.go#L133-L135
					exptime, err := time.ParseInLocation(time.RFC1123, v, time.UTC)
					if err != nil {
						exptime, err = time.Parse("Mon, 02-Jan-2006 15:04:05 MST", v)
						if err != nil {
							return err
						}
					}
					c.expire = exptime
				}

			case 'd': // "domain"
				if utils.CaseInsensitiveCompare(bytestr.StrCookieDomain, kv.key) {
					c.domain = append(c.domain[:0], kv.value...)
				}

			case 'p': // "path"
				if utils.CaseInsensitiveCompare(bytestr.StrCookiePath, kv.key) {
					c.path = append(c.path[:0], kv.value...)
				}

			case 's': // "samesite"
				if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSite, kv.key) {
					// Case-insensitive switch on first char
					switch kv.value[0] | 0x20 {
					case 'l': // "lax"
						if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSiteLax, kv.value) {
							c.sameSite = CookieSameSiteLaxMode
						}
					case 's': // "strict"
						if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSiteStrict, kv.value) {
							c.sameSite = CookieSameSiteStrictMode
						}
					case 'n': // "none"
						if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSiteNone, kv.value) {
							c.sameSite = CookieSameSiteNoneMode
						}
					}
				}
			}
		} else if len(kv.value) != 0 {
			// Case-insensitive switch on first char
			switch kv.value[0] | 0x20 {
			case 'h': // "httponly"
				if utils.CaseInsensitiveCompare(bytestr.StrCookieHTTPOnly, kv.value) {
					c.httpOnly = true
				}

			case 's': // "secure"
				if utils.CaseInsensitiveCompare(bytestr.StrCookieSecure, kv.value) {
					c.secure = true
				} else if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSite, kv.value) {
					c.sameSite = CookieSameSiteDefaultMode
				}
			case 'p': // "partitioned"
				if utils.CaseInsensitiveCompare(bytestr.StrCookiePartitioned, kv.value) {
					c.partitioned = true
				}
			}
		} // else empty or no match
	}

	return nil
}

// MaxAge returns the seconds until the cookie is meant to expire or 0
// if no max age.
func (c *Cookie) MaxAge() int {
	return c.maxAge
}

// SetMaxAge sets cookie expiration time based on seconds. This takes precedence
// over any absolute expiry set on the cookie
//
// Set max age to 0 to unset
func (c *Cookie) SetMaxAge(seconds int) {
	c.maxAge = seconds
}

// Expire returns cookie expiration time.
//
// CookieExpireUnlimited is returned if cookie doesn't expire
func (c *Cookie) Expire() time.Time {
	expire := c.expire
	if expire.IsZero() {
		expire = CookieExpireUnlimited
	}
	return expire
}

// Domain returns cookie domain.
//
// The returned domain is valid until the next Cookie modification method call.
func (c *Cookie) Domain() []byte {
	return c.domain
}

// Path returns cookie path.
func (c *Cookie) Path() []byte {
	return c.path
}

// Secure returns true if the cookie is secure.
func (c *Cookie) Secure() bool {
	return c.secure
}

// SetSecure sets cookie's secure flag to the given value.
func (c *Cookie) SetSecure(secure bool) {
	c.secure = secure
}

// SameSite returns the SameSite mode.
func (c *Cookie) SameSite() CookieSameSite {
	return c.sameSite
}

// Partitioned returns if cookie is partitioned.
func (c *Cookie) Partitioned() bool {
	return c.partitioned
}

// SetSameSite sets the cookie's SameSite flag to the given value.
// set value CookieSameSiteNoneMode will set Secure to true also to avoid browser rejection
func (c *Cookie) SetSameSite(mode CookieSameSite) {
	c.sameSite = mode
	if mode == CookieSameSiteNoneMode {
		c.SetSecure(true)
	}
}

// HTTPOnly returns true if the cookie is http only.
func (c *Cookie) HTTPOnly() bool {
	return c.httpOnly
}

// SetHTTPOnly sets cookie's httpOnly flag to the given value.
func (c *Cookie) SetHTTPOnly(httpOnly bool) {
	c.httpOnly = httpOnly
}

// SetPartitioned sets cookie as partitioned. Setting Partitioned to true will also set Secure.
func (c *Cookie) SetPartitioned(partitioned bool) {
	c.partitioned = partitioned
	if partitioned {
		c.SetSecure(true)
	}
}

// String returns cookie representation.
func (c *Cookie) String() string {
	return string(c.Cookie())
}

func decodeCookieArg(dst, src []byte, skipQuotes bool) []byte {
	for len(src) > 0 && src[0] == ' ' {
		src = src[1:]
	}
	for len(src) > 0 && src[len(src)-1] == ' ' {
		src = src[:len(src)-1]
	}
	if skipQuotes {
		if len(src) > 1 && src[0] == '"' && src[len(src)-1] == '"' {
			src = src[1 : len(src)-1]
		}
	}
	return append(dst[:0], src...)
}

func getCookieKey(dst, src []byte) []byte {
	n := bytes.IndexByte(src, '=')
	if n >= 0 {
		src = src[:n]
	}
	return decodeCookieArg(dst, src, false)
}

func warnIfInvalid(value []byte) bool {
	for i := range value {
		if bytesconv.ValidCookieValueTable[value[i]] == 0 {
			hlog.SystemLogger().Warnf("Invalid byte %q in Cookie.Value, "+
				"it may cause compatibility problems with user agents", value[i])
			return false
		}
	}
	return true
}
