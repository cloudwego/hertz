// Package withoutcancel provides a middleware that allows specific routes to continue executing even if the client disconnects.
package withoutcancel

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

// WithoutCancelConfig is used to configure the WithoutCancel middleware.
type WithoutCancelConfig struct {
	// WhitelistEndpoints is a list of endpoints that need to retain the client disconnect detection feature.
	// These endpoints will use the original context, allowing them to be aware of client disconnection and cancel operations.
	// Format: "GET /api/v1/stream", "POST /api/v1/websocket", etc.
	WhitelistEndpoints []string
}

// WithoutCancel creates a context for all routes that is not affected by client connection interruptions.
// Even if the client disconnects, the handler will continue executing until completion.
func WithoutCancel() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		// Create a new context using context.Background() that will not be canceled.
		ctx.Next(context.Background())
	}
}

// WithoutCancelWithWhitelist creates a middleware that supports whitelisted endpoint configuration.
// This middleware will use a context that is not affected by client disconnection for endpoints not in the whitelist.
func WithoutCancelWithWhitelist(config WithoutCancelConfig) app.HandlerFunc {
	// Convert the whitelist endpoints to a map for quick lookup.
	whitelistMap := make(map[string]bool)
	for _, endpoint := range config.WhitelistEndpoints {
		whitelistMap[endpoint] = true
	}

	return func(c context.Context, ctx *app.RequestContext) {
		// Build the identifier for the current request endpoint.
		currentEndpoint := string(ctx.Request.Method()) + " " + string(ctx.Request.URI().Path())

		// If the current endpoint is in the whitelist, continue processing with the original context.
		if whitelistMap[currentEndpoint] {
			ctx.Next(c)
			return
		}

		// For non-whitelisted endpoints, use context.Background() directly, unaffected by client disconnection.
		ctx.Next(context.Background())
	}
}
