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

 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package basic_auth

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestPairs(t *testing.T) {
	t1 := Accounts{"test1": "value1"}
	t2 := Accounts{"test2": "value2"}
	p1 := constructPairs(t1)
	p2 := constructPairs(t2)

	u1, ok1 := p1.findValue("Basic dGVzdDE6dmFsdWUx")
	u2, ok2 := p2.findValue("Basic dGVzdDI6dmFsdWUy")
	_, ok3 := p1.findValue("bad header")
	_, ok4 := p2.findValue("bad header")
	assert.True(t, ok1)
	assert.DeepEqual(t, "test1", u1)
	assert.True(t, ok2)
	assert.DeepEqual(t, "test2", u2)
	assert.False(t, ok3)
	assert.False(t, ok4)
}

func TestBasicAuth(t *testing.T) {
	userName1 := "user1"
	password1 := "value1"
	userName2 := "user2"
	password2 := "value2"

	c1 := app.RequestContext{}
	encodeStr := "Basic " + base64.StdEncoding.EncodeToString(bytesconv.S2b(userName1+":"+password1))
	c1.Request.Header.Add("Authorization", encodeStr)

	t1 := Accounts{userName1: password1}
	handler := BasicAuth(t1)
	handler(context.TODO(), &c1)

	user, ok := c1.Get("user")
	assert.DeepEqual(t, userName1, user)
	assert.True(t, ok)

	c2 := app.RequestContext{}
	encodeStr = "Basic " + base64.StdEncoding.EncodeToString(bytesconv.S2b(userName2+":"+password2))
	c2.Request.Header.Add("Authorization", encodeStr)

	handler(context.TODO(), &c2)

	user, ok = c2.Get("user")
	assert.Nil(t, user)
	assert.False(t, ok)
}
