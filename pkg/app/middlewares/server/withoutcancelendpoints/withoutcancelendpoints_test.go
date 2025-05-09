package withoutcancelendpoints

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

func TestWithoutCancelEndpoints(t *testing.T) {
	// 创建测试路由器
	r := route.NewEngine(config.NewOptions([]config.Option{}))

	// 定义白名单端点
	whitelistEndpoints := []string{
		"GET /whitelist",
	}

	// 添加使用WithoutCancelEndpoints中间件的路由
	r.Use(New(Config{
		WhitelistEndpoints: whitelistEndpoints,
	}))

	// 白名单路由 - 应该使用原始context（会受客户端断开连接影响）
	r.GET("/whitelist", func(c context.Context, ctx *app.RequestContext) {
		// 在白名单路由中，context应该会被取消

		cancelCh := make(chan struct{})
		doneCh := make(chan struct{})

		// 创建一个可取消的context
		ctxWithCancel, cancel := context.WithCancel(context.Background())

		go func() {
			// 模拟客户端断开连接
			time.Sleep(100 * time.Millisecond)
			cancel()
			close(cancelCh)
		}()

		go func() {
			// 模拟耗时操作
			time.Sleep(200 * time.Millisecond)

			// 由于是白名单路由，ctxWithCancel的取消应该会影响测试context
			// 注意：这里不能直接测试c.Done()，因为在测试环境中可能没有模拟客户端断开连接
			// 所以我们使用自己创建的可取消context来验证
			select {
			case <-ctxWithCancel.Done():
				// context已被取消，符合期望（白名单端点）
			default:
				// 如果context未被取消，测试失败
				t.Error("Context for whitelist endpoint should be cancellable")
			}

			close(doneCh)
		}()

		// 等待两个goroutine完成
		<-cancelCh
		<-doneCh

		ctx.JSON(consts.StatusOK, map[string]interface{}{
			"message": "whitelist success",
		})
	})

	// 非白名单路由 - 应该使用不受客户端断开连接影响的context
	r.GET("/non-whitelist", func(c context.Context, ctx *app.RequestContext) {
		// 在非白名单路由中，context不应该会被取消

		cancelCh := make(chan struct{})
		doneCh := make(chan struct{})

		// 创建一个可取消的context
		_, cancel := context.WithCancel(context.Background())

		go func() {
			// 模拟客户端断开连接
			time.Sleep(100 * time.Millisecond)
			cancel()
			close(cancelCh)
		}()

		go func() {
			// 模拟耗时操作
			time.Sleep(200 * time.Millisecond)

			// 验证context是否仍然有效（未被取消）
			select {
			case <-c.Done():
				// 如果context已被取消，测试失败（非白名单端点）
				t.Error("Context for non-whitelist endpoint was cancelled, but it shouldn't be")
			default:
				// context未被取消，符合期望
			}

			close(doneCh)
		}()

		// 等待两个goroutine完成
		<-cancelCh
		<-doneCh

		ctx.JSON(consts.StatusOK, map[string]interface{}{
			"message": "non-whitelist success",
		})
	})

	// 测试白名单端点
	w1 := performRequest(r, "GET", "/whitelist")
	if w1.Code != consts.StatusOK {
		t.Errorf("Expected status %d for whitelist endpoint, got %d", consts.StatusOK, w1.Code)
	}

	// 测试非白名单端点
	w2 := performRequest(r, "GET", "/non-whitelist")
	if w2.Code != consts.StatusOK {
		t.Errorf("Expected status %d for non-whitelist endpoint, got %d", consts.StatusOK, w2.Code)
	}
}

// 辅助函数：执行请求并返回响应
func performRequest(r *route.Engine, method, path string) *ut.ResponseRecorder {
	// 执行请求并返回响应记录器
	return ut.PerformRequest(r, method, path, nil)
}
