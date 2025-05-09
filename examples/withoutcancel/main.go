package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/withoutcancel"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/withoutcancelendpoints"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

func main() {
	h := server.Default(
		// 开启客户端断开连接检测功能
		server.WithSenseClientDisconnection(true),
	)

	// 路由1: 普通路由，会感知客户端断开连接
	// 当客户端断开连接时，处理函数会收到ctx.Done()信号
	h.GET("/api/normal", func(c context.Context, ctx *app.RequestContext) {
		hlog.Info("开始处理 /api/normal 请求")

		// 模拟耗时操作
		select {
		case <-time.After(10 * time.Second):
			hlog.Info("/api/normal: 操作完成")
			ctx.String(200, "操作已完成")
		case <-c.Done():
			// 当客户端断开连接时，这里会被触发
			hlog.Warn("/api/normal: 客户端已断开连接，操作被取消")
			return
		}
	})

	// 路由2: 使用WithoutCancel中间件
	// 即使客户端断开连接，处理函数也会继续执行直到完成
	h.GET("/api/withoutcancel", withoutcancel.New(), func(c context.Context, ctx *app.RequestContext) {
		hlog.Info("开始处理 /api/withoutcancel 请求")

		// 模拟耗时操作
		select {
		case <-time.After(10 * time.Second):
			hlog.Info("/api/withoutcancel: 操作完成")
			ctx.String(200, "操作已完成")
		case <-c.Done():
			// 这个分支永远不会被触发，因为使用了WithoutCancel中间件
			hlog.Warn("/api/withoutcancel: 客户端已断开连接，操作被取消")
			return
		}
	})

	// 创建白名单端点配置
	whitelistConfig := withoutcancelendpoints.Config{
		WhitelistEndpoints: []string{
			"GET /api/stream",
			"GET /api/websocket",
		},
	}

	// 路由组: 使用WithoutCancelEndpoints中间件
	// 只有白名单中的端点会感知客户端断开连接，其他端点不受影响
	apiGroup := h.Group("/api", withoutcancelendpoints.New(whitelistConfig))

	// 这个端点在白名单中，会感知客户端断开连接
	apiGroup.GET("/stream", func(c context.Context, ctx *app.RequestContext) {
		hlog.Info("开始处理 /api/stream 请求")

		// 模拟SSE或Websocket等需要感知客户端断开连接的场景
		select {
		case <-time.After(10 * time.Second):
			hlog.Info("/api/stream: 操作完成")
			ctx.String(200, "操作已完成")
		case <-c.Done():
			// 当客户端断开连接时，这里会被触发
			hlog.Warn("/api/stream: 客户端已断开连接，操作被取消")
			return
		}
	})

	// 这个端点不在白名单中，不会感知客户端断开连接
	apiGroup.GET("/longprocess", func(c context.Context, ctx *app.RequestContext) {
		hlog.Info("开始处理 /api/longprocess 请求")

		// 模拟耗时操作
		select {
		case <-time.After(10 * time.Second):
			hlog.Info("/api/longprocess: 操作完成")
			ctx.String(200, "操作已完成")
		case <-c.Done():
			// 这个分支永远不会被触发，因为该端点不在白名单中
			hlog.Warn("/api/longprocess: 客户端已断开连接，操作被取消")
			return
		}
	})

	// 启动服务器
	addr := "127.0.0.1:8888"
	hlog.Infof("服务器启动在 %s", addr)
	hlog.Infof("访问以下URL测试:")
	hlog.Infof("  - http://%s/api/normal (会感知客户端断开连接)", addr)
	hlog.Infof("  - http://%s/api/withoutcancel (不会感知客户端断开连接)", addr)
	hlog.Infof("  - http://%s/api/stream (白名单端点，会感知客户端断开连接)", addr)
	hlog.Infof("  - http://%s/api/longprocess (非白名单端点，不会感知客户端断开连接)", addr)

	fmt.Println("提示: 打开URL后，在10秒内关闭浏览器或取消请求，观察服务器日志输出")
	h.Spin()
}
