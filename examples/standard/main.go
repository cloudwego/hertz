/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

type Test struct {
	A string
	B string
}

func main() {
	h := server.Default()
	h.StaticFS("/", &app.FS{Root: "./", GenerateIndexPages: true})

	h.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, utils.H{"ping": "pong"})
	})

	h.GET("/json", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, &Test{
			A: "aaa",
			B: "bbb",
		})
	})

	h.GET("/redirect", func(c context.Context, ctx *app.RequestContext) {
		ctx.Redirect(consts.StatusMovedPermanently, []byte("http://www.google.com/"))
	})

	h.GET("/flush/chunk", func(c context.Context, ctx *app.RequestContext) {
		// Hijack the writer of response
		ctx.Response.HijackWriter(resp.NewChunkedBodyWriter(&ctx.Response, ctx.GetWriter()))

		for i := 0; i < 10; i++ {
			ctx.Write([]byte(fmt.Sprintf("chunk %d: %s", i, strings.Repeat("hi~", i)))) // nolint: errcheck
			ctx.Flush()                                                                 // nolint: errcheck
			time.Sleep(200 * time.Millisecond)
		}
	})

	v1 := h.Group("/v1")
	{
		v1.GET("/hello/:name", func(c context.Context, ctx *app.RequestContext) {
			fmt.Fprintf(ctx, "Hi %s, this is the response from Hertz.\n", ctx.Param("name"))
		})
	}

	h.Spin()
}
