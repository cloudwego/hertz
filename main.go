package main

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func main() {
	// 初始化 Hertz 服务器，默认监听 8888 端口
	h := server.Default()

	// 原有测试接口
	h.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, map[string]string{"message": "pong"})
	})


	h.GET("/api/v1/stream/monitor", func(c context.Context, ctx *app.RequestContext) {
		// 这里模拟返回一个流的状态信息
		ctx.JSON(consts.StatusOK, map[string]interface{}{
			"code":    200,
			"message": "查询成功",
			"data": map[string]interface{}{
				"stream_name": "live/test",
				"clients":     10,
				"bitrate":     "2000kbps",
				"status":      "active",
			},
		})
	})

	// 启动服务
	h.Spin()
}