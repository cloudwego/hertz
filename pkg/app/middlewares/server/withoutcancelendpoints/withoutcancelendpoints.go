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

// Package withoutcancelendpoints 提供一个中间件，允许通过白名单端点的WithoutCancel功能，该功能将会使非白名单的路由使用不会因为客户端连接关闭而取消的context
package withoutcancelendpoints

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

// Config 用于配置WithoutCancelEndpoints中间件
type Config struct {
	// WhitelistEndpoints 是需要保留客户端断开连接检测功能的端点列表
	// 这些端点将会使用原始的context，当客户端断开连接时可以感知到并取消操作
	// 格式为: "GET /api/v1/stream", "POST /api/v1/websocket" 等
	WhitelistEndpoints []string
}

// New 创建一个新的WithoutCancelEndpoints中间件
// 该中间件会对不在白名单中的端点使用不会被客户端断开连接影响的context
func New(config Config) app.HandlerFunc {
	// 将白名单端点转换为map，以便快速查找
	whitelistMap := make(map[string]bool)
	for _, endpoint := range config.WhitelistEndpoints {
		whitelistMap[endpoint] = true
	}

	return func(c context.Context, ctx *app.RequestContext) {
		// 构建当前请求的端点标识
		currentEndpoint := string(ctx.Request.Method()) + " " + string(ctx.Request.URI().Path())

		// 如果当前端点在白名单中，使用原始context继续处理
		if whitelistMap[currentEndpoint] {
			ctx.Next(c)
			return
		}

		// 对于非白名单端点，创建一个新的context，不受客户端断开连接影响
		newCtx := context.Background()

		// 从原始context复制一些必要的值到新context
		// 注意：由于context的设计，我们无法直接复制所有值
		// 在实际应用中，可能需要根据需求复制特定的键值

		// 使用新的context继续处理请求
		ctx.Next(newCtx)
	}
}
