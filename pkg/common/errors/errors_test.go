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

package errors

import (
	"errors"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestError(t *testing.T) {
	baseError := errors.New("test error")
	err := &Error{
		Err:  baseError,
		Type: ErrorTypePrivate,
	}
	assert.DeepEqual(t, err.Error(), baseError.Error())
	assert.DeepEqual(t, map[string]interface{}{"error": baseError.Error()}, err.JSON())

	assert.DeepEqual(t, err.SetType(ErrorTypePublic), err)
	assert.DeepEqual(t, ErrorTypePublic, err.Type)

	assert.DeepEqual(t, err.SetMeta("some data"), err)
	assert.DeepEqual(t, "some data", err.Meta)
	assert.DeepEqual(t, map[string]interface{}{
		"error": baseError.Error(),
		"meta":  "some data",
	}, err.JSON())

	err.SetMeta(map[string]interface{}{ // nolint: errcheck
		"status": "200",
		"data":   "some data",
	})
	assert.DeepEqual(t, map[string]interface{}{
		"error":  baseError.Error(),
		"status": "200",
		"data":   "some data",
	}, err.JSON())

	err.SetMeta(map[string]interface{}{ // nolint: errcheck
		"error":  "custom error",
		"status": "200",
		"data":   "some data",
	})
	assert.DeepEqual(t, map[string]interface{}{
		"error":  "custom error",
		"status": "200",
		"data":   "some data",
	}, err.JSON())

	type customError struct {
		status string
		data   string
	}
	err.SetMeta(customError{status: "200", data: "other data"}) // nolint: errcheck
	assert.DeepEqual(t, customError{status: "200", data: "other data"}, err.JSON())
}

func TestErrorSlice(t *testing.T) {
	errs := ErrorChain{
		{Err: errors.New("first"), Type: ErrorTypePrivate},
		{Err: errors.New("second"), Type: ErrorTypePrivate, Meta: "some data"},
		{Err: errors.New("third"), Type: ErrorTypePublic, Meta: map[string]interface{}{"status": "400"}},
	}

	assert.DeepEqual(t, errs, errs.ByType(ErrorTypeAny))
	assert.DeepEqual(t, "third", errs.Last().Error())
	assert.DeepEqual(t, []string{"first", "second", "third"}, errs.Errors())
	assert.DeepEqual(t, []string{"third"}, errs.ByType(ErrorTypePublic).Errors())
	assert.DeepEqual(t, []string{"first", "second"}, errs.ByType(ErrorTypePrivate).Errors())
	assert.DeepEqual(t, []string{"first", "second", "third"}, errs.ByType(ErrorTypePublic|ErrorTypePrivate).Errors())
	assert.DeepEqual(t, "", errs.ByType(ErrorTypeBind).String())
	assert.DeepEqual(t, `Error #01: first
Error #02: second
     Meta: some data
Error #03: third
     Meta: map[status:400]
`, errs.String())
	assert.DeepEqual(t, []interface{}{
		map[string]interface{}{"error": "first"},
		map[string]interface{}{"error": "second", "meta": "some data"},
		map[string]interface{}{"error": "third", "status": "400"},
	}, errs.JSON())
	errs = ErrorChain{
		{Err: errors.New("first"), Type: ErrorTypePrivate},
	}

	assert.DeepEqual(t, map[string]interface{}{"error": "first"}, errs.JSON())
	errs = ErrorChain{}
	assert.DeepEqual(t, true, errs.Last() == nil)
	assert.Nil(t, errs.JSON())
	assert.DeepEqual(t, "", errs.String())
}

func TestErrorFormat(t *testing.T) {
	err := Newf(ErrorTypeAny, nil, "caused by %s", "reason")
	assert.DeepEqual(t, New(errors.New("caused by reason"), ErrorTypeAny, nil), err)
	publicErr := NewPublicf("caused by %s", "reason")
	assert.DeepEqual(t, New(errors.New("caused by reason"), ErrorTypePublic, nil), publicErr)
	privateErr := NewPrivatef("caused by %s", "reason")
	assert.DeepEqual(t, New(errors.New("caused by reason"), ErrorTypePrivate, nil), privateErr)
}
