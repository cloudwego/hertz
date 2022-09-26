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

package bytesconv

import (
	"net/url"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/network"
)

func TestAppendDate(t *testing.T) {
	t.Parallel()
	// GMT+8
	shanghaiTimeZone := time.FixedZone("Asia/Shanghai", 8*60*60)

	for _, c := range []struct {
		name    string
		date    time.Time
		dateStr string
	}{
		{
			name:    "UTC",
			date:    time.Date(2022, 6, 15, 11, 12, 13, 123, time.UTC),
			dateStr: "Wed, 15 Jun 2022 11:12:13 GMT",
		},
		{
			name:    "Asia/Shanghai",
			date:    time.Date(2022, 6, 15, 3, 12, 45, 999, shanghaiTimeZone),
			dateStr: "Tue, 14 Jun 2022 19:12:45 GMT",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := AppendHTTPDate(nil, c.date)
			assert.DeepEqual(t, c.dateStr, B2s(s))
		})
	}
}

func TestLowercaseBytes(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		b1, b2 []byte
	}{
		{[]byte("CLOUDWEGO-HERTZ"), []byte("cloudwego-hertz")},
		{[]byte("CLOUDWEGO"), []byte("cloudwego")},
		{[]byte("HERTZ"), []byte("hertz")},
	} {
		LowercaseBytes(v.b1)
		assert.DeepEqual(t, v.b2, v.b1)
	}
}

// The test converts byte slice to a string without memory allocation.
func TestB2s(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		b []byte
	}{
		{"cloudwego-hertz", []byte("cloudwego-hertz")},
		{"cloudwego", []byte("cloudwego")},
		{"hertz", []byte("hertz")},
	} {
		assert.DeepEqual(t, v.s, B2s(v.b))
	}
}

// The test converts string to a byte slice without memory allocation.
func TestS2b(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		b []byte
	}{
		{"cloudwego-hertz", []byte("cloudwego-hertz")},
		{"cloudwego", []byte("cloudwego")},
		{"hertz", []byte("hertz")},
	} {
		assert.DeepEqual(t, S2b(v.s), v.b)
	}
}

// common test function for 32bit and 64bit
func testWriteHexInt(t *testing.T, n int, expectedS string) {
	w := bytebufferpool.Get()
	zw := network.NewWriter(w)
	if err := WriteHexInt(zw, n); err != nil {
		t.Errorf("unexpected error when writing hex %x: %v", n, err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error when flushing hex %x: %v", n, err)
	}
	s := B2s(w.B)
	assert.DeepEqual(t, s, expectedS)
}

// common test function for 32bit and 64bit
func testReadHexInt(t *testing.T, s string, expectedN int) {
	zr := mock.NewZeroCopyReader(s)
	n, err := ReadHexInt(zr)
	if err != nil {
		t.Errorf("unexpected error: %v. s=%q", err, s)
	}
	assert.DeepEqual(t, n, expectedN)
}

func TestAppendQuotedPath(t *testing.T) {
	t.Parallel()

	// Test all characters
	pathSegment := make([]byte, 256)
	for i := 0; i < 256; i++ {
		pathSegment[i] = byte(i)
	}
	for _, s := range []struct {
		path string
	}{
		{"/"},
		{"//"},
		{"/foo/bar"},
		{"*"},
		{"/foo/" + B2s(pathSegment)},
	} {
		u := url.URL{Path: s.path}
		expectedS := u.EscapedPath()
		res := B2s(AppendQuotedPath(nil, S2b(s.path)))
		assert.DeepEqual(t, expectedS, res)
	}
}

func TestAppendQuotedArg(t *testing.T) {
	t.Parallel()

	// Sync with url.QueryEscape
	allcases := make([]byte, 256)
	for i := 0; i < 256; i++ {
		allcases[i] = byte(i)
	}
	res := B2s(AppendQuotedArg(nil, allcases))
	expect := url.QueryEscape(B2s(allcases))
	assert.DeepEqual(t, expect, res)
}

func TestParseHTTPDate(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		t string
	}{
		{"Thu, 04 Feb 2010 21:00:57 PST"},
		{"Mon, 02 Jan 2006 15:04:05 MST"},
	} {
		t1, err := time.Parse(time.RFC1123, v.t)
		if err != nil {
			t.Fatalf("unexpected error: %v. t=%q", err, v.t)
		}
		t2, err := ParseHTTPDate(S2b(t1.Format(time.RFC1123)))
		if err != nil {
			t.Fatalf("unexpected error: %v. t=%q", err, v.t)
		}
		assert.DeepEqual(t, t1, t2)
	}
}
