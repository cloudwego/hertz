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

package ut

import (
	"io"
	"io/ioutil"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"
)

// CreateUtRequestContext returns an app.RequestContext for testing purposes
func CreateUtRequestContext(method, url string, body *Body, headers ...Header) *app.RequestContext {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	return createUtRequestContext(engine, method, url, body, headers...)
}

func createUtRequestContext(engine *route.Engine, method, url string, body *Body, headers ...Header) *app.RequestContext {
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

	return ctx
}
