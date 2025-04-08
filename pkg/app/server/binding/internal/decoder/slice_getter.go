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

type sliceGetter func(req *protocol.Request, params param.Params, key string) (ret []string)

var tag2sliceGetter = map[string]sliceGetter{
	pathTag:   pathSlice,
	formTag:   formSlice,
	queryTag:  querySlice,
	cookieTag: cookieSlice,
	headerTag: headerSlice,
}

func pathSlice(req *protocol.Request, params param.Params, key string) (ret []string) {
	if params == nil {
		return
	}
	value, ok := params.Get(key)
	if ok {
		return []string{value}
	}
	return
}

func formSlice(req *protocol.Request, params param.Params, key string) (ret []string) {
	req.PostArgs().VisitAll(func(formKey, value []byte) {
		if bytesconv.B2s(formKey) == key {
			ret = append(ret, string(value))
		}
	})
	if len(ret) > 0 {
		return
	}

	mf, err := req.MultipartForm()
	if err == nil && mf.Value != nil {
		for k, v := range mf.Value {
			if k == key && len(v) > 0 {
				ret = append(ret, v...)
			}
		}
	}
	if len(ret) > 0 {
		return
	}

	req.URI().QueryArgs().VisitAll(func(queryKey, value []byte) {
		if bytesconv.B2s(queryKey) == key {
			ret = append(ret, string(value))
		}
	})
	return
}

func querySlice(req *protocol.Request, params param.Params, key string) (ret []string) {
	req.URI().QueryArgs().VisitAll(func(queryKey, value []byte) {
		if key == bytesconv.B2s(queryKey) {
			ret = append(ret, string(value))
		}
	})
	return
}

func cookieSlice(req *protocol.Request, params param.Params, key string) (ret []string) {
	req.Header.VisitAllCookie(func(cookieKey, value []byte) {
		if bytesconv.B2s(cookieKey) == key {
			ret = append(ret, string(value))
		}
	})
	return
}

func headerSlice(req *protocol.Request, params param.Params, key string) (ret []string) {
	req.Header.VisitAll(func(headerKey, value []byte) {
		if bytesconv.B2s(headerKey) == key {
			ret = append(ret, string(value))
		}
	})
	return
}
