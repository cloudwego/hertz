package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils/withoutcancel"
)

func main() {
	h := server.Default(
		// Enable client disconnection detection.
		server.WithSenseClientDisconnection(true),
	)

	// Route 1: Normal route, will detect client disconnection.
	// When the client disconnects, the handler will receive a ctx.Done() signal.
	h.GET("/api/normal", func(c context.Context, ctx *app.RequestContext) {
		hlog.Info("Start processing /api/normal request")

		// Simulate a time-consuming operation.
		select {
		case <-time.After(10 * time.Second):
			hlog.Info("/api/normal: Operation completed")
			ctx.String(200, "Operation completed")
		case <-c.Done():
			// This will be triggered when the client disconnects.
			hlog.Warn("/api/normal: Client has disconnected, operation canceled")
			return
		}
	})

	// Route 2: Using WithoutCancel middleware.
	// Even if the client disconnects, the handler will continue executing until completion.
	h.GET("/api/withoutcancel", withoutcancel.WithoutCancel(), func(c context.Context, ctx *app.RequestContext) {
		hlog.Info("Start processing /api/withoutcancel request")

		// Simulate a time-consuming operation.
		select {
		case <-time.After(10 * time.Second):
			hlog.Info("/api/withoutcancel: Operation completed")
			ctx.String(200, "Operation completed")
		case <-c.Done():
			// This branch will never be triggered because the WithoutCancel middleware is used.
			hlog.Warn("/api/withoutcancel: Client has disconnected, operation canceled")
			return
		}
	})

	// Create whitelist endpoint configuration.
	whitelistConfig := withoutcancel.WithoutCancelConfig{
		WhitelistEndpoints: []string{
			"GET /api/stream",
			"GET /api/websocket",
		},
	}

	// Route group: Using WithoutCancelWithWhitelist middleware.
	// Only endpoints in the whitelist will detect client disconnection; others will not be affected.
	apiGroup := h.Group("/api", withoutcancel.WithoutCancelWithWhitelist(whitelistConfig))

	// This endpoint is in the whitelist and will detect client disconnection.
	apiGroup.GET("/stream", func(c context.Context, ctx *app.RequestContext) {
		hlog.Info("Start processing /api/stream request")

		// Simulate a scenario that requires detecting client disconnection, such as SSE or WebSocket.
		select {
		case <-time.After(10 * time.Second):
			hlog.Info("/api/stream: Operation completed")
			ctx.String(200, "Operation completed")
		case <-c.Done():
			// This will be triggered when the client disconnects.
			hlog.Warn("/api/stream: Client has disconnected, operation canceled")
			return
		}
	})

	// This endpoint is not in the whitelist and will not detect client disconnection.
	apiGroup.GET("/longprocess", func(c context.Context, ctx *app.RequestContext) {
		hlog.Info("Start processing /api/longprocess request")

		// Simulate a time-consuming operation.
		select {
		case <-time.After(10 * time.Second):
			hlog.Info("/api/longprocess: Operation completed")
			ctx.String(200, "Operation completed")
		case <-c.Done():
			// This branch will never be triggered because this endpoint is not in the whitelist.
			hlog.Warn("/api/longprocess: Client has disconnected, operation canceled")
			return
		}
	})

	// Start the server.
	addr := "127.0.0.1:8888"
	hlog.Infof("Server started at %s", addr)
	hlog.Infof("Access the following URLs for testing:")
	hlog.Infof("  - http://%s/api/normal (will detect client disconnection)", addr)
	hlog.Infof("  - http://%s/api/withoutcancel (will not detect client disconnection)", addr)
	hlog.Infof("  - http://%s/api/stream (whitelisted endpoint, will detect client disconnection)", addr)
	hlog.Infof("  - http://%s/api/longprocess (non-whitelisted endpoint, will not detect client disconnection)", addr)

	fmt.Println("Tip: After opening the URL, close the browser or cancel the request within 10 seconds to observe the server log output.")
	h.Spin()
}
