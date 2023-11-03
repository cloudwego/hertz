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

package server

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server/binding"
	"github.com/cloudwego/hertz/pkg/app/server/registry"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/tracer"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

// WithKeepAliveTimeout sets keep-alive timeout.
//
// In most cases, there is no need to care about this option.
func WithKeepAliveTimeout(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.KeepAliveTimeout = t
	}}
}

// WithReadTimeout sets read timeout.
//
// Close the connection when read request timeout.
func WithReadTimeout(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ReadTimeout = t
	}}
}

// WithWriteTimeout sets write timeout.
//
// Connection will be closed when write request timeout.
func WithWriteTimeout(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.WriteTimeout = t
	}}
}

// WithIdleTimeout sets idle timeout.
//
// Close the connection when the successive request timeout (in keepalive mode).
// Set this to protect server from misbehavior clients.
func WithIdleTimeout(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.IdleTimeout = t
	}}
}

// WithRedirectTrailingSlash sets redirectTrailingSlash.
//
// Enables automatic redirection if the current route can't be matched but a
// handler for the path with (without) the trailing slash exists.
// For example if /foo/ is requested but a route only exists for /foo, the
// client is redirected to /foo with http status code 301 for GET requests
// and 307 for all other request methods.
func WithRedirectTrailingSlash(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.RedirectTrailingSlash = b
	}}
}

// WithRedirectFixedPath sets redirectFixedPath.
//
// If enabled, the router tries to fix the current request path, if no
// handle is registered for it.
// First superfluous path elements like ../ or // are removed.
// Afterwards the router does a case-insensitive lookup of the cleaned path.
// If a handle can be found for this route, the router makes a redirection
// to the corrected path with status code 301 for GET requests and 308 for
// all other request methods.
// For example /FOO and /..//Foo could be redirected to /foo.
// RedirectTrailingSlash is independent of this option.
func WithRedirectFixedPath(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.RedirectFixedPath = b
	}}
}

// WithHandleMethodNotAllowed sets handleMethodNotAllowed.
//
// If enabled, the router checks if another method is allowed for the
// current route, if the current request can not be routed.
// If this is the case, the request is answered with 'Method Not Allowed'
// and HTTP status code 405.
// If no other Method is allowed, the request is delegated to the NotFound
// handler.
func WithHandleMethodNotAllowed(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.HandleMethodNotAllowed = b
	}}
}

// WithUseRawPath sets useRawPath.
//
// If enabled, the url.RawPath will be used to find parameters.
func WithUseRawPath(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.UseRawPath = b
	}}
}

// WithRemoveExtraSlash sets removeExtraSlash.
//
// RemoveExtraSlash a parameter can be parsed from the URL even with extra slashes.
// If UseRawPath is false (by default), the RemoveExtraSlash effectively is true,
// as url.Path gonna be used, which is already cleaned.
func WithRemoveExtraSlash(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.RemoveExtraSlash = b
	}}
}

// WithUnescapePathValues sets unescapePathValues.
//
// If true, the path value will be unescaped.
// If UseRawPath is false (by default), the UnescapePathValues effectively is true,
// as url.Path gonna be used, which is already unescaped.
func WithUnescapePathValues(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.UnescapePathValues = b
	}}
}

// WithDisablePreParseMultipartForm sets disablePreParseMultipartForm.
//
// This option is useful for servers that desire to treat
// multipart form data as a binary blob, or choose when to parse the data.
// Server pre parses multipart form data by default.
func WithDisablePreParseMultipartForm(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.DisablePreParseMultipartForm = b
	}}
}

// WithHostPorts sets listening address.
func WithHostPorts(hp string) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.Addr = hp
	}}
}

// WithBasePath sets basePath.Must be "/" prefix and suffix,If not the default concatenate "/"
func WithBasePath(basePath string) config.Option {
	return config.Option{F: func(o *config.Options) {
		// Must be "/" prefix and suffix,If not the default concatenate "/"
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
		if !strings.HasSuffix(basePath, "/") {
			basePath = basePath + "/"
		}
		o.BasePath = basePath
	}}
}

// WithMaxRequestBodySize sets the limitation of request body size. Unit: byte
//
// Body buffer which larger than this size will be put back into buffer poll.
func WithMaxRequestBodySize(bs int) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.MaxRequestBodySize = bs
	}}
}

// WithMaxKeepBodySize sets max size of request/response body to keep when recycled. Unit: byte
//
// Body buffer which larger than this size will be put back into buffer poll.
func WithMaxKeepBodySize(bs int) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.MaxKeepBodySize = bs
	}}
}

// WithGetOnly sets whether accept GET request only. Default: false
func WithGetOnly(isOnly bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.GetOnly = isOnly
	}}
}

// WithKeepAlive sets Whether use long connection. Default: true
func WithKeepAlive(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.DisableKeepalive = !b
	}}
}

// WithStreamBody determines whether read body in stream or not.
//
// StreamRequestBody enables streaming request body,
// and calls the handler sooner when given body is
// larger than the current limit.
func WithStreamBody(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.StreamRequestBody = b
	}}
}

// WithNetwork sets network. Support "tcp", "udp", "unix"(unix domain socket).
func WithNetwork(nw string) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.Network = nw
	}}
}

// WithExitWaitTime sets timeout for graceful shutdown.
//
// The server may exit ahead after all connections closed.
// All responses after shutdown will be added 'Connection: close' header.
func WithExitWaitTime(timeout time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ExitWaitTimeout = timeout
	}}
}

// WithTLS sets TLS config to start a tls server.
//
// NOTE: If a tls server is started, it won't accept non-tls request.
func WithTLS(cfg *tls.Config) config.Option {
	return config.Option{F: func(o *config.Options) {
		// If there is no explicit transporter, change it to standard one. Netpoll do not support tls yet.
		if o.TransporterNewer == nil {
			o.TransporterNewer = standard.NewTransporter
		}
		o.TLS = cfg
	}}
}

// WithListenConfig sets listener config.
func WithListenConfig(l *net.ListenConfig) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ListenConfig = l
	}}
}

// WithTransport sets which network library to use.
func WithTransport(transporter func(options *config.Options) network.Transporter) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.TransporterNewer = transporter
	}}
}

// WithAltTransport sets which network library to use as an alternative transporter(need to be implemented by specific transporter).
func WithAltTransport(transporter func(options *config.Options) network.Transporter) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.AltTransporterNewer = transporter
	}}
}

// WithH2C sets whether enable H2C.
func WithH2C(enable bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.H2C = enable
	}}
}

// WithReadBufferSize sets the read buffer size which also limit the header size.
func WithReadBufferSize(size int) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ReadBufferSize = size
	}}
}

// WithALPN sets whether enable ALPN.
func WithALPN(enable bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ALPN = enable
	}}
}

// WithTracer adds tracer to server.
func WithTracer(t tracer.Tracer) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.Tracers = append(o.Tracers, t)
	}}
}

// WithTraceLevel sets the level trace.
func WithTraceLevel(level stats.Level) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.TraceLevel = level
	}}
}

// WithRegistry sets the registry and registry's info
func WithRegistry(r registry.Registry, info *registry.Info) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.Registry = r
		o.RegistryInfo = info
	}}
}

// WithAutoReloadRender sets the config of auto reload render.
// If auto reload render is enabled:
// 1. interval = 0 means reload render according to file watch mechanism.(recommended)
// 2. interval > 0 means reload render every interval.
func WithAutoReloadRender(b bool, interval time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.AutoReloadRender = b
		o.AutoReloadInterval = interval
	}}
}

// WithDisablePrintRoute sets whether disable debugPrintRoute
// If we don't set it, it will default to false
func WithDisablePrintRoute(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.DisablePrintRoute = b
	}}
}

// WithOnAccept sets the callback function when a new connection is accepted but cannot
// receive data in netpoll. In go net, it will be called before converting tls connection
func WithOnAccept(fn func(conn net.Conn) context.Context) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.OnAccept = fn
	}}
}

// WithOnConnect sets the onConnect function. It can received data from connection in netpoll.
// In go net, it will be called after converting tls connection.
func WithOnConnect(fn func(ctx context.Context, conn network.Conn) context.Context) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.OnConnect = fn
	}}
}

// WithBindConfig sets bind config.
func WithBindConfig(bc *binding.BindConfig) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.BindConfig = bc
	}}
}

// WithValidateConfig sets validate config.
func WithValidateConfig(vc *binding.ValidateConfig) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ValidateConfig = vc
	}}
}

// WithCustomBinder sets customized Binder.
func WithCustomBinder(b binding.Binder) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.CustomBinder = b
	}}
}

// WithCustomValidator sets customized Binder.
func WithCustomValidator(b binding.StructValidator) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.CustomValidator = b
	}}
}

// WithDisableHeaderNamesNormalizing is used to set whether disable header names normalizing.
func WithDisableHeaderNamesNormalizing(disable bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.DisableHeaderNamesNormalizing = disable
	}}
}

func WithDisableDefaultDate(disable bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.NoDefaultDate = disable
	}}
}

func WithDisableDefaultContentType(disable bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.NoDefaultContentType = disable
	}}
}
