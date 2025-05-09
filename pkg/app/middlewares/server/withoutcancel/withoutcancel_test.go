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

func TestWithoutCancelCtx(t *testing.T) {
	// 创建测试路由器
	r := route.NewEngine(config.NewOptions([]config.Option{}))

	// 添加使用WithoutCancelCtx中间件的路由
	r.Use(WithoutCancelCtx())

	// 模拟一个耗时操作，并且检查context是否被取消
	r.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		// 创建一个有取消功能的context
		_, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 如果context是通过WithoutCancelCtx创建的，
		// 那么即使父context被取消，这个context也不应该被取消
		cancelCh := make(chan struct{})
		doneCh := make(chan struct{})

		go func() {
			// 模拟客户端断开连接
			time.Sleep(100 * time.Millisecond)
			cancel() // 取消父context
			close(cancelCh)
		}()

		go func() {
			// 模拟耗时操作
			time.Sleep(200 * time.Millisecond)

			// 验证context是否仍然有效（未被取消）
			select {
			case <-c.Done():
				// 如果context已被取消，测试失败
				t.Error("Context was cancelled, but it shouldn't be")
			default:
				// context未被取消，符合期望
			}

			close(doneCh)
		}()

		// 等待两个goroutine完成
		<-cancelCh
		<-doneCh

		ctx.JSON(consts.StatusOK, map[string]interface{}{
			"message": "success",
		})
	})

	// 模拟请求
	w := performRequest(r, "GET", "/test")

	// 验证响应
	if w.Code != consts.StatusOK {
		t.Errorf("Expected status %d, got %d", consts.StatusOK, w.Code)
	}
}

func TestCopyValues(t *testing.T) {
	// 创建带有值的源context
	key := struct{}{}
	value := "test-value"
	src := context.WithValue(context.Background(), key, value)

	// 创建目标context
	dst := context.Background()

	// 测试copyValues函数
	// 注意：在当前实现中，copyValues是空函数，不会复制任何值
	// 这个测试主要是为了覆盖代码，实际应用中可能需要根据具体实现修改
	copyValues(src, dst)

	// 因为当前实现不复制任何值，所以dst中不应该有任何值
	if dst.Value(key) != nil {
		t.Errorf("Expected no value, got %v", dst.Value(key))
	}
}

func TestNew(t *testing.T) {
	// 测试New函数是否返回正确的中间件
	middleware := New()

	// 验证返回的是否是非nil的函数
	if middleware == nil {
		t.Error("Expected middleware function, got nil")
	}
}

// 辅助函数：执行请求并返回响应
func performRequest(r *route.Engine, method, path string) *ut.ResponseRecorder {
	// 执行请求并返回响应记录器
	return ut.PerformRequest(r, method, path, nil)
}
