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
 * The MIT License (MIT)
 *
 * Copyright (c) 2014 Manuel Martínez-Almeida
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
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package utils

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestPathCleanPath(t *testing.T) {
	normalPath := "/Foo/Bar/go/src/github.com/cloudwego/hertz/pkg/common/utils/path_test.go"
	expectedNormalPath := "/Foo/Bar/go/src/github.com/cloudwego/hertz/pkg/common/utils/path_test.go"
	cleanNormalPath := CleanPath(normalPath)
	assert.DeepEqual(t, expectedNormalPath, cleanNormalPath)

	singleDotPath := "/Foo/Bar/./././go/src"
	expectedSingleDotPath := "/Foo/Bar/go/src"
	cleanSingleDotPath := CleanPath(singleDotPath)
	assert.DeepEqual(t, expectedSingleDotPath, cleanSingleDotPath)

	doubleDotPath := "../../.."
	expectedDoubleDotPath := "/"
	cleanDoublePotPath := CleanPath(doubleDotPath)
	assert.DeepEqual(t, expectedDoubleDotPath, cleanDoublePotPath)

	// MultiDot can be treated as a file name
	multiDotPath := "/../...."
	expectedMultiDotPath := "/...."
	cleanMultiDotPath := CleanPath(multiDotPath)
	assert.DeepEqual(t, expectedMultiDotPath, cleanMultiDotPath)

	nullPath := ""
	expectedNullPath := "/"
	cleanNullPath := CleanPath(nullPath)
	assert.DeepEqual(t, expectedNullPath, cleanNullPath)

	relativePath := "/Foo/Bar/../go/src/../../github.com/cloudwego/hertz"
	expectedRelativePath := "/Foo/github.com/cloudwego/hertz"
	cleanRelativePath := CleanPath(relativePath)
	assert.DeepEqual(t, expectedRelativePath, cleanRelativePath)

	multiSlashPath := "///////Foo//Bar////go//src/github.com/cloudwego/hertz//.."
	expectedMultiSlashPath := "/Foo/Bar/go/src/github.com/cloudwego"
	cleanMultiSlashPath := CleanPath(multiSlashPath)
	assert.DeepEqual(t, expectedMultiSlashPath, cleanMultiSlashPath)

	inputPath := "/Foo/Bar/go/src/github.com/cloudwego/hertz/pkg/common/utils/path_test.go/."
	expectedPath := "/Foo/Bar/go/src/github.com/cloudwego/hertz/pkg/common/utils/path_test.go/"
	cleanedPath := CleanPath(inputPath)
	assert.DeepEqual(t, expectedPath, cleanedPath)
}

// The Function AddMissingPort can only add the missed port, don't consider the other error case.
func TestPathAddMissingPort(t *testing.T) {
	ipList := []string{"127.0.0.1", "111.111.1.1", "[0:0:0:0:0:ffff:192.1.56.10]", "[0:0:0:0:0:ffff:c0a8:101]", "www.foobar.com"}
	for _, ip := range ipList {
		assert.DeepEqual(t, ip+":443", AddMissingPort(ip, true))
		assert.DeepEqual(t, ip+":80", AddMissingPort(ip, false))
		customizedPort := ":8080"
		assert.DeepEqual(t, ip+customizedPort, AddMissingPort(ip+customizedPort, true))
		assert.DeepEqual(t, ip+customizedPort, AddMissingPort(ip+customizedPort, false))
	}
}

// TestBufApp tests different branch logic of bufApp function
// Handles buffer creation and management when modifying strings
func TestBufApp(t *testing.T) {
	var buf []byte
	s := "test"
	w := 1
	
	// Test case when buffer is empty and next char is same as original string
	bufApp(&buf, s, w, 'e')
	assert.DeepEqual(t, 0, len(buf)) // Buffer should remain empty as no modification needed

	// Test case when buffer is empty and next char differs from original string
	bufApp(&buf, s, w, 'x')
	assert.DeepEqual(t, len(s), len(buf))  // New buffer should be created
	assert.DeepEqual(t, byte('x'), buf[w]) // New char should be written to buffer
	assert.DeepEqual(t, byte('t'), buf[0]) // Original string prefix should be copied

	// Test case when buffer already exists
	bufApp(&buf, s, 2, 'y')
	assert.DeepEqual(t, byte('y'), buf[2]) // New char should be written to buffer
	
	// Test case with index w = 0 (first character)
	var buf2 []byte
	bufApp(&buf2, s, 0, 'X')
	assert.DeepEqual(t, len(s), len(buf2))
	assert.DeepEqual(t, byte('X'), buf2[0])
	
	// Test case with large string (exceeding stack buffer size)
	var buf3 []byte
	largeString := string(make([]byte, 256)) // Larger than stackBufSize (128)
	bufApp(&buf3, largeString, 100, 'Z')
	assert.DeepEqual(t, len(largeString), len(buf3))
	assert.DeepEqual(t, byte('Z'), buf3[100])
	
	// Test edge case: when w is at the end of string
	var buf4 []byte
	lastIndex := len(s) - 1
	bufApp(&buf4, s, lastIndex, 'L')
	assert.DeepEqual(t, len(s), len(buf4))
	assert.DeepEqual(t, byte('L'), buf4[lastIndex])
}
