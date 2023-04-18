# hz

English | [中文](guide_cn.md)


**Introduction**

`hz` is a command-line tool provided by the Hertz framework for generating code. Currently, `hz` can generate scaffolding for Hertz projects based on thrift and protobuf IDLs.

## Table of Contents

- Basic Usage

## Basic Usage

### Creating a Project

1. When executed under GOPATH, the go mod name is set to the path relative to GOPATH by default, but it can also be specified.

```shell
hz new
```

2. When executed outside of GOPATH, the go mod name must be specified. In the following command, `hertz/demo` is the 
module name.

```shell
hz new -mod hertz/demo
```

### Middlewares


| Extensions                                                                                         | Description                                                                                                                                                                    |
|----------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Autotls](https://github.com/hertz-contrib/autotls)                                                | Make Hertz support Let's Encrypt.                                                                                                                                              |
| [Http2](https://github.com/hertz-contrib/http2)                                                    | HTTP2 support for Hertz.                                                                                                                                                       |
| [Websocket](https://github.com/hertz-contrib/websocket)                                            | Enable Hertz to support the Websocket protocol.                                                                                                                                |
| [Limiter](https://github.com/hertz-contrib/limiter)                                                | Provides a current limiter based on the bbr algorithm.                                                                                                                         |
| [Monitor-prometheus](https://github.com/hertz-contrib/monitor-prometheus)                          | Provides service monitoring based on Prometheus.                                                                                                                               |
| [Obs-opentelemetry](https://github.com/hertz-contrib/obs-opentelemetry)                            | Hertz's Opentelemetry extension that supports Metric, Logger, Tracing and works out of the box.                                                                                |
| [Opensergo](https://github.com/hertz-contrib/opensergo)                                            | The Opensergo extension.                                                                                                                                                       |
| [Pprof](https://github.com/hertz-contrib/pprof)                                                    | Extension for Hertz integration with Pprof.                                                                                                                                    |
| [Registry](https://github.com/hertz-contrib/registry)                                              | Provides service registry and discovery functions. So far, the supported service discovery extensions are nacos, consul, etcd, eureka, polaris, servicecomb, zookeeper, redis. |
| [Sentry](https://github.com/hertz-contrib/hertzsentry)                                             | Sentry extension provides some unified interfaces to help users perform real-time error monitoring.                                                                            |
| [Tracer](https://github.com/hertz-contrib/tracer)                                                  | Link tracing based on Opentracing.                                                                                                                                             |
| [Basicauth](https://github.com/cloudwego/hertz/tree/develop/pkg/app/middlewares/server/basic_auth) | Basicauth middleware can provide HTTP basic authentication.                                                                                                                    |
| [Jwt](https://github.com/hertz-contrib/jwt)                                                        | Jwt extension.                                                                                                                                                                 |
| [Keyauth](https://github.com/hertz-contrib/keyauth)                                                | Provides token-based authentication.                                                                                                                                           |
| [Requestid](https://github.com/hertz-contrib/requestid)                                            | Add request id in response.                                                                                                                                                    |
| [Sessions](https://github.com/hertz-contrib/sessions)                                              | Session middleware with multi-state store support.                                                                                                                             |
| [Cors](https://github.com/hertz-contrib/cors)                                                      | Provides cross-domain resource sharing support.                                                                                                                                |
| [Csrf](https://github.com/hertz-contrib/csrf)                                                      | Csrf middleware is used to prevent cross-site request forgery attacks.                                                                                                         |
| [Secure](https://github.com/hertz-contrib/secure)                                                  | Secure middleware with multiple configuration items.                                                                                                                           |
| [Gzip](https://github.com/hertz-contrib/gzip)                                                      | A Gzip extension with multiple options.                                                                                                                                        |
| [I18n](https://github.com/hertz-contrib/i18n)                                                      | Helps translate Hertz programs into multi programming languages.                                                                                                               |
| [Lark](https://github.com/hertz-contrib/lark-hertz)                                                | Use hertz handle Lark/Feishu card message and event callback.                                                                                                                  |
| [Loadbalance](https://github.com/hertz-contrib/loadbalance)                                        | Provides load balancing algorithms for Hertz.                                                                                                                                  |
| [Logger](https://github.com/hertz-contrib/logger)                                                  | Logger extension for Hertz, which provides support for zap, logrus, zerologs logging frameworks.                                                                               |
| [Recovery](https://github.com/cloudwego/hertz/tree/develop/pkg/app/middlewares/server/recovery)    | Recovery middleware for Hertz.                                                                                                                                                 |
| [Reverseproxy](https://github.com/hertz-contrib/reverseproxy)                                      | Implement a reverse proxy.                                                                                                                                                     |
| [Swagger](https://github.com/hertz-contrib/swagger)                                                | Automatically generate RESTful API documentation with Swagger 2.0.                                                                                                             |


