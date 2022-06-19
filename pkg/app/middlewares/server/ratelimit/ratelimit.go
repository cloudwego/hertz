package ratelimit

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	ErrLimit = "Hertz Rate limiting"
)

/*
	CPU sampling algorithm using BBR
*/
func Ratelimit(opts ...options) app.HandlerFunc {
	limiter := NewLimiter()
	return func(c context.Context, ctx *app.RequestContext) {
		done, err := limiter.Allow()
		if err != nil {
			ctx.String(consts.StatusTooManyRequests, ErrLimit)
			return
		}
		ctx.Next(c)
		done()
	}
}
