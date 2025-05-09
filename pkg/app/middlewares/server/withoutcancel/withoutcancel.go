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

// Package withoutcancel 提供一个中间件，使特定的路由即使在客户端断开连接的情况下也能继续执行
package withoutcancel

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

// WithoutCancelCtx 为特定路由创建一个不受客户端连接中断影响的上下文
// 即使客户端断开连接，处理程序仍将继续执行直到完成
func WithoutCancelCtx() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		// 创建一个新的context，该context不会被客户端断开连接而取消
		newCtx := context.Background()

		// 从原始context复制值到新context
		// 注意：这里不会复制原始context的Done channel和取消函数
		copyValues(c, newCtx)

		// 使用新的context继续处理请求
		ctx.Next(newCtx)
	}
}

// copyValues 将src context中的值复制到dst context
// 注意：由于context.Context中的值是不可变的，我们无法直接复制
// 这个函数主要用于示例，实际应用中可能需要更复杂的逻辑来处理特定值
func copyValues(src, dst context.Context) {
	// 在实际应用中，你可能需要根据具体需求复制特定的值
	// 例如，如果你知道有特定的键需要复制：
	// if val := src.Value(someKey); val != nil {
	//     dst = context.WithValue(dst, someKey, val)
	// }
}

// New 返回WithoutCancelCtx中间件
func New() app.HandlerFunc {
	return WithoutCancelCtx()
}
