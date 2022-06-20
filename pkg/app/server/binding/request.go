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

package binding

import (
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/bytedance/go-tagexpr/v2/binding"
	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func wrapRequest(req *protocol.Request) binding.Request {
	r := &bindRequest{
		req: req,
	}
	return r
}

type bindRequest struct {
	req *protocol.Request
}

func (r *bindRequest) GetQuery() url.Values {
	queryMap := make(url.Values)
	r.req.URI().QueryArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := queryMap[keyStr]
		values = append(values, string(value))
		queryMap[keyStr] = values
	})

	return queryMap
}

func (r *bindRequest) GetPostForm() (url.Values, error) {
	postMap := make(url.Values)
	r.req.PostArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := postMap[keyStr]
		values = append(values, string(value))
		postMap[keyStr] = values
	})
	mf, err := r.req.MultipartForm()
	if err == nil {
		for k, v := range mf.Value {
			if len(v) > 0 {
				postMap[k] = v
			}
		}
	}

	return postMap, nil
}

func (r *bindRequest) GetForm() (url.Values, error) {
	formMap := make(url.Values)
	r.req.URI().QueryArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := formMap[keyStr]
		values = append(values, string(value))
		formMap[keyStr] = values
	})
	r.req.PostArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := formMap[keyStr]
		values = append(values, string(value))
		formMap[keyStr] = values
	})

	return formMap, nil
}

func (r *bindRequest) GetCookies() []*http.Cookie {
	var cookies []*http.Cookie
	r.req.Header.VisitAllCookie(func(key, value []byte) {
		cookies = append(cookies, &http.Cookie{
			Name:  string(key),
			Value: string(value),
		})
	})

	return cookies
}

func (r *bindRequest) GetHeader() http.Header {
	header := make(http.Header)
	r.req.Header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := header[keyStr]
		values = append(values, string(value))
		header[keyStr] = values
	})

	return header
}

func (r *bindRequest) GetMethod() string {
	return bytesconv.B2s(r.req.Method())
}

func (r *bindRequest) GetContentType() string {
	return bytesconv.B2s(r.req.Header.ContentType())
}

func (r *bindRequest) GetBody() ([]byte, error) {
	return r.req.Body(), nil
}

func (r *bindRequest) GetFileHeaders() (map[string][]*multipart.FileHeader, error) {
	files := make(map[string][]*multipart.FileHeader)
	mf, err := r.req.MultipartForm()
	if err == nil {
		for k, v := range mf.File {
			if len(v) > 0 {
				files[k] = v
			}
		}
	}

	return files, nil
}
