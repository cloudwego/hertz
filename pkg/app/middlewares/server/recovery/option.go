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

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type (
	options struct {
		recoveryHandler func(c context.Context, ctx *app.RequestContext, err interface{}, stack []byte)
	}

	Option func(o *options)
)

func defaultRecoveryHandler(c context.Context, ctx *app.RequestContext, err interface{}, stack []byte) {
	hlog.SystemLogger().CtxErrorf(c, "[Recovery] err=%v\nstack=%s", err, stack)
	ctx.AbortWithStatus(consts.StatusInternalServerError)
}

func newOptions(opts ...Option) *options {
	cfg := &options{
		recoveryHandler: defaultRecoveryHandler,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

func WithRecoveryHandler(f func(c context.Context, ctx *app.RequestContext, err interface{}, stack []byte)) Option {
	return func(o *options) {
		o.recoveryHandler = f
	}
}
