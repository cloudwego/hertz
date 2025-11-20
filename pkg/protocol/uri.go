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
	"path/filepath"
	"sync"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/internal/nocopy"
)

// AcquireURI returns an empty URI instance from the pool.
//
// Release the URI with ReleaseURI after the URI is no longer needed.
// This allows reducing GC load.
func AcquireURI() *URI {
	return uriPool.Get().(*URI)
}

// ReleaseURI releases the URI acquired via AcquireURI.
//
// The released URI mustn't be used after releasing it, otherwise data races
// may occur.
func ReleaseURI(u *URI) {
	u.Reset()
	uriPool.Put(u)
}

var uriPool = &sync.Pool{
	New: func() interface{} {
		return &URI{}
	},
}

type URI struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	pathOriginal []byte
	scheme       []byte
	path         []byte
	queryString  []byte
	hash         []byte
	host         []byte

	queryArgs       Args
	parsedQueryArgs bool

	DisablePathNormalizing bool

	fullURI    []byte
	requestURI []byte

	username []byte
	password []byte
}

type argsKV struct {
	key     []byte
	value   []byte
	noValue bool
}

func (kv *argsKV) GetKey() []byte {
	return kv.key
}

func (kv *argsKV) GetValue() []byte {
	return kv.value
}

// CopyTo copies uri contents to dst.
func (u *URI) CopyTo(dst *URI) {
	dst.Reset()
	dst.pathOriginal = append(dst.pathOriginal[:0], u.pathOriginal...)
	dst.scheme = append(dst.scheme[:0], u.scheme...)
	dst.path = append(dst.path[:0], u.path...)
	dst.queryString = append(dst.queryString[:0], u.queryString...)
	dst.hash = append(dst.hash[:0], u.hash...)
	dst.host = append(dst.host[:0], u.host...)
	dst.username = append(dst.username[:0], u.username...)
	dst.password = append(dst.password[:0], u.password...)

	u.queryArgs.CopyTo(&dst.queryArgs)
	dst.parsedQueryArgs = u.parsedQueryArgs
	dst.DisablePathNormalizing = u.DisablePathNormalizing

	// fullURI and requestURI shouldn't be copied, since they are created
	// from scratch on each FullURI() and RequestURI() call.
}

// QueryArgs returns query args.
func (u *URI) QueryArgs() *Args {
	u.parseQueryArgs()
	return &u.queryArgs
}

func (u *URI) parseQueryArgs() {
	if u.parsedQueryArgs {
		return
	}
	u.queryArgs.ParseBytes(u.queryString)
	u.parsedQueryArgs = true
}

// Hash returns URI hash, i.e. qwe of http://aaa.com/foo/bar?baz=123#qwe .
//
// The returned value is valid until the next URI method call.
func (u *URI) Hash() []byte {
	return u.hash
}

// SetHash sets URI hash.
func (u *URI) SetHash(hash string) {
	u.hash = append(u.hash[:0], hash...)
}

// SetHashBytes sets URI hash.
func (u *URI) SetHashBytes(hash []byte) {
	u.hash = append(u.hash[:0], hash...)
}

// Username returns URI username
func (u *URI) Username() []byte {
	return u.username
}

// SetUsername sets URI username.
func (u *URI) SetUsername(username string) {
	u.username = append(u.username[:0], username...)
}

// SetUsernameBytes sets URI username.
func (u *URI) SetUsernameBytes(username []byte) {
	u.username = append(u.username[:0], username...)
}

// Password returns URI password
func (u *URI) Password() []byte {
	return u.password
}

// SetPassword sets URI password.
func (u *URI) SetPassword(password string) {
	u.password = append(u.password[:0], password...)
}

// SetPasswordBytes sets URI password.
func (u *URI) SetPasswordBytes(password []byte) {
	u.password = append(u.password[:0], password...)
}

// QueryString returns URI query string,
// i.e. baz=123 of http://aaa.com/foo/bar?baz=123#qwe .
//
// The returned value is valid until the next URI method call.
func (u *URI) QueryString() []byte {
	return u.queryString
}

// SetQueryString sets URI query string.
func (u *URI) SetQueryString(queryString string) {
	u.queryString = append(u.queryString[:0], queryString...)
	u.parsedQueryArgs = false
}

// SetQueryStringBytes sets URI query string.
func (u *URI) SetQueryStringBytes(queryString []byte) {
	u.queryString = append(u.queryString[:0], queryString...)
	u.parsedQueryArgs = false
}

// Path returns URI path, i.e. /foo/bar of http://aaa.com/foo/bar?baz=123#qwe .
//
// The returned path is always urldecoded and normalized,
// i.e. '//f%20obar/baz/../zzz' becomes '/f obar/zzz'.
//
// The returned value is valid until the next URI method call.
func (u *URI) Path() []byte {
	path := u.path
	if len(path) == 0 {
		path = bytestr.StrSlash
	}
	return path
}

// SetPath sets URI path.
func (u *URI) SetPath(path string) {
	u.pathOriginal = append(u.pathOriginal[:0], path...)
	u.path = normalizePath(u.path, u.pathOriginal)
}

// String returns full uri.
func (u *URI) String() string {
	return string(u.FullURI())
}

// SetPathBytes sets URI path.
func (u *URI) SetPathBytes(path []byte) {
	u.pathOriginal = append(u.pathOriginal[:0], path...)
	u.path = normalizePath(u.path, u.pathOriginal)
}

// PathOriginal returns the original path from requestURI passed to URI.Parse().
//
// The returned value is valid until the next URI method call.
func (u *URI) PathOriginal() []byte {
	return u.pathOriginal
}

// Scheme returns URI scheme, i.e. http of http://aaa.com/foo/bar?baz=123#qwe .
//
// Returned scheme is always lowercased.
//
// The returned value is valid until the next URI method call.
func (u *URI) Scheme() []byte {
	scheme := u.scheme
	if len(scheme) == 0 {
		scheme = bytestr.StrHTTP
	}
	return scheme
}

// SetScheme sets URI scheme, i.e. http, https, ftp, etc.
func (u *URI) SetScheme(scheme string) {
	u.scheme = append(u.scheme[:0], scheme...)
	bytesconv.LowercaseBytes(u.scheme)
}

// SetSchemeBytes sets URI scheme, i.e. http, https, ftp, etc.
func (u *URI) SetSchemeBytes(scheme []byte) {
	u.scheme = append(u.scheme[:0], scheme...)
	bytesconv.LowercaseBytes(u.scheme)
}

// Reset clears uri.
func (u *URI) Reset() {
	u.pathOriginal = u.pathOriginal[:0]
	u.scheme = u.scheme[:0]
	u.path = u.path[:0]
	u.queryString = u.queryString[:0]
	u.hash = u.hash[:0]
	u.username = u.username[:0]
	u.password = u.password[:0]

	u.host = u.host[:0]
	u.queryArgs.Reset()
	u.parsedQueryArgs = false
	u.DisablePathNormalizing = false

	// There is no need in u.fullURI = u.fullURI[:0], since full uri
	// is calculated on each call to FullURI().

	// There is no need in u.requestURI = u.requestURI[:0], since requestURI
	// is calculated on each call to RequestURI().
}

// Host returns host part, i.e. aaa.com of http://aaa.com/foo/bar?baz=123#qwe .
//
// Host is always lowercased.
func (u *URI) Host() []byte {
	return u.host
}

// SetHost sets host for the uri.
func (u *URI) SetHost(host string) {
	u.host = append(u.host[:0], host...)
	bytesconv.LowercaseBytes(u.host)
}

// SetHostBytes sets host for the uri.
func (u *URI) SetHostBytes(host []byte) {
	u.host = append(u.host[:0], host...)
	bytesconv.LowercaseBytes(u.host)
}

// LastPathSegment returns the last part of uri path after '/'.
//
// Examples:
//
//   - For /foo/bar/baz.html path returns baz.html.
//   - For /foo/bar/ returns empty byte slice.
//   - For /foobar.js returns foobar.js.
func (u *URI) LastPathSegment() []byte {
	path := u.Path()
	n := bytes.LastIndexByte(path, '/')
	if n < 0 {
		return path
	}
	return path[n+1:]
}

// Update updates uri.
//
// The following newURI types are accepted:
//
//   - Absolute, i.e. http://foobar.com/aaa/bb?cc . In this case the original
//     uri is replaced by newURI.
//   - Absolute without scheme, i.e. //foobar.com/aaa/bb?cc. In this case
//     the original scheme is preserved.
//   - Missing host, i.e. /aaa/bb?cc . In this case only RequestURI part
//     of the original uri is replaced.
//   - Relative path, i.e.  xx?yy=abc . In this case the original RequestURI
//     is updated according to the new relative path.
func (u *URI) Update(newURI string) {
	u.UpdateBytes(bytesconv.S2b(newURI))
}

// UpdateBytes updates uri.
//
// The following newURI types are accepted:
//
//   - Absolute, i.e. http://foobar.com/aaa/bb?cc . In this case the original
//     uri is replaced by newURI.
//   - Absolute without scheme, i.e. //foobar.com/aaa/bb?cc. In this case
//     the original scheme is preserved.
//   - Missing host, i.e. /aaa/bb?cc . In this case only RequestURI part
//     of the original uri is replaced.
//   - Relative path, i.e.  xx?yy=abc . In this case the original RequestURI
//     is updated according to the new relative path.
func (u *URI) UpdateBytes(newURI []byte) {
	u.requestURI = u.updateBytes(newURI, u.requestURI)
}

// Parse initializes URI from the given host and uri.
//
// host may be nil. In this case uri must contain fully qualified uri,
// i.e. with scheme and host. http is assumed if scheme is omitted.
//
// uri may contain e.g. RequestURI without scheme and host if host is non-empty.
func (u *URI) Parse(host, uri []byte) {
	u.parse(host, uri, false)
}

// Maybe rawURL is of the form scheme:path.
// (Scheme must be [a-zA-Z][a-zA-Z0-9+-.]*)
// If so, return scheme, path; else return nil, rawURL.
func getScheme(rawURL []byte) (scheme, path []byte) {
	for i := 0; i < len(rawURL); i++ {
		c := rawURL[i]
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
		// do nothing
		case '0' <= c && c <= '9' || c == '+' || c == '-' || c == '.':
			if i == 0 {
				return nil, rawURL
			}
		case c == ':':
			return checkSchemeWhenCharIsColon(i, rawURL)
		default:
			// we have encountered an invalid character,
			// so there is no valid scheme
			return nil, rawURL
		}
	}
	return nil, rawURL
}

func (u *URI) parse(host, uri []byte, isTLS bool) {
	u.Reset()

	if stringContainsCTLByte(uri) {
		return
	}

	if len(host) == 0 || bytes.Contains(uri, bytestr.StrColonSlashSlash) {
		scheme, newHost, newURI := splitHostURI(host, uri)
		u.scheme = append(u.scheme, scheme...)
		bytesconv.LowercaseBytes(u.scheme)
		host = newHost
		uri = newURI
	}

	if isTLS {
		u.scheme = append(u.scheme[:0], bytestr.StrHTTPS...)
	}

	if n := bytes.Index(host, bytestr.StrAt); n >= 0 {
		auth := host[:n]
		host = host[n+1:]

		if n := bytes.Index(auth, bytestr.StrColon); n >= 0 {
			u.username = append(u.username[:0], auth[:n]...)
			u.password = append(u.password[:0], auth[n+1:]...)
		} else {
			u.username = append(u.username[:0], auth...)
			u.password = u.password[:0]
		}
	}

	u.host = append(u.host, host...)
	bytesconv.LowercaseBytes(u.host)

	b := uri
	queryIndex := bytes.IndexByte(b, '?')
	fragmentIndex := bytes.IndexByte(b, '#')
	// Ignore query in fragment part
	if fragmentIndex >= 0 && queryIndex > fragmentIndex {
		queryIndex = -1
	}

	if queryIndex < 0 && fragmentIndex < 0 {
		u.pathOriginal = append(u.pathOriginal, b...)
		u.path = normalizePath(u.path, u.pathOriginal)
		return
	}

	if queryIndex >= 0 {
		// Path is everything up to the start of the query
		u.pathOriginal = append(u.pathOriginal, b[:queryIndex]...)
		u.path = normalizePath(u.path, u.pathOriginal)

		if fragmentIndex < 0 {
			u.queryString = append(u.queryString, b[queryIndex+1:]...)
		} else {
			u.queryString = append(u.queryString, b[queryIndex+1:fragmentIndex]...)
			u.hash = append(u.hash, b[fragmentIndex+1:]...)
		}
		return
	}

	// fragmentIndex >= 0 && queryIndex < 0
	// Path is up to the start of fragment
	u.pathOriginal = append(u.pathOriginal, b[:fragmentIndex]...)
	u.path = normalizePath(u.path, u.pathOriginal)
	u.hash = append(u.hash, b[fragmentIndex+1:]...)
}

// stringContainsCTLByte reports whether s contains any ASCII control character.
func stringContainsCTLByte(s []byte) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < ' ' || b == 0x7f {
			return true
		}
	}
	return false
}

func splitHostURI(host, uri []byte) ([]byte, []byte, []byte) {
	scheme, path := getScheme(uri)

	if scheme == nil {
		return bytestr.StrHTTP, host, uri
	}

	uri = path[len(bytestr.StrSlashSlash):]
	n := bytes.IndexByte(uri, '/')
	if n < 0 {
		// A hack for bogus urls like foobar.com?a=b without
		// slash after host.
		if n = bytes.IndexByte(uri, '?'); n >= 0 {
			return scheme, uri[:n], uri[n:]
		}
		return scheme, uri, bytestr.StrSlash
	}
	return scheme, uri[:n], uri[n:]
}

func normalizePath(dst, src []byte) []byte {
	dst = dst[:0]
	dst = addLeadingSlash(dst, src)
	dst = decodeArgAppendNoPlus(dst, src)

	// Windows server need to replace all backslashes with
	// forward slashes to avoid path traversal attacks.
	if filepath.Separator == '\\' {
		for {
			n := bytes.IndexByte(dst, '\\')
			if n < 0 {
				break
			}
			dst[n] = '/'
		}
	}

	// remove duplicate slashes
	b := dst
	bSize := len(b)
	for {
		n := bytes.Index(b, bytestr.StrSlashSlash)
		if n < 0 {
			break
		}
		b = b[n:]
		copy(b, b[1:])
		b = b[:len(b)-1]
		bSize--
	}
	dst = dst[:bSize]

	// remove /./ parts
	b = dst
	for {
		n := bytes.Index(b, bytestr.StrSlashDotSlash)
		if n < 0 {
			break
		}
		nn := n + len(bytestr.StrSlashDotSlash) - 1
		copy(b[n:], b[nn:])
		b = b[:len(b)-nn+n]
	}

	// remove /foo/../ parts
	for {
		n := bytes.Index(b, bytestr.StrSlashDotDotSlash)
		if n < 0 {
			break
		}
		nn := bytes.LastIndexByte(b[:n], '/')
		if nn < 0 {
			nn = 0
		}
		n += len(bytestr.StrSlashDotDotSlash) - 1
		copy(b[nn:], b[n:])
		b = b[:len(b)-n+nn]
	}

	// remove trailing /foo/..
	n := bytes.LastIndex(b, bytestr.StrSlashDotDot)
	if n >= 0 && n+len(bytestr.StrSlashDotDot) == len(b) {
		nn := bytes.LastIndexByte(b[:n], '/')
		if nn < 0 {
			return bytestr.StrSlash
		}
		b = b[:nn+1]
	}

	return b
}

func copyArgs(dst, src []argsKV) []argsKV {
	if cap(dst) < len(src) {
		tmp := make([]argsKV, len(src))
		copy(tmp, dst)
		dst = tmp
	}
	n := len(src)
	dst = dst[:n]
	for i := 0; i < n; i++ {
		dstKV := &dst[i]
		srcKV := &src[i]
		dstKV.key = append(dstKV.key[:0], srcKV.key...)
		if srcKV.noValue {
			dstKV.value = dstKV.value[:0]
		} else {
			dstKV.value = append(dstKV.value[:0], srcKV.value...)
		}
		dstKV.noValue = srcKV.noValue
	}
	return dst
}

func (u *URI) updateBytes(newURI, buf []byte) []byte {
	if len(newURI) == 0 {
		return buf
	}

	n := bytes.Index(newURI, bytestr.StrSlashSlash)
	if n >= 0 {
		// absolute uri
		var b [32]byte
		schemeOriginal := b[:0]
		if len(u.scheme) > 0 {
			schemeOriginal = append([]byte(nil), u.scheme...)
		}
		if n == 0 {
			newURI = bytes.Join([][]byte{u.scheme, bytestr.StrColon, newURI}, nil)
		}
		u.Parse(nil, newURI)
		if len(schemeOriginal) > 0 && len(u.scheme) == 0 {
			u.scheme = append(u.scheme[:0], schemeOriginal...)
		}
		return buf
	}

	if newURI[0] == '/' {
		// uri without host
		buf = u.appendSchemeHost(buf[:0])
		buf = append(buf, newURI...)
		u.Parse(nil, buf)
		return buf
	}

	// relative path
	switch newURI[0] {
	case '?':
		// query string only update
		u.SetQueryStringBytes(newURI[1:])
		return append(buf[:0], u.FullURI()...)
	case '#':
		// update only hash
		u.SetHashBytes(newURI[1:])
		return append(buf[:0], u.FullURI()...)
	default:
		// update the last path part after the slash
		path := u.Path()
		n = bytes.LastIndexByte(path, '/')
		if n < 0 {
			panic("BUG: path must contain at least one slash")
		}
		buf = u.appendSchemeHost(buf[:0])
		buf = bytesconv.AppendQuotedPath(buf, path[:n+1])
		buf = append(buf, newURI...)
		u.Parse(nil, buf)
		return buf
	}
}

// AppendBytes appends full uri to dst and returns the extended dst.
func (u *URI) AppendBytes(dst []byte) []byte {
	dst = u.appendSchemeHost(dst)
	dst = append(dst, u.RequestURI()...)
	if len(u.hash) > 0 {
		dst = append(dst, '#')
		dst = append(dst, u.hash...)
	}
	return dst
}

// RequestURI returns RequestURI - i.e. URI without Scheme and Host.
func (u *URI) RequestURI() []byte {
	var dst []byte
	if u.DisablePathNormalizing {
		dst = append(u.requestURI[:0], u.PathOriginal()...)
	} else {
		dst = bytesconv.AppendQuotedPath(u.requestURI[:0], u.Path())
	}
	if u.queryArgs.Len() > 0 {
		dst = append(dst, '?')
		dst = u.queryArgs.AppendBytes(dst)
	} else if len(u.queryString) > 0 {
		dst = append(dst, '?')
		dst = append(dst, u.queryString...)
	}
	u.requestURI = dst
	return u.requestURI
}

func (u *URI) appendSchemeHost(dst []byte) []byte {
	dst = append(dst, u.Scheme()...)
	dst = append(dst, bytestr.StrColonSlashSlash...)
	return append(dst, u.Host()...)
}

// FullURI returns full uri in the form {Scheme}://{Host}{RequestURI}#{Hash}.
func (u *URI) FullURI() []byte {
	u.fullURI = u.AppendBytes(u.fullURI[:0])
	return u.fullURI
}

func ParseURI(uriStr string) *URI {
	uri := &URI{}
	uri.Parse(nil, []byte(uriStr))

	return uri
}

type Proxy func(*Request) (*URI, error)

func ProxyURI(fixedURI *URI) Proxy {
	return func(*Request) (*URI, error) {
		return fixedURI, nil
	}
}
