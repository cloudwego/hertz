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

// Package ut provides a convenient way to write unit test for the business logic.
package ut

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"
)

// Header is a key-value pair indicating one http header
type Header struct {
	Key   string
	Value string
}

// Body is for setting Request.Body
type Body struct {
	Body io.Reader
	Len  int
}

// PerformRequest send a constructed request to given engine without network transporting
//
// # Url can be a standard relative URI or a simple absolute path
//
// If engine.streamRequestBody is true, it sets body as bodyStream
// if not, it sets body as bodyBytes
//
// ResponseRecorder returned are flushed, which means its StatusCode is always set (default 200)
//
// See ./request_test.go for more examples
func PerformRequest(engine *route.Engine, method, url string, body *Body, headers ...Header) *ResponseRecorder {
	ctx := engine.NewContext()

	var r *protocol.Request
	if body != nil && body.Body != nil {
		r = protocol.NewRequest(method, url, body.Body)
		r.CopyTo(&ctx.Request)
		if engine.IsStreamRequestBody() || body.Len == -1 {
			ctx.Request.SetBodyStream(body.Body, body.Len)
		} else {
			buf, err := ioutil.ReadAll(&io.LimitedReader{R: body.Body, N: int64(body.Len)})
			ctx.Request.SetBody(buf)
			if err != nil && err != io.EOF {
				panic(err)
			}
		}
	} else {
		r = protocol.NewRequest(method, url, nil)
		r.CopyTo(&ctx.Request)
	}

	for _, v := range headers {
		if ctx.Request.Header.Get(v.Key) != "" {
			ctx.Request.Header.Add(v.Key, v.Value)
		} else {
			ctx.Request.Header.Set(v.Key, v.Value)
		}
	}

	engine.ServeHTTP(context.Background(), ctx)

	w := NewRecorder()
	h := w.Header()
	ctx.Response.Header.CopyTo(h)

	w.WriteHeader(ctx.Response.StatusCode())
	w.Write(ctx.Response.Body())
	w.Flush()
	return w
}
