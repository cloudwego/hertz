# Hertz

[English](README.md) | 中文

[![Release](https://img.shields.io/github/v/release/cloudwego/hertz)](https://github.com/cloudwego/hertz/releases)
[![WebSite](https://img.shields.io/website?up_message=cloudwego&url=https%3A%2F%2Fwww.cloudwego.io%2F)](https://www.cloudwego.io/)
[![License](https://img.shields.io/github/license/cloudwego/hertz)](https://github.com/cloudwego/hertz/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudwego/hertz)](https://goreportcard.com/report/github.com/cloudwego/hertz)
[![OpenIssue](https://img.shields.io/github/issues/cloudwego/hertz)](https://github.com/cloudwego/hertz/issues)
[![ClosedIssue](https://img.shields.io/github/issues-closed/cloudwego/hertz)](https://github.com/cloudwego/hertz/issues?q=is%3Aissue+is%3Aclosed)
![Stars](https://img.shields.io/github/stars/cloudwego/hertz)
![Forks](https://img.shields.io/github/forks/cloudwego/hertz)

Hertz[həːts] 是一个 Golang 微服务 HTTP 框架，在设计之初参考了其他开源框架 [fasthttp](https://github.com/valyala/fasthttp)、[gin](https://github.com/gin-gonic/gin)、[echo](https://github.com/labstack/echo) 的优势，并结合字节跳动内部的需求，使其具有高易用性、高性能、高扩展性等特点，目前在字节跳动内部已广泛使用。如今越来越多的微服务选择使用 Golang，如果对微服务性能有要求，又希望框架能够充分满足内部的可定制化需求，Hertz 会是一个不错的选择。
## 框架特点
- 高易用性

  在开发过程中，快速写出来正确的代码往往是更重要的。因此，在 Hertz 在迭代过程中，积极听取用户意见，持续打磨框架，希望为用户提供一个更好的使用体验，帮助用户更快的写出正确的代码。
- 高性能

  Hertz 默认使用自研的高性能网络库 Netpoll，在一些特殊场景相较于 go net，Hertz 在 QPS、时延上均具有一定优势。关于性能数据，可参考下图 Echo 数据。
  ![Performance](images/performance.png)
  关于详细的性能数据，可参考 [hertz-benchmark](https://github.com/cloudwego/hertz-benchmark)。
- 高扩展性

  Hertz 采用了分层设计，提供了较多的接口以及默认的扩展实现，用户也可以自行扩展。同时得益于框架的分层设计，框架的扩展性也会大很多。目前仅将稳定的能力开源给社区，更多的规划参考 [RoadMap](ROADMAP.md)。
- 多协议支持

  Hertz 框架原生提供 HTTP1.1、ALPN 协议支持。除此之外，由于分层设计，Hertz 甚至支持自定义构建协议解析逻辑，以满足协议层扩展的任意需求。
- 网络层切换能力

  Hertz 实现了 Netpoll 和 Golang 原生网络库 间按需切换能力，用户可以针对不同的场景选择合适的网络库，同时也支持以插件的方式为 Hertz 扩展网络库实现。
## 详细文档
### [快速开始](https://www.cloudwego.io/zh/docs/hertz/getting-started/)
### Example
  Hertz-Examples 仓库提供了开箱即用的代码，[详见](https://www.cloudwego.io/zh/docs/hertz/tutorials/example/)。
### 用户指南
### 基本特性
  包含通用中间件的介绍和使用，上下文选择，数据绑定，数据渲染，直连访问，日志，错误处理，[详见文档](https://www.cloudwego.io/zh/docs/hertz/tutorials/basic-feature/)
### 治理特性
  包含 trace monitor，[详见文档](https://www.cloudwego.io/zh/docs/hertz/tutorials/service-governance/)
### 框架扩展
  包含网络库扩展，[详见文档](https://www.cloudwego.io/zh/docs/hertz/tutorials/framework-exten/)
### 参考
  apidoc、框架可配置项一览，[详见文档](https://www.cloudwego.io/zh/docs/hertz/reference/)
### FAQ
  常见问题排查，[详见文档](https://www.cloudwego.io/zh/docs/hertz/faq/)
## 框架性能
  性能测试只能提供相对参考，工业场景下，有诸多因素可以影响实际的性能表现
  我们提供了 hertz-benchmark 项目用来长期追踪和比较 Hertz 与其他框架在不同情况下的性能数据以供参考
## 相关项目
- [Netpoll](https://github.com/cloudwego/netpoll): 自研高性能网络库，Hertz 默认集成
- [Hertz-Contrib](https://github.com/hertz-contrib): Hertz 扩展仓库，提供中间件、tracer 等能力
- [Example](https://github.com/cloudwego/hertz-examples): Hertz 使用例子
## 相关拓展

| 拓展                                                         | 描述                                                         |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [websocket](https://github.com/hertz-contrib/websocket)      | 使 Hertz 支持 WebSocket 协议。                               |
| [pprof](https://github.com/hertz-contrib/pprof)              | 通过其 HTTP 服务器运行时分析数据并以 pprof 可视化工具所期望的格式提供服务。 |
| [sessions](https://github.com/hertz-contrib/sessions)        | 具有多后端支持的 session 中间件。                            |
| [obs-opentelemetry](https://github.com/hertz-contrib/obs-opentelemetry) | 集成了默认 opentelemetry 程序，达到开箱即用并支持设置环境变量。 |
| [registry](https://github.com/hertz-contrib/registry)        | 到现在为止，支持的服务发现拓展有 nacos， consul， etcd， eureka， polaris， servicecomb， zookeeper。 |
| [keyauth](https://github.com/hertz-contrib/keyauth)          | 提供基于密钥的身份验证。                                     |
| [secure](https://github.com/hertz-contrib/secure)            | 具有多配置项的 secure 中间件。                               |
| [sentry](https://github.com/hertz-contrib/hertzsentry)       | sentry 拓展提供了一些统一的接口来帮助用户获取哨兵中心以及报告错误信息。 |
| [requestid](https://github.com/hertz-contrib/requestid)      | 使用 X-Request-ID 标头向响应添加标识符，如果在请求标头中发送 X-Request-ID 值，则将其传递回调用方。 |
| [limiter](https://github.com/hertz-contrib/limiter)          | 使用 bbr 算法，通过读取/proc/stat收集CPU负载，判断CPU负载触发限流条件。 |
| [jwt](https://github.com/hertz-contrib/jwt)                  | JWT 返回一个 JSON Web Token(JWT) 身份验证中间件。            |
| [autotls](https://github.com/hertz-contrib/autotls)          | 为 Hertz 支持 Let's Encrypt 。                               |
| [monitor-prometheus](https://github.com/hertz-contrib/monitor-prometheus) | Prometheus 中存储的数据为时间序列，是由 metric 的名字和一系列的标签（键值对）唯一标识的，不同的标签则代表不同的时间序列。 |
| [i18n](https://github.com/hertz-contrib/i18n)                | 可帮助将 Hertz 程序翻译成多种语言。                          |
| [reverseproxy](https://github.com/hertz-contrib/reverseproxy) | 实现反向代理，目前 netpoll 不支持 tls ，需要使用 net（标准库）。 |
| [opensergo](https://github.com/hertz-contrib/opensergo)      | 提供 opensergo 的哨兵适配器。                                |
| [gzip](https://github.com/hertz-contrib/gzip)                | 含多个可选项的 gzip 拓展。                                   |
| [cors](https://github.com/hertz-contrib/cors)                | 提供跨域资源共享支持。                                       |
| [swagger](https://github.com/hertz-contrib/swagger)          | 使用 Swagger 2.0 自动生成 RESTful API 文档。                 |
| [tracer](https://github.com/hertz-contrib/tracer)            | 能够快速在 Hertz 中启用链路追踪功能。                        |

## 相关文章
- [字节跳动在 Go 网络库上的实践](https://www.cloudwego.io/blog/2021/10/09/bytedance-practices-on-go-network-library/)
## 贡献代码
  [Contributing](https://github.com/cloudwego/hertz/blob/main/CONTRIBUTING.md)
## RoadMap
  [Hertz RoadMap](ROADMAP.md)
## 开源许可

Hertz 基于[Apache License 2.0](https://github.com/cloudwego/hertz/blob/main/LICENSE) 许可证，其依赖的三方组件的开源许可见 [Licenses](https://github.com/cloudwego/hertz/blob/main/licenses)。

## 联系我们
- Email: conduct@cloudwego.io
- 如何成为 member: [COMMUNITY MEMBERSHIP](https://github.com/cloudwego/community/blob/main/COMMUNITY_MEMBERSHIP.md)
- Issues: [Issues](https://github.com/cloudwego/hertz/issues)
- 飞书用户群（[注册飞书](https://www.larksuite.com/zh_cn/download)进群）

  ![LarkGroup](images/lark_group_cn.png)
- 微信: CloudWeGo community

  ![WechatGroup](images/wechat_group_cn.png)
## Landscapes

<p align="center">
<img src="https://landscape.cncf.io/images/left-logo.svg" width="150"/>&nbsp;&nbsp;<img src="https://landscape.cncf.io/images/right-logo.svg" width="200"/>
<br/><br/>
CloudWeGo 丰富了 <a href="https://landscape.cncf.io/">CNCF 云原生生态</a>。
</p>