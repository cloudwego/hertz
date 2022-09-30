//go:build amd64 || arm64 || ppc64
// +build amd64 arm64 ppc64

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

package bytesconv

import (
	"fmt"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestWriteHexInt(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		n int
	}{
		{"0", 0},
		{"1", 1},
		{"123", 0x123},
		{"7fffffffffffffff", 0x7fffffffffffffff},
	} {
		testWriteHexInt(t, v.n, v.s)
	}
}

func TestReadHexInt(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		n int
	}{
		//errTooLargeHexNum "too large hex number"
		//{"0123456789abcdef", -1},
		{"0", 0},
		{"fF", 0xff},
		{"00abc", 0xabc},
		{"7fffffff", 0x7fffffff},
		{"000", 0},
		{"1234ZZZ", 0x1234},
		{"7ffffffffffffff", 0x7ffffffffffffff},
	} {
		testReadHexInt(t, v.s, v.n)
	}
}

func TestParseUint(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		i int
	}{
		{"0", 0},
		{"123", 123},
		{"1234567890", 1234567890},
		{"123456789012345678", 123456789012345678},
		{"9223372036854775807", 9223372036854775807},
	} {
		n, err := ParseUint(S2b(v.s))
		if err != nil {
			t.Errorf("unexpected error: %v. s=%q n=%v", err, v.s, n)
		}
		assert.DeepEqual(t, n, v.i)
	}
}

func TestParseUintError(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
	}{
		{""},
		{"cloudwego123"},
		{"1234.545"},
		{"-9223372036854775808"},
		{"9223372036854775808"},
		{"18446744073709551615"},
	} {
		n, err := ParseUint(S2b(v.s))
		if err == nil {
			t.Fatalf("Expecting error when parsing %q. obtained %d", v.s, n)
		}
		if n >= 0 {
			t.Fatalf("Unexpected n=%d when parsing %q. Expected negative num", n, v.s)
		}
	}
}

func TestAppendUint(t *testing.T) {
	t.Parallel()

	for _, s := range []struct {
		n int
	}{
		{0},
		{123},
		{0x7fffffffffffffff},
	} {
		expectedS := fmt.Sprintf("%d", s.n)
		s := AppendUint(nil, s.n)
		assert.DeepEqual(t, expectedS, B2s(s))
	}
}
