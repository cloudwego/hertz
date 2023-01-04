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

package ext

import (
	"strings"
	"testing"

	"github.com/cloudwego/hertz/internal/bytestr"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func Test_stripSpace(t *testing.T) {
	a := stripSpace([]byte("     a"))
	b := stripSpace([]byte("b       "))
	c := stripSpace([]byte("    c     "))
	assert.DeepEqual(t, []byte("a"), a)
	assert.DeepEqual(t, []byte("b"), b)
	assert.DeepEqual(t, []byte("c"), c)
}

func Test_bufferSnippet(t *testing.T) {
	a := make([]byte, 39)
	b := make([]byte, 41)
	assert.False(t, strings.Contains(BufferSnippet(a), "\"...\""))
	assert.True(t, strings.Contains(BufferSnippet(b), "\"...\""))
}

func Test_isOnlyCRLF(t *testing.T) {
	assert.True(t, isOnlyCRLF([]byte("\r\n")))
	assert.True(t, isOnlyCRLF([]byte("\n")))
}

func TestIsBadTrailer(t *testing.T) {
	assert.True(t, IsBadTrailer(bytestr.StrAuthorization))
	assert.True(t, IsBadTrailer(bytestr.StrContentEncoding))
	assert.True(t, IsBadTrailer(bytestr.StrContentLength))
	assert.True(t, IsBadTrailer(bytestr.StrContentType))
	assert.True(t, IsBadTrailer(bytestr.StrContentRange))
	assert.True(t, IsBadTrailer(bytestr.StrConnection))
	assert.True(t, IsBadTrailer(bytestr.StrExpect))
	assert.True(t, IsBadTrailer(bytestr.StrHost))
	assert.True(t, IsBadTrailer(bytestr.StrKeepAlive))
	assert.True(t, IsBadTrailer(bytestr.StrMaxForwards))
	assert.True(t, IsBadTrailer(bytestr.StrProxyConnection))
	assert.True(t, IsBadTrailer(bytestr.StrProxyAuthenticate))
	assert.True(t, IsBadTrailer(bytestr.StrProxyAuthorization))
	assert.True(t, IsBadTrailer(bytestr.StrRange))
	assert.True(t, IsBadTrailer(bytestr.StrTE))
	assert.True(t, IsBadTrailer(bytestr.StrTrailer))
	assert.True(t, IsBadTrailer(bytestr.StrTransferEncoding))
	assert.True(t, IsBadTrailer(bytestr.StrWWWAuthenticate))
}
