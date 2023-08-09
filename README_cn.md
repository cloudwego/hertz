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
  
  四个框架的对比:
  ![Performance](images/performance-4.png)
  三个框架的对比:
  ![Performance](images/performance-3.png)
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

| 拓展                                                                                                 | 描述                                                                                                |
|----------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------|
| [Autotls](https://github.com/hertz-contrib/autotls)                                                | 为 Hertz 支持 Let's Encrypt 。                                                                        |
| [Http2](https://github.com/hertz-contrib/http2)                                                    | 提供对 HTTP2 的支持。                                                                                    |
| [Websocket](https://github.com/hertz-contrib/websocket)                                            | 使 Hertz 支持 Websocket 协议。                                                                          |
| [Etag](https://github.com/hertz-contrib/etag)                                                      | 提供 ETag HTTP 响应标头。                                                                                |
| [Limiter](https://github.com/hertz-contrib/limiter)                                                | 提供了基于 bbr 算法的限流器。                                                                                 |
| [Monitor-prometheus](https://github.com/hertz-contrib/monitor-prometheus)                          | 提供基于 Prometheus 服务监控功能。                                                                           |
| [Obs-opentelemetry](https://github.com/hertz-contrib/obs-opentelemetry)                            | Hertz 的 Opentelemetry 扩展，支持 Metric、Logger、Tracing 并且达到开箱即用。                                       |
| [Opensergo](https://github.com/hertz-contrib/opensergo)                                            | Opensergo 扩展。                                                                                     |
| [Pprof](https://github.com/hertz-contrib/pprof)                                                    | Hertz 集成 Pprof 的扩展。                                                                               |
| [Registry](https://github.com/hertz-contrib/registry)                                              | 提供服务注册与发现功能。到现在为止，支持的服务发现拓展有 nacos， consul， etcd， eureka， polaris， servicecomb， zookeeper， redis。 |
| [Sentry](https://github.com/hertz-contrib/hertzsentry)                                             | Sentry 拓展提供了一些统一的接口来帮助用户进行实时的错误监控。                                                                |
| [Tracer](https://github.com/hertz-contrib/tracer)                                                  | 基于 Opentracing 的链路追踪。                                                                             |
| [Basicauth](https://github.com/cloudwego/hertz/tree/develop/pkg/app/middlewares/server/basic_auth) | Basicauth 中间件能够提供 HTTP 基本身份验证。                                                                    |
| [Jwt](https://github.com/hertz-contrib/jwt)                                                        | Jwt 拓展。                                                                                           |
| [Keyauth](https://github.com/hertz-contrib/keyauth)                                                | 提供基于 token 的身份验证。                                                                                 |
| [Requestid](https://github.com/hertz-contrib/requestid)                                            | 在 response 中添加 request id。                                                                        |
| [Sessions](https://github.com/hertz-contrib/sessions)                                              | 具有多状态存储支持的 Session 中间件。                                                                           |
| [Casbin](https://github.com/hertz-contrib/casbin)                                                  | 通过 Casbin 支持各种访问控制模型。                                                                             |
| [Cors](https://github.com/hertz-contrib/cors)                                                      | 提供跨域资源共享支持。                                                                                       |
| [Csrf](https://github.com/hertz-contrib/csrf)                                                      | Csrf 中间件用于防止跨站点请求伪造攻击。                                                                            |
| [Secure](https://github.com/hertz-contrib/secure)                                                  | 具有多配置项的 Secure 中间件。                                                                               |
| [Gzip](https://github.com/hertz-contrib/gzip)                                                      | 含多个可选项的 Gzip 拓展。                                                                                  |
| [I18n](https://github.com/hertz-contrib/i18n)                                                      | 可帮助将 Hertz 程序翻译成多种语言。                                                                             |
| [Lark](https://github.com/hertz-contrib/lark-hertz)                                                | 在 Hertz 中处理 Lark/飞书的卡片消息和事件的回调。                                                                   |
| [Loadbalance](https://github.com/hertz-contrib/loadbalance)                                        | 提供适用于 Hertz 的负载均衡算法。                                                                              |
| [Logger](https://github.com/hertz-contrib/logger)                                                  | Hertz 的日志拓展，提供了对 zap、logrus、zerologs 日志框架的支持。                                                     |
| [Recovery](https://github.com/cloudwego/hertz/tree/develop/pkg/app/middlewares/server/recovery)    | Hertz 的异常恢复中间件。                                                                                   |
| [Reverseproxy](https://github.com/hertz-contrib/reverseproxy)                                      | 实现反向代理。                                                                                           |
| [Swagger](https://github.com/hertz-contrib/swagger)                                                | 使用 Swagger 2.0 自动生成 RESTful API 文档。                                                               |
| [Cache](https://github.com/hertz-contrib/cache)                                                    | 用于缓存 HTTP 接口内容的 Hertz 中间件，支持多种客户端。                                                                |

## 相关文章
- [字节跳动在 Go 网络库上的实践](https://www.cloudwego.io/blog/2021/10/09/bytedance-practices-on-go-network-library/)
- [超大规模的企业级微服务 HTTP 框架 — Hertz 正式开源！](https://www.cloudwego.io/zh/blog/2022/06/21/%E8%B6%85%E5%A4%A7%E8%A7%84%E6%A8%A1%E7%9A%84%E4%BC%81%E4%B8%9A%E7%BA%A7%E5%BE%AE%E6%9C%8D%E5%8A%A1-http-%E6%A1%86%E6%9E%B6-hertz-%E6%AD%A3%E5%BC%8F%E5%BC%80%E6%BA%90/)
- [字节跳动开源 Go HTTP 框架 Hertz 设计实践](https://www.cloudwego.io/zh/blog/2022/06/21/%E5%AD%97%E8%8A%82%E8%B7%B3%E5%8A%A8%E5%BC%80%E6%BA%90-go-http-%E6%A1%86%E6%9E%B6-hertz-%E8%AE%BE%E8%AE%A1%E5%AE%9E%E8%B7%B5/)
- [助力字节降本增效，大规模企业级 HTTP 框架 Hertz 设计实践](https://www.cloudwego.io/zh/blog/2022/09/27/%E5%8A%A9%E5%8A%9B%E5%AD%97%E8%8A%82%E9%99%8D%E6%9C%AC%E5%A2%9E%E6%95%88%E5%A4%A7%E8%A7%84%E6%A8%A1%E4%BC%81%E4%B8%9A%E7%BA%A7-http-%E6%A1%86%E6%9E%B6-hertz-%E8%AE%BE%E8%AE%A1%E5%AE%9E%E8%B7%B5/)
- [HTTP 框架 Hertz 实践入门：性能测试指南](https://www.cloudwego.io/zh/blog/2022/11/01/http-%E6%A1%86%E6%9E%B6-hertz-%E5%AE%9E%E8%B7%B5%E5%85%A5%E9%97%A8%E6%80%A7%E8%83%BD%E6%B5%8B%E8%AF%95%E6%8C%87%E5%8D%97/)
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
- Slack: 加入我们的 [Slack 频道](https://join.slack.com/t/cloudwego/shared_invite/zt-tmcbzewn-UjXMF3ZQsPhl7W3tEDZboA)
- 飞书用户群（[注册飞书](https://www.larksuite.com/zh_cn/download)进群）

  ![LarkGroup](images/lark_group_cn.png)

## 贡献者
感谢您对 Hertz 作出的贡献！

[![Contributors](https://contrib.rocks/image?repo=cloudwego/hertz)](https://github.com/cloudwego/hertz/graphs/contributors)
## Landscapes

<p align="center">
<img src="https://landscape.cncf.io/images/left-logo.svg" width="150"/>&nbsp;&nbsp;<img src="https://landscape.cncf.io/images/right-logo.svg" width="200"/>
<br/><br/>
CloudWeGo 丰富了 <a href="https://landscape.cncf.io/">CNCF 云原生生态</a>。
</p>
