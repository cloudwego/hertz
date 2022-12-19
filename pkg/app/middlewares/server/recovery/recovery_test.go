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

package recovery

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestRecovery(t *testing.T) {
	ctx := app.NewContext(0)
	var hc app.HandlersChain
	hc = append(hc, func(c context.Context, ctx *app.RequestContext) {
		fmt.Println("this is test")
		panic("test")
	})
	ctx.SetHandlers(hc)

	Recovery()(context.Background(), ctx)

	if ctx.Response.StatusCode() != 500 {
		t.Fatalf("unexpected %v. Expecting %v", ctx.Response.StatusCode(), 500)
	}
}

func TestWithRecoveryHandler(t *testing.T) {
	ctx := app.NewContext(0)
	var hc app.HandlersChain
	hc = append(hc, func(c context.Context, ctx *app.RequestContext) {
		fmt.Println("this is test")
		panic("test")
	})
	ctx.SetHandlers(hc)

	Recovery(WithRecoveryHandler(newRecoveryHandler))(context.Background(), ctx)

	if ctx.Response.StatusCode() != consts.StatusNotImplemented {
		t.Fatalf("unexpected %v. Expecting %v", ctx.Response.StatusCode(), 501)
	}
	assert.DeepEqual(t, "{\"msg\":\"test\"}", string(ctx.Response.Body()))
}
