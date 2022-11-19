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

package utils

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

// test assert func
func TestUtilsAssert(t *testing.T) {
	// nothing to test
}

func TestUtilsIsTrueString(t *testing.T) {
	normalTrueStr := "true"
	upperTrueStr := "trUe"
	otherStr := "hertz"

	assert.DeepEqual(t, true, IsTrueString(normalTrueStr))
	assert.DeepEqual(t, true, IsTrueString(upperTrueStr))
	assert.DeepEqual(t, false, IsTrueString(otherStr))
}

// used for TestUtilsNameOfFunction
func testName(a int) {
}

// return the relative path for the function
func TestUtilsNameOfFunction(t *testing.T) {
	pathOfTestName := "github.com/cloudwego/hertz/pkg/common/utils.testName"
	pathOfIsTrueString := "github.com/cloudwego/hertz/pkg/common/utils.IsTrueString"
	nameOfTestName := NameOfFunction(testName)
	nameOfIsTrueString := NameOfFunction(IsTrueString)

	assert.DeepEqual(t, pathOfTestName, nameOfTestName)
	assert.DeepEqual(t, pathOfIsTrueString, nameOfIsTrueString)
}

func TestUtilsCaseInsensitiveCompare(t *testing.T) {
	lowerStr := []byte("content-length")
	upperStr := []byte("Content-Length")
	assert.DeepEqual(t, true, CaseInsensitiveCompare(lowerStr, upperStr))

	lessStr := []byte("content-type")
	moreStr := []byte("content-length")
	assert.DeepEqual(t, false, CaseInsensitiveCompare(lessStr, moreStr))

	firstStr := []byte("content-type")
	secondStr := []byte("contant-type")
	assert.DeepEqual(t, false, CaseInsensitiveCompare(firstStr, secondStr))
}

// NormalizeHeaderKey can upper the first letter and lower the other letter in
// HTTP header, invervaled by '-'.
// Example: "content-type" -> "Content-Type"
func TestUtilsNormalizeHeaderKey(t *testing.T) {
	contentTypeStr := []byte("Content-Type")
	lowerContentTypeStr := []byte("content-type")
	mixedContentTypeStr := []byte("conTENt-tYpE")
	mixedContertTypeStrWithoutNormalizing := []byte("Content-type")
	NormalizeHeaderKey(contentTypeStr, false)
	NormalizeHeaderKey(lowerContentTypeStr, false)
	NormalizeHeaderKey(mixedContentTypeStr, false)
	NormalizeHeaderKey(lowerContentTypeStr, true)

	assert.DeepEqual(t, "Content-Type", string(contentTypeStr))
	assert.DeepEqual(t, "Content-Type", string(lowerContentTypeStr))
	assert.DeepEqual(t, "Content-Type", string(mixedContentTypeStr))
	assert.DeepEqual(t, "Content-type", string(mixedContertTypeStrWithoutNormalizing))
}

// Cutting up the header Type.
// Example: "Content-Type: application/x-www-form-urlencoded\r\nDate: Fri, 6 Aug 2021 11:00:31 GMT"
// ->"Content-Type: application/x-www-form-urlencoded" and "Date: Fri, 6 Aug 2021 11:00:31 GMT"
func TestUtilsNextLine(t *testing.T) {
	multiHeaderStr := []byte("Content-Type: application/x-www-form-urlencoded\r\nDate: Fri, 6 Aug 2021 11:00:31 GMT")
	contentTypeStr, dateStr, hErr := NextLine(multiHeaderStr)
	assert.DeepEqual(t, nil, hErr)
	assert.DeepEqual(t, "Content-Type: application/x-www-form-urlencoded", string(contentTypeStr))
	assert.DeepEqual(t, "Date: Fri, 6 Aug 2021 11:00:31 GMT", string(dateStr))

	multiHeaderStrWithoutReturn := []byte("Content-Type: application/x-www-form-urlencoded\nDate: Fri, 6 Aug 2021 11:00:31 GMT")
	contentTypeStr, dateStr, hErr = NextLine(multiHeaderStrWithoutReturn)
	assert.DeepEqual(t, nil, hErr)
	assert.DeepEqual(t, "Content-Type: application/x-www-form-urlencoded", string(contentTypeStr))
	assert.DeepEqual(t, "Date: Fri, 6 Aug 2021 11:00:31 GMT", string(dateStr))

	singleHeaderStrWithFirstNewLine := []byte("\nContent-Type: application/x-www-form-urlencoded")
	firstStr, secondStr, sErr := NextLine(singleHeaderStrWithFirstNewLine)
	assert.DeepEqual(t, nil, sErr)
	assert.DeepEqual(t, string(""), string(firstStr))
	assert.DeepEqual(t, "Content-Type: application/x-www-form-urlencoded", string(secondStr))

	singleHeaderStr := []byte("Content-Type: application/x-www-form-urlencoded")
	_, _, sErr = NextLine(singleHeaderStr)
	assert.DeepEqual(t, errNeedMore, sErr)
}
