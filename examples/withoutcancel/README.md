# 客户端断开连接检测控制示例

这个示例展示了如何在Hertz框架中实现对客户端断开连接检测的细粒度控制。

## 背景

在大多数HTTP服务场景中，我们不需要关注客户端是否已经断开连接，服务器会继续执行请求处理直到完成。但在某些特定场景（如SSE、WebSocket连接）中，我们需要能够及时检测到客户端是否已经关闭连接，以便释放资源。

Hertz框架通过`WithSenseClientDisconnection(true)`选项提供了检测客户端断开连接的能力，但这是全局性的设置。本示例提供了两种中间件，允许对此功能进行更细粒度的控制：

1. `withoutcancel`: 对特定路由禁用客户端断开连接检测
2. `withoutcancelendpoints`: 基于白名单机制，仅对特定端点启用客户端断开连接检测

## 工作原理

### WithSenseClientDisconnection

Hertz框架的`WithSenseClientDisconnection`选项允许检测客户端断开连接。当启用此选项时：

1. 每个请求上下文(context.Context)都会附加一个取消函数
2. 当客户端断开连接时，这个取消函数会被调用，导致上下文被取消
3. 处理程序可以通过检查`<-ctx.Done()`来感知客户端是否已断开连接

### WithoutCancel中间件

`withoutcancel`中间件通过以下方式禁用客户端断开连接检测：

1. 创建一个新的`context.Background()`上下文，该上下文没有取消机制
2. 使用这个新上下文继续处理请求

这样，即使客户端断开连接，处理程序也会继续执行直到完成。

### WithoutCancelEndpoints中间件

`withoutcancelendpoints`中间件提供了基于白名单的控制机制：

1. 配置中定义了一个白名单端点列表
2. 对于白名单中的端点，保留原始上下文，允许感知客户端断开连接
3. 对于不在白名单中的端点，使用新的`context.Background()`上下文，禁用客户端断开连接检测

## 使用示例

### 1. 启用全局客户端断开连接检测

```go
h := server.Default(
    server.WithSenseClientDisconnection(true),
)
```

### 2. 对特定路由禁用客户端断开连接检测

```go
// 导入中间件
import "github.com/cloudwego/hertz/pkg/app/middlewares/server/withoutcancel"

// 在路由定义中使用
h.GET("/api/longprocess", withoutcancel.New(), func(c context.Context, ctx *app.RequestContext) {
    // 即使客户端断开连接，这个处理程序也会继续执行直到完成
})
```

### 3. 使用白名单机制

```go
// 导入中间件
import "github.com/cloudwego/hertz/pkg/app/middlewares/server/withoutcancelendpoints"

// 定义白名单配置
whitelistConfig := withoutcancelendpoints.Config{
    WhitelistEndpoints: []string{
        "GET /api/stream",
        "GET /api/websocket",
    },
}

// 应用到路由组
apiGroup := h.Group("/api", withoutcancelendpoints.New(whitelistConfig))

// 白名单中的端点会感知客户端断开连接
apiGroup.GET("/stream", func(c context.Context, ctx *app.RequestContext) {
    select {
    case <-time.After(10 * time.Second):
        // 正常完成
    case <-c.Done():
        // 客户端断开连接
        return
    }
})

// 非白名单端点不会感知客户端断开连接
apiGroup.GET("/longprocess", func(c context.Context, ctx *app.RequestContext) {
    // 即使客户端断开连接，这个处理程序也会继续执行直到完成
})
```

## 运行示例

1. 启动服务器:

```bash
go run main.go
```

2. 在浏览器中访问以下URL:

- http://127.0.0.1:8888/api/normal (会感知客户端断开连接)
- http://127.0.0.1:8888/api/withoutcancel (不会感知客户端断开连接)
- http://127.0.0.1:8888/api/stream (白名单端点，会感知客户端断开连接)
- http://127.0.0.1:8888/api/longprocess (非白名单端点，不会感知客户端断开连接)

3. 打开URL后，在10秒内关闭浏览器或取消请求，然后观察服务器日志输出 