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

package protocol

import (
	"strings"
	"testing"

	"github.com/cloudwego/hertz/internal/bytestr"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestTrailerAdd(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Add("foo", "value1"))
	assert.Nil(t, tr.Add("foo", "value2"))
	assert.Nil(t, tr.Add("bar", "value3"))
	assert.True(t, strings.Contains(string(tr.Header()), "Foo: value1"))
	assert.True(t, strings.Contains(string(tr.Header()), "Foo: value2"))
	assert.True(t, strings.Contains(string(tr.Header()), "Bar: value3"))
}

func TestTrailerAddError(t *testing.T) {
	var tr Trailer
	assert.NotNil(t, tr.Add(consts.HeaderContentType, ""))
	assert.NotNil(t, tr.Set(consts.HeaderProxyConnection, ""))
}

func TestTrailerDel(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Add("foo", "value1"))
	assert.Nil(t, tr.Add("foo", "value2"))
	assert.Nil(t, tr.Add("bar", "value3"))
	tr.Del("foo")
	assert.False(t, strings.Contains(string(tr.Header()), "Foo: value1"))
	assert.False(t, strings.Contains(string(tr.Header()), "Foo: value2"))
	assert.True(t, strings.Contains(string(tr.Header()), "Bar: value3"))
}

func TestTrailerSet(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Set("foo", "value1"))
	assert.Nil(t, tr.Set("foo", "value2"))
	assert.Nil(t, tr.Set("bar", "value3"))
	assert.False(t, strings.Contains(string(tr.Header()), "Foo: value1"))
	assert.True(t, strings.Contains(string(tr.Header()), "Foo: value2"))
	assert.True(t, strings.Contains(string(tr.Header()), "Bar: value3"))
}

func TestTrailerGet(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Add("foo", "value1"))
	assert.Nil(t, tr.Add("bar", "value3"))
	assert.DeepEqual(t, tr.Get("foo"), "value1")
	assert.DeepEqual(t, tr.Get("bar"), "value3")
}

func TestTrailerUpdateArgBytes(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.AddArgBytes([]byte("Foo"), []byte("value0"), argsNoValue))
	assert.Nil(t, tr.UpdateArgBytes([]byte("Foo"), []byte("value1")))
	assert.Nil(t, tr.UpdateArgBytes([]byte("Foo"), []byte("value2")))
	assert.Nil(t, tr.UpdateArgBytes([]byte("Bar"), []byte("value3")))
	assert.True(t, strings.Contains(string(tr.Header()), "Foo: value1"))
	assert.False(t, strings.Contains(string(tr.Header()), "Foo: value2"))
	assert.False(t, strings.Contains(string(tr.Header()), "Bar: value3"))
}

func TestTrailerEmpty(t *testing.T) {
	var tr Trailer
	assert.DeepEqual(t, tr.Empty(), true)
	assert.Nil(t, tr.Set("foo", ""))
	assert.DeepEqual(t, tr.Empty(), false)
}

func TestTrailerVisitAll(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Add("foo", "value1"))
	assert.Nil(t, tr.Add("bar", "value2"))
	tr.VisitAll(
		func(k, v []byte) {
			key := string(k)
			value := string(v)
			if (key != "Foo" || value != "value1") && (key != "Bar" || value != "value2") {
				t.Fatalf("Unexpected (%v, %v). Expected %v", key, value, "(foo, value1) or (bar, value2)")
			}
		})
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
