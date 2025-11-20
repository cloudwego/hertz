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
	ctx := createUtRequestContext(engine, method, url, body, headers...)
	engine.ServeHTTP(context.Background(), ctx)

	w := NewRecorder()
	h := w.Header()
	ctx.Response.Header.CopyTo(h)

	w.WriteHeader(ctx.Response.StatusCode())
	w.Write(ctx.Response.Body())
	w.Flush()
	return w
}
