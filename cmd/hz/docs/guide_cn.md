# hz

English | [中文](guide_cn.md)

**简介**

hz 是 Hertz 框架提供的一个用于生成代码的命令行工具。目前，hz 可以基于 thrift 和 protobuf 的 IDL 生成 Hertz 项目的脚手架

## 目录

- [基本使用](#basic-usage)
    - [创建一个初始项目](#create-project)
    - [基于 thrift IDL 创建项目](#basic-thrift)
    - [基于 protobuf IDL 创建项目](#basic-protobuf)
    - [更新一个已有的项目](#update-project)
    - [使用 hz client 生成客户端代码](#client)
      - [简介](#client-intro)
      - [简易示例](#client-example)
- [hz 默认生成的代码结构](#default-struct)
- [自定义模板](#custom-template)
- [hz 命令参数梳理](#hz-args)
    - [new](#arg-new)
    - [update](#arg-update)
    - [client](#arg-client)

- [中间件](#middleware)

## <div id="basic-usage">基本使用</div>

### <div id="create-project">创建一个初始项目<h3/>

1. 在 GOPATH 下执行时, go mod 名字默认为当前路径相对GOPATH的路径, 也可自己指定

```shell
hz new
```

2. 非GOPATH 下执行，需要指定 go mod 名. 以下命令中, `hertz/demo` 为 module 名

```shell
hz new -mod hertz/demo
```

### <div id="basic-thrift">基于 thrift IDL 创建项目</div>

1. 在当前目录下创建 thrift IDL 文件

```thrift
// idl/hello.thrift
namespace go hello.example

struct HelloReq {
    1: string Name (api.query="name"); // 添加 api 注解为方便进行参数绑定
}

struct HelloResp {
    1: string RespBody;
}


service HelloService {
    HelloResp HelloMethod(1: HelloReq request) (api.get="/hello");
}
```

2. 创建新项目

在 GOPATH 下执行时, 无需 -mod 选项.

```shell
hz new -idl idl/hello.thrift

## 整理 & 拉取依赖
go mod tidy
```

### <div id="basic-protobuf">基于 protobuf IDL 创建项目<div/>

在当前目录下创建 protobuf idl 文件

注: 为在 protobuf 中支持 api 注解，请在使用了注解的 proto 文件中, import 下面的文件

暂定以下文件为 idl/api.proto

```protobuf
syntax = "proto2";

package api;

import "google/protobuf/descriptor.proto";

option go_package = "/api";

extend google.protobuf.FieldOptions {
  optional string raw_body = 50101;
  optional string query = 50102;
  optional string header = 50103;
  optional string cookie = 50104;
  optional string body = 50105;
  optional string path = 50106;
  optional string vd = 50107;
  optional string form = 50108;
  optional string go_tag = 51001;
  optional string js_conv = 50109;
}

extend google.protobuf.MethodOptions {
  optional string get = 50201;
  optional string post = 50202;
  optional string put = 50203;
  optional string delete = 50204;
  optional string patch = 50205;
  optional string options = 50206;
  optional string head = 50207;
  optional string any = 50208;
  optional string gen_path = 50301;
  optional string api_version = 50302;
  optional string tag = 50303;
  optional string name = 50304;
  optional string api_level = 50305;
  optional string serializer = 50306;
  optional string param = 50307;
  optional string baseurl = 50308;
}

extend google.protobuf.EnumValueOptions {
  optional int32 http_code = 50401;
}
```

主 IDL 定义:

```protobuf
// idl/hello/hello.proto
syntax = "proto3";

package hello;

option go_package = "hertz/hello";

import "api.proto";

message HelloReq {
  string Name = 1[(api.query) = "name"];
}

message HelloResp {
  string RespBody = 1;
}

service HelloService {
  rpc Method1(HelloReq) returns(HelloResp) {
    option (api.get) = "/hello";
  }
}
```

2. 创建新项目

此为在 GOPATH 下执行, 如果主 IDL 的依赖和主 IDL 不在同一路径下，需要加入 -I 选项，其含义为IDL搜索路径，等同于 protoc 的 -I
命令

```shell
hz new -I idl -idl idl/hello/hello.proto

# 整理 & 拉取依赖
go mod tidy
```

### <div id="update-project">更新一个已有的项目</div>

以上面的<a href="#basic-thrift">示例<a/>为基础, 当 IDL 出现更新时, 可以执行以下命令来更新代码

protobuf:

```shell
hz update -I idl -idl idl/hello.proto 
```

thrift:

```shell
hz update -idl idl/hello.thrift
```

执行后便会发现代码结构进行了相应的更新

这里展示了基本的更新操作, 详细操作可见:

- <a href="https://www.cloudwego.io/zh/docs/hertz/tutorials/toolkit/usage/usage-protobuf/#update-%E6%9B%B4%E6%96%B0%E4%B8%80%E4%B8%AA%E5%B7%B2%E6%9C%89%E7%9A%84%E9%A1%B9%E7%9B%AE">
  protobuf 更新一个已有的项目</a>
- <a href="https://www.cloudwego.io/zh/docs/hertz/tutorials/toolkit/usage/usage-thrift/#update-%E6%9B%B4%E6%96%B0%E4%B8%80%E4%B8%AA%E5%B7%B2%E6%9C%89%E7%9A%84%E9%A1%B9%E7%9B%AE">
  thrift 更新一个已有的项目</a>

### <div id="client">使用 hz client 生成客户端代码</div>

#### <div id="client-intro">介绍</div>

hz 生成的代码结构都类似，下面以"基于 thrift IDL 创建项目"小节生成的代码结构为例，说明 hz 生成的代码的含义

基于 IDL 生成类似 RPC 形式的 http 请求一键调用，屏蔽掉创建和初始化 hertz client 的繁琐操作，并且实现和 hz 生成的 server
代码直接互通。

生成代码示例可以参考 [code](https://github.com/cloudwego/hertz-examples/tree/main/hz/hz_client)

#### <div id="client-example">简易示例</div>

>IDL 的定义和语义与目前的定义完全相同，所以基本不用修改原先的 IDL 即可生成 client 代码
>但是为针对 client 的场景，增加了两种注解
>api.file_name： 指定文件
>api.base_domain： 指定默认访问的请求 domain

```thrift
namespace go toutiao.middleware.hertz_client

struct FormReq {
    1: string FormValue (api.form="form1"); // form 注解用来声明 form 参数("multipart/form-data")
    2: string FileValue (api.file_name="file1"); // file_name 用来声明要上传文件的 key，其实际的值为文件名
}

struct QueryReq {
    1: string QueryValue (api.query="query1"); // query 注解用来声明请求的 query 参数
}

struct PathReq {
    1: string PathValue (api.path="path1"); // path 注解用来声明url中的路由参数
}

struct BodyReq {
    1: string BodyValue (api.body="body"); // body 注解不管是否声明都将整个结构体以 json 的形式设置到 body
    2: string QueryValue (api.query="query2");
}

struct Resp {
    1: string Resp;
}

service HelloService {
    // api.post 用来声明请求的路由
    Resp FormMethod(1: FormReq request) (api.post="/form", api.handler_path="post");
    Resp QueryMethod(1: QueryReq request) (api.get="/query", api.handler_path="get");
    Resp PathMethod(1: PathReq request) (api.post="/path:path1", api.handler_path="post");
    Resp BodyMethod(1: BodyReq request) (api.post="/body", api.handler_path="post");
}(
    // api.base_domain 用来指定默认的 client 请求的 domain
    api.base_domain="http://127.0.0.1:8888";
)
```

##### 生成 client 代码

```shell
hz client --mod=a/b/c --idl=../idl/psm.thrift --model_dir=model --client_dir=hertz_client -t=template=slim
```

高级设置

请求级别的配置
> 以 thrift IDL 生成的代码为例

```go
func main() {
	generatedClient, err := hello_service.NewHelloServiceClient(
		"http://toutiao.hertz.testa", // 指定 psm 作为域名 
		)
	// 在发起调用的时候可指定请求级别的配置
    resp, rawResp, err := generatedClient.QueryMethod(
        context.Background(),
        QueryReq,
        config.WithSD(true), // 指定请求级别的设置，用来开启服务发现
        config.WithReadTimeout(), // 指定请求读超时
        )
    if err != nil {
       fmt.Println(err)
       return
    }
}
```

设置 client 中间件
> 以 thrift IDL 生成的代码为例

```go
func main() {
	generatedClient, err := hello_service.NewHelloServiceClient(
		"http://toutiao.hertz.testa", // 指定 psm 作为域名 
		hello_service.WithHertzClientMiddleware(), // 指定 client 的中间件 
	)
}
```

设置全局 header
>以 thrift IDL 生成的代码为例

有一些通用的 header 可能每次请求都需要携带，或者是一些不能定义到 IDL 中的 header，这时我们就可以通过 "WithHeader" 注入这些 header，使得每次发送请求都会携带这些header。
```go
func main() {
	generatedClient, err := hello_service.NewHelloServiceClient(
		"http://toutiao.hertz.testa", // 指定 psm 作为域名 
		hello_service.WithHeader(), // 指定每次发送请求都需要携带的header 
	)
}
```

### 配置 TLS
> 以 thrift IDL 生成的代码为例

Hertz client 的 TLS 走的是标准网络库，因此在使用生成的一键调用时需要配置为了标准网络库
```go
func main() {
	generatedClient, err := hello_service.NewHelloServiceClient("https://www.example.com"), 
	hello_service.WithHertzClientOption(
		client.WithDialer(standard.NewDialer()), // 使用标准库 
		client.WithTLSConfig(clientCfg), // TLS 配置 
		), 
	)
}
```

## <div id="default-struct">hz 默认生成的代码结构</div>



```
.
├── biz                                // business 层，存放业务逻辑相关流程
│   ├── handler                        // 存放 handler 文件
│   │   ├── hello                      // hello/example 对应 thrift idl 中定义的 namespace；而对于 protobuf idl，则是对应 go_package 的最后一级
│   │   │   └── example
│   │   │       ├── hello_service.go   // handler 文件，用户在该文件里实现 IDL service 定义的方法，update 时会查找 当前文件已有的 handler 在尾部追加新的 handler
│   │   │       └── new_service.go     // 同上，idl 中定义的每一个 service 对应一个文件
│   │   └── ping.go                    // 默认携带的 ping handler，用于生成代码快速调试，无其他特殊含义
│   ├── model                          // IDL 内容相关的生成代码
│   │   └── hello                      // hello/example 对应 thrift idl 中定义的 namespace；而对于 protobuf idl，则是对应 go_package
│   │     └── example
│   │         └── hello.go             // thriftgo 的产物，包含 hello.thrift 定义的内容的 go 代码，update 时会重新生成
│   └── router                         // idl 中定义的路由相关生成代码
│       ├── hello                      // hello/example 对应 thrift idl 中定义的namespace；而对于 protobuf idl，则是对应 go_package 的最后一级
│       │   └── example
│       │       ├── hello.go           // hz 为 hello.thrift 中定义的路由生成的路由注册代码；每次 update 相关 idl 会重新生成该文件
│       │       └── middleware.go      // 默认中间件函数，hz 为每一个生成的路由组都默认加了一个中间件；update 时会查找当前文件已有的 middleware 在尾部追加新的 middleware
│       └── register.go                // 调用注册每一个 idl 文件中的路由定义；当有新的 idl 加入，在更新的时候会自动插入其路由注册的调用；勿动
├── go.mod                             // go.mod 文件，如不在命令行指定，则默认使用相对于GOPATH的相对路径作为 module 名
├── idl                                // 用户定义的idl，位置可任意
│   └── hello.thrift
├── main.go                            // 程序入口
├── router.go                          // 用户自定义除 idl 外的路由方法
└── router_gen.go                      // hz 生成的路由注册代码，用于调用用户自定义的路由以及 hz 生成的路由
```

## <div id="custom-template">自定义模板</div>

## <div id="hz-args">命令参数梳理</div>

### <div id="args-new">new</div>

| 命令参数              | 命令参数描述                                                                                                                 |
|-------------------|------------------------------------------------------------------------------------------------------------------------|
| client_dir        | 指定 client 侧代码的生成路径，如果不指定则不生成；当前为每个 service 生成一个全局的 client，后续会提供更丰富的 client 代码能力                                        |
| customize_layout  | 自定义项目 layout 模板，具体详见：自定义模板使用                                                                                           |
| customize_package | 自定义项目 package 相关模板，主要可针对 handler 模板进行定制化，具体详见：自定义模板使用                                                                  |
| exclude_file      | 不需要更新的文件(相对项目路径，支持多个)                                                                                                  |
| handler_dir       | 指定 handler 的生成路径，默认为 “biz/handler”                                                                                     |
| idl               | idl 文件路径(.thrift 或者.proto)                                                                                             |
| json_enumstr      | 当 idl 为 thrift 时，json enums 使用 string 代替 num(透传给 thriftgo 的选项)                                                         |
| model_dir         | 指定 model 的生成路径，默认为"biz/model"                                                                                          |
| module/mod        | 指定 go mod 的名字，非 GOPATH 下必须指定，GOPATH 下默认以相对于 GOPATH 的路径作为名字                                                             |
| no_recurse        | 只生成主 idl 的 model 代码                                                                                                    |
| option_package/P  | 指定包的路径，({include_path}={import_path})                                                                                  |
| out_dir           | 指定项目生成路径                                                                                                               |
| proto_path/I      | 当 idl 为 protobuf 时，指定 idl 的搜索路径，同 protoc 的 -I 指令                                                                       |
| protoc/p          | 透传给 protoc 的选项({flag}={value})                                                                                         |
| service           | 服务名，为之后做服务发现等功能预留                                                                                                      |
| snake_tag         | tag 使用 snake_case 风格命名(仅对 form、query、json 生效)                                                                          |
| thriftgo/t        | 透传给 thriftgo 的选项({flag}={value})                                                                                       |
| unset_omitempty   | 当 idl 为 protobuf 时，生成 model field，去掉 omitempty tag；当 idl 为 thrift 时，是否添加 omitempty 根据 field 是 “optional"还是"required"决定 |

### <div id="args-updae">update</div>

| 命令参数              | 命令参数描述                                                                                                                 |
|-------------------|------------------------------------------------------------------------------------------------------------------------|
| client_dir        | 指定 client 侧代码的生成路径，如果不指定则不生成；当前为每个 service 生成一个全局的 client，后续会提供更丰富的 client 代码能力                                        |
| customize_package | 自定义项目 package 相关模板，主要可针对 handler 模板进行定制化，具体详见：自定义模板使用                                                                  |
| exclude_file      | 不需要更新的文件(相对项目路径，支持多个)                                                                                                  |
| handler_dir       | 指定 handler 的生成路径，默认为 “biz/handler”                                                                                     |
| idl               | idl 文件路径(.thrift 或者.proto)                                                                                             |
| json_enumstr      | 当 idl 为 thrift 时，json enums 使用 string 代替 num(透传给 thriftgo 的选项)                                                         |
| model_dir         | 指定 model 的生成路径，默认为"biz/model"                                                                                          |
| module/mod        | 指定 go mod 的名字，非 GOPATH 下必须指定，GOPATH 下默认以相对于 GOPATH 的路径作为名字                                                             |
| no_recurse        | 只生成主 idl 的 model 代码                                                                                                    |
| option_package/P  | 指定包的路径，({include_path}={import_path})                                                                                  |
| out_dir           | 指定项目生成路径                                                                                                               |
| proto_path/I      | 当 idl 为 protobuf 时，指定 idl 的搜索路径，同 protoc 的 -I 指令                                                                       |
| protoc/p          | 透传给 protoc 的选项({flag}={value})                                                                                         |
| service           | 服务名，为之后做服务发现等功能预留                                                                                                      |
| snake_tag         | tag 使用 snake_case 风格命名(仅对 form、query、json 生效)                                                                          |
| thriftgo/t        | 透传给 thriftgo 的选项({flag}={value})                                                                                       |
| unset_omitempty   | 当 idl 为 protobuf 时，生成 model field，去掉 omitempty tag；当 idl 为 thrift 时，是否添加 omitempty 根据 field 是 “optional"还是"required"决定 |

### <div id="args-client">client</div>

| 命令参数        | 命令参数说明                                                              |
|-------------|---------------------------------------------------------------------|
| idl         | 指定 idl 路径                                                           |
| module      | 指定项目的 go module，如果不指定则默认为相对于 “go path” 的路径                          |
| model_dir   | 指定项目生成的 model 路径，默认为 “biz/model”                                    |
| client_dir  | 指定生成 client 桩代码的路径，默认为 “biz/model/{Namespace}”                      |
| base_domain | 指定要访问的 domain，可以是 域名、 IP:PORT、service name(配合服务发现)，也可以在 IDL 中通过注解声明 |

## <div id="middleware">中间件</div>

| 扩展                                                                                                 | 描述                                                                                     |
|----------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------|
| [Autotls](https://github.com/hertz-contrib/autotls)                                                | 使 Hertz 支持 Let's Encrypt。                                                              |
| [Http2](https://github.com/hertz-contrib/http2)                                                    | 为 Hertz 提供 HTTP2 支持。                                                                   |
| [Websocket](https://github.com/hertz-contrib/websocket)                                            | 启用 Hertz 支持 Websocket 协议。                                                              |
| [Limiter](https://github.com/hertz-contrib/limiter)                                                | 基于 bbr 算法提供当前限制器。                                                                      |
| [Monitor-prometheus](https://github.com/hertz-contrib/monitor-prometheus)                          | 基于 Prometheus 提供服务监控。                                                                  |
| [Obs-opentelemetry](https://github.com/hertz-contrib/obs-opentelemetry)                            | Hertz 的 Opentelemetry 扩展，支持指标、日志、跟踪，并且可以直接使用。                                          |
| [Opensergo](https://github.com/hertz-contrib/opensergo)                                            | Opensergo 扩展。                                                                          |
| [Pprof](https://github.com/hertz-contrib/pprof)                                                    | 用于与 Pprof 集成的 Hertz 扩展。                                                                |
| [Registry](https://github.com/hertz-contrib/registry)                                              | 提供服务注册和发现功能。目前支持的服务发现扩展有：nacos、consul、etcd、eureka、polaris、servicecomb、zookeeper、redis。 |
| [Sentry](https://github.com/hertz-contrib/hertzsentry)                                             | Sentry 扩展提供了一些统一的接口，帮助用户进行实时错误监控。                                                      |
| [Tracer](https://github.com/hertz-contrib/tracer)                                                  | 基于 Opentracing 的链路追踪。                                                                  |
| [Basicauth](https://github.com/cloudwego/hertz/tree/develop/pkg/app/middlewares/server/basic_auth) | Basicauth 中间件可以提供 HTTP 基本认证。                                                           |
| [Jwt](https://github.com/hertz-contrib/jwt)                                                        | Jwt 扩展。                                                                                |
| [Keyauth](https://github.com/hertz-contrib/keyauth)                                                | 提供基于令牌的身份验证。                                                                           |
| [Requestid](https://github.com/hertz-contrib/requestid)                                            | 在响应中添加请求 ID。                                                                           |
| [Sessions](https://github.com/hertz-contrib/sessions)                                              | 带有多状态存储支持的会话中间件。                                                                       |
| [Cors](https://github.com/hertz-contrib/cors)                                                      | 提供跨域资源共享支持。                                                                            |
| [Csrf](https://github.com/hertz-contrib/csrf)                                                      | Csrf 中间件用于防止跨站点请求伪造攻击。                                                                 |
| [Secure](https://github.com/hertz-contrib/secure)                                                  | 具有多个配置项的安全中间件。                                                                         |
| [Gzip](https://github.com/hertz-contrib/gzip)                                                      | 一个带有多个选项的 Gzip 扩展。                                                                     |
| [I18n](https://github.com/hertz-contrib/i18n)                                                      | 帮助将 Hertz 程序翻译成多种编程语言。                                                                 |
| [Lark](https://github.com/hertz-contrib/lark-hertz)                                                | 使用 Hertz 处理 Lark/飞书卡片消息和事件回调。                                                          |
| [Loadbalance](https://github.com/hertz-contrib/loadbalance)                                        | 为 Hertz 提供负载均衡算法。                                                                      |
| [Logger](https://github.com/hertz-contrib/logger)                                                  | Hertz 的日志记录器扩展，提供对 zap、logrus、zerologs 日志框架的支持。                                        |
| [Recovery](https://github.com/cloudwego/hertz/tree/develop/pkg/app/middlewares/server/recovery)    | Hertz 的恢复中间件。                                                                          |
| [Reverseproxy](https://github.com/hertz-contrib/reverseproxy)                                      | 实现反向代理。                                                                                |
| [Swagger](https://github.com/hertz-contrib/swagger)                                                | 使用 Swagger 2.0 自动生成 RESTful API 文档。                                                    |
