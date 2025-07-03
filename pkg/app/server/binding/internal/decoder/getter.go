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
 */

package decoder

import (
	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type getter func(fi *fieldInfo, req *protocol.Request, params param.Params, key string) (ret string, exist bool)

var tag2getter = map[string]getter{
	pathTag:    path,
	formTag:    form,
	queryTag:   query,
	cookieTag:  cookie,
	headerTag:  header,
	rawBodyTag: rawBody,
}

func unsafeStr(fi *fieldInfo, v []byte) string {
	if fi.unsafestr {
		return bytesconv.B2s(v)
	}
	return string(v)
}

func path(fi *fieldInfo, req *protocol.Request, params param.Params, key string) (string, bool) {
	if params == nil {
		return "", false
	}
	return params.Get(key)
}

func form(fi *fieldInfo, req *protocol.Request, params param.Params, key string) (string, bool) {
	if v := req.PostArgs().Peek(key); v != nil {
		return unsafeStr(fi, v), true
	}
	mf, err := req.MultipartForm()
	if err == nil && mf.Value != nil {
		for k, v := range mf.Value {
			if k == key && len(v) > 0 {
				return v[0], true
			}
		}
	}
	if v := req.URI().QueryArgs().Peek(key); v != nil {
		return unsafeStr(fi, v), true
	}
	return "", false
}

func query(fi *fieldInfo, req *protocol.Request, params param.Params, key string) (string, bool) {
	if v := req.URI().QueryArgs().Peek(key); v != nil {
		return unsafeStr(fi, v), true
	}
	return "", false
}

func cookie(fi *fieldInfo, req *protocol.Request, params param.Params, key string) (string, bool) {
	if v := req.Header.Cookie(key); v != nil {
		return unsafeStr(fi, v), true
	}
	return "", false
}

func header(fi *fieldInfo, req *protocol.Request, params param.Params, key string) (string, bool) {
	if v := req.Header.Peek(key); v != nil {
		return unsafeStr(fi, v), true
	}
	return "", false
}

func rawBody(fi *fieldInfo, req *protocol.Request, params param.Params, key string) (string, bool) {
	if req.Header.ContentLength() > 0 {
		return unsafeStr(fi, req.Body()), true
	}
	return "", false
}
