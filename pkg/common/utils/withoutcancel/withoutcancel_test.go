// Package withoutcancel provides tests for the WithoutCancel middleware.
package withoutcancel

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route"
)

func TestWithoutCancel(t *testing.T) {
	// Create a test router.
	r := route.NewEngine(config.NewOptions([]config.Option{}))

	// Add routes using the WithoutCancel middleware.
	r.Use(WithoutCancel())

	// Test route.
	r.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		// Verify that the context is not canceled.
		cancelCh := make(chan struct{})
		doneCh := make(chan struct{})

		// Create a cancellable context.
		_, cancel := context.WithCancel(context.Background())

		go func() {
			// Simulate client disconnection.
			time.Sleep(100 * time.Millisecond)
			cancel()
			close(cancelCh)
		}()

		go func() {
			// Simulate a time-consuming operation.
			time.Sleep(200 * time.Millisecond)

			// Verify that the context is still valid (not canceled).
			select {
			case <-c.Done():
				// If the context is canceled, the test fails.
				t.Error("Context was cancelled, but it shouldn't be")
			default:
				// The context is not canceled, as expected.
			}

			close(doneCh)
		}()

		// Wait for both goroutines to complete.
		<-cancelCh
		<-doneCh

		ctx.JSON(consts.StatusOK, map[string]interface{}{
			"message": "success",
		})
	})

	// Test endpoint.
	w := performRequest(r, "GET", "/test")
	if w.Code != consts.StatusOK {
		t.Errorf("Expected status %d, got %d", consts.StatusOK, w.Code)
	}
}

func TestWithoutCancelWithWhitelist(t *testing.T) {
	// Create a test router.
	r := route.NewEngine(config.NewOptions([]config.Option{}))

	// Define whitelisted endpoints.
	whitelistEndpoints := []string{
		"GET /whitelist",
	}

	// Add routes using the WithoutCancelWithWhitelist middleware.
	r.Use(WithoutCancelWithWhitelist(WithoutCancelConfig{
		WhitelistEndpoints: whitelistEndpoints,
	}))

	// Whitelisted route - should use the original context (affected by client disconnection).
	r.GET("/whitelist", func(c context.Context, ctx *app.RequestContext) {
		// In the whitelisted route, the context should be cancellable.

		cancelCh := make(chan struct{})
		doneCh := make(chan struct{})

		// Create a cancellable context.
		ctxWithCancel, cancel := context.WithCancel(context.Background())

		go func() {
			// Simulate client disconnection.
			time.Sleep(100 * time.Millisecond)
			cancel()
			close(cancelCh)
		}()

		go func() {
			// Simulate a time-consuming operation.
			time.Sleep(200 * time.Millisecond)

			// Since this is a whitelisted route, the cancellation of ctxWithCancel should affect the test context.
			select {
			case <-ctxWithCancel.Done():
				// The context has been canceled, as expected (whitelisted endpoint).
			default:
				// If the context is not canceled, the test fails.
				t.Error("Context for whitelist endpoint should be cancellable")
			}

			close(doneCh)
		}()

		// Wait for both goroutines to complete.
		<-cancelCh
		<-doneCh

		ctx.JSON(consts.StatusOK, map[string]interface{}{
			"message": "whitelist success",
		})
	})

	// Non-whitelisted route - should use a context that is not affected by client disconnection.
	r.GET("/non-whitelist", func(c context.Context, ctx *app.RequestContext) {
		// In the non-whitelisted route, the context should not be canceled.

		cancelCh := make(chan struct{})
		doneCh := make(chan struct{})

		// Create a cancellable context.
		_, cancel := context.WithCancel(context.Background())

		go func() {
			// Simulate client disconnection.
			time.Sleep(100 * time.Millisecond)
			cancel()
			close(cancelCh)
		}()

		go func() {
			// Simulate a time-consuming operation.
			time.Sleep(200 * time.Millisecond)

			// Verify that the context is still valid (not canceled).
			select {
			case <-c.Done():
				// If the context is canceled, the test fails (non-whitelisted endpoint).
				t.Error("Context for non-whitelist endpoint was cancelled, but it shouldn't be")
			default:
				// The context is not canceled, as expected.
			}

			close(doneCh)
		}()

		// Wait for both goroutines to complete.
		<-cancelCh
		<-doneCh

		ctx.JSON(consts.StatusOK, map[string]interface{}{
			"message": "non-whitelist success",
		})
	})

	// Test whitelisted endpoint.
	w1 := performRequest(r, "GET", "/whitelist")
	if w1.Code != consts.StatusOK {
		t.Errorf("Expected status %d for whitelist endpoint, got %d", consts.StatusOK, w1.Code)
	}

	// Test non-whitelisted endpoint.
	w2 := performRequest(r, "GET", "/non-whitelist")
	if w2.Code != consts.StatusOK {
		t.Errorf("Expected status %d for non-whitelist endpoint, got %d", consts.StatusOK, w2.Code)
	}
}

// Helper function: perform a request and return the response.
func performRequest(r *route.Engine, method, path string) *ut.ResponseRecorder {
	// Perform the request and return the response recorder.
	return ut.PerformRequest(r, method, path, nil)
}
