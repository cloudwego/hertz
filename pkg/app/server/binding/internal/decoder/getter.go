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
 * MIT License
 *
 * Copyright (c) 2019-present Fenny and Contributors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2023 CloudWeGo Authors
 */

package decoder

import (
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type getter func(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exist bool)

func path(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exist bool) {
	if params != nil {
		ret, exist = params.Get(key)
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = defaultValue[0]
	}
	return ret, exist
}

func postForm(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exist bool) {
	if ret, exist = req.PostArgs().PeekExists(key); exist {
		return
	}

	mf, err := req.MultipartForm()
	if err == nil && mf.Value != nil {
		for k, v := range mf.Value {
			if k == key && len(v) > 0 {
				ret = v[0]
			}
		}
	}

	if len(ret) != 0 {
		return ret, true
	}
	if ret, exist = req.URI().QueryArgs().PeekExists(key); exist {
		return
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = defaultValue[0]
	}

	return ret, false
}

func query(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exist bool) {
	if ret, exist = req.URI().QueryArgs().PeekExists(key); exist {
		return
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = defaultValue[0]
	}

	return
}

func cookie(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exist bool) {
	if val := req.Header.Cookie(key); val != nil {
		ret = string(val)
		return ret, true
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = defaultValue[0]
	}

	return ret, false
}

func header(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exist bool) {
	if val := req.Header.Peek(key); val != nil {
		ret = string(val)
		return ret, true
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = defaultValue[0]
	}

	return ret, false
}

func rawBody(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exist bool) {
	exist = false
	if req.Header.ContentLength() > 0 {
		ret = string(req.Body())
		exist = true
	}
	return
}
