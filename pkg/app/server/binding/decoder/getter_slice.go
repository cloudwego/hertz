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
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package decoder

import (
	"net/http"
	"net/url"

	path1 "github.com/cloudwego/hertz/pkg/app/server/binding/path"
)

type sliceGetter func(req *bindRequest, params path1.PathParam, key string, defaultValue ...string) (ret []string)

func pathSlice(req *bindRequest, params path1.PathParam, key string, defaultValue ...string) (ret []string) {
	var value string
	if params != nil {
		value, _ = params.Get(key)
	}

	if len(value) == 0 && len(defaultValue) != 0 {
		value = defaultValue[0]
	}
	if len(value) != 0 {
		ret = append(ret, value)
	}

	return
}

func postFormSlice(req *bindRequest, params path1.PathParam, key string, defaultValue ...string) (ret []string) {
	if req.Form == nil {
		req.Form = make(url.Values)
		req.Req.PostArgs().VisitAll(func(formKey, value []byte) {
			keyStr := string(formKey)
			values := req.Form[keyStr]
			values = append(values, string(value))
			req.Form[keyStr] = values
		})
	}
	ret = req.Form[key]
	if len(ret) > 0 {
		return
	}

	if req.MultipartForm == nil {
		req.MultipartForm = make(url.Values)
		mf, err := req.Req.MultipartForm()
		if err == nil && mf.Value != nil {
			for k, v := range mf.Value {
				if len(v) > 0 {
					req.MultipartForm[k] = v
				}
			}
		}
	}
	ret = req.MultipartForm[key]
	if len(ret) > 0 {
		return
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func querySlice(req *bindRequest, params path1.PathParam, key string, defaultValue ...string) (ret []string) {
	if req.Query == nil {
		req.Query = make(url.Values, 100)
		req.Req.URI().QueryArgs().VisitAll(func(queryKey, value []byte) {
			keyStr := string(queryKey)
			values := req.Query[keyStr]
			values = append(values, string(value))
			req.Query[keyStr] = values
		})
	}

	ret = req.Query[key]
	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func cookieSlice(req *bindRequest, params path1.PathParam, key string, defaultValue ...string) (ret []string) {
	if len(req.Cookie) == 0 {
		req.Req.Header.VisitAllCookie(func(cookieKey, value []byte) {
			req.Cookie = append(req.Cookie, &http.Cookie{
				Name:  string(cookieKey),
				Value: string(value),
			})
		})
	}
	for _, c := range req.Cookie {
		if c.Name == key {
			ret = append(ret, c.Value)
		}
	}
	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func headerSlice(req *bindRequest, params path1.PathParam, key string, defaultValue ...string) (ret []string) {
	if req.Header == nil {
		req.Header = make(http.Header)
		req.Req.Header.VisitAll(func(headerKey, value []byte) {
			keyStr := string(headerKey)
			values := req.Header[keyStr]
			values = append(values, string(value))
			req.Header[keyStr] = values
		})
	}

	ret = req.Header[key]
	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func rawBodySlice(req *bindRequest, params path1.PathParam, key string, defaultValue ...string) (ret []string) {
	if req.Req.Header.ContentLength() > 0 {
		ret = append(ret, string(req.Req.Body()))
	}
	return
}
