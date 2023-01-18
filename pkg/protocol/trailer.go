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

package protocol

import (
	"bytes"

	"github.com/cloudwego/hertz/internal/bytestr"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type Trailer struct {
	h                  []argsKV
	bufKV              argsKV
	disableNormalizing bool
}

// Get returns trailer value for the given key.
func (t *Trailer) Get(key string) string {
	return string(t.Peek(key))
}

// Peek returns trailer value for the given key.
//
// Returned value is valid until the next call to Trailer.
// Do not store references to returned value. Make copies instead.
func (t *Trailer) Peek(key string) []byte {
	k := getHeaderKeyBytes(&t.bufKV, key, t.disableNormalizing)
	return peekArgBytes(t.h, k)
}

// Del deletes trailer with the given key.
func (t *Trailer) Del(key string) {
	k := getHeaderKeyBytes(&t.bufKV, key, t.disableNormalizing)
	t.h = delAllArgsBytes(t.h, k)
}

// VisitAll calls f for each header.
func (t *Trailer) VisitAll(f func(key, value []byte)) {
	visitArgs(t.h, f)
}

// Set sets the given 'key: value' trailer.
//
// If the key is forbidden by RFC 7230, section 4.1.2, Set will return error
func (t *Trailer) Set(key, value string) error {
	initHeaderKV(&t.bufKV, key, value, t.disableNormalizing)
	return t.setArgBytes(t.bufKV.key, t.bufKV.value, ArgsHasValue)
}

// Add adds the given 'key: value' trailer.
//
// Multiple headers with the same key may be added with this function.
// Use Set for setting a single header for the given key.
//
// If the key is forbidden by RFC 7230, section 4.1.2, Add will return error
func (t *Trailer) Add(key, value string) error {
	initHeaderKV(&t.bufKV, key, value, t.disableNormalizing)
	return t.addArgBytes(t.bufKV.key, t.bufKV.value, ArgsHasValue)
}

func (t *Trailer) addArgBytes(key, value []byte, noValue bool) error {
	if IsBadTrailer(key) {
		return errs.NewPublicf("forbidden trailer key: %q", key)
	}
	t.h = appendArgBytes(t.h, key, value, noValue)
	return nil
}

func (t *Trailer) setArgBytes(key, value []byte, noValue bool) error {
	if IsBadTrailer(key) {
		return errs.NewPublicf("forbidden trailer key: %q", key)
	}
	t.h = setArgBytes(t.h, key, value, noValue)
	return nil
}

func (t *Trailer) UpdateArgBytes(key, value []byte) error {
	if IsBadTrailer(key) {
		return errs.NewPublicf("forbidden trailer key: %q", key)
	}

	t.h = updateArgBytes(t.h, key, value)
	return nil
}

func (t *Trailer) GetTrailers() []argsKV {
	return t.h
}

func (t *Trailer) Empty() bool {
	return len(t.h) == 0
}

// GetBytes return the 'Trailer' Header which is composed by the Trailer key
func (t *Trailer) GetBytes() []byte {
	var dst []byte
	for i, n := 0, len(t.h); i < n; i++ {
		kv := &t.h[i]
		dst = append(dst, kv.key...)
		if i+1 < n {
			dst = append(dst, bytestr.StrCommaSpace...)
		}
	}
	return dst
}

func (t *Trailer) ResetSkipNormalize() {
	t.h = t.h[:0]
}

func (t *Trailer) Reset() {
	t.disableNormalizing = false
	t.ResetSkipNormalize()
}

func (t *Trailer) DisableNormalizing() {
	t.disableNormalizing = true
}

func (t *Trailer) IsDisableNormalizing() bool {
	return t.disableNormalizing
}

// CopyTo copies all the trailer to dst.
func (t *Trailer) CopyTo(dst *Trailer) {
	dst.Reset()

	dst.disableNormalizing = t.disableNormalizing
	dst.h = copyArgs(dst.h, t.h)
}

func (t *Trailer) SetTrailers(trailers []byte) (err error) {
	t.ResetSkipNormalize()
	for i := -1; i+1 < len(trailers); {
		trailers = trailers[i+1:]
		i = bytes.IndexByte(trailers, ',')
		if i < 0 {
			i = len(trailers)
		}
		trailerKey := trailers[:i]
		for len(trailerKey) > 0 && trailerKey[0] == ' ' {
			trailerKey = trailerKey[1:]
		}
		for len(trailerKey) > 0 && trailerKey[len(trailerKey)-1] == ' ' {
			trailerKey = trailerKey[:len(trailerKey)-1]
		}

		utils.NormalizeHeaderKey(trailerKey, t.disableNormalizing)
		err = t.addArgBytes(trailerKey, nilByteSlice, argsNoValue)
	}
	return
}

func (t *Trailer) Header() []byte {
	t.bufKV.value = t.AppendBytes(t.bufKV.value[:0])
	return t.bufKV.value
}

func (t *Trailer) AppendBytes(dst []byte) []byte {
	for i, n := 0, len(t.h); i < n; i++ {
		kv := &t.h[i]
		dst = appendHeaderLine(dst, kv.key, kv.value)
	}

	dst = append(dst, bytestr.StrCRLF...)
	return dst
}

func IsBadTrailer(key []byte) bool {
	switch key[0] | 0x20 {
	case 'a':
		return utils.CaseInsensitiveCompare(key, bytestr.StrAuthorization)
	case 'c':
		if len(key) >= len(consts.HeaderContentType) && utils.CaseInsensitiveCompare(key[:8], bytestr.StrContentType[:8]) {
			// skip compare prefix 'Content-'
			return utils.CaseInsensitiveCompare(key[8:], bytestr.StrContentEncoding[8:]) ||
				utils.CaseInsensitiveCompare(key[8:], bytestr.StrContentLength[8:]) ||
				utils.CaseInsensitiveCompare(key[8:], bytestr.StrContentType[8:]) ||
				utils.CaseInsensitiveCompare(key[8:], bytestr.StrContentRange[8:])
		}
		return utils.CaseInsensitiveCompare(key, bytestr.StrConnection)
	case 'e':
		return utils.CaseInsensitiveCompare(key, bytestr.StrExpect)
	case 'h':
		return utils.CaseInsensitiveCompare(key, bytestr.StrHost)
	case 'k':
		return utils.CaseInsensitiveCompare(key, bytestr.StrKeepAlive)
	case 'm':
		return utils.CaseInsensitiveCompare(key, bytestr.StrMaxForwards)
	case 'p':
		if len(key) >= len(consts.HeaderProxyConnection) && utils.CaseInsensitiveCompare(key[:6], bytestr.StrProxyConnection[:6]) {
			// skip compare prefix 'Proxy-'
			return utils.CaseInsensitiveCompare(key[6:], bytestr.StrProxyConnection[6:]) ||
				utils.CaseInsensitiveCompare(key[6:], bytestr.StrProxyAuthenticate[6:]) ||
				utils.CaseInsensitiveCompare(key[6:], bytestr.StrProxyAuthorization[6:])
		}
	case 'r':
		return utils.CaseInsensitiveCompare(key, bytestr.StrRange)
	case 't':
		return utils.CaseInsensitiveCompare(key, bytestr.StrTE) ||
			utils.CaseInsensitiveCompare(key, bytestr.StrTrailer) ||
			utils.CaseInsensitiveCompare(key, bytestr.StrTransferEncoding)
	case 'w':
		return utils.CaseInsensitiveCompare(key, bytestr.StrWWWAuthenticate)
	}
	return false
}
