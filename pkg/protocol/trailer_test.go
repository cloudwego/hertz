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
	assert.NotNil(t, tr.Add(consts.HeaderProxyConnection, ""))
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
