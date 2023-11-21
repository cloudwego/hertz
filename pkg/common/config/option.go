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

package config

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server/registry"
	"github.com/cloudwego/hertz/pkg/network"
)

// Option is the only struct that can be used to set Options.
type Option struct {
	F func(o *Options)
}

const (
	defaultKeepAliveTimeout   = 1 * time.Minute
	defaultReadTimeout        = 3 * time.Minute
	defaultAddr               = ":8888"
	defaultNetwork            = "tcp"
	defaultBasePath           = "/"
	defaultMaxRequestBodySize = 4 * 1024 * 1024
	defaultWaitExitTimeout    = time.Second * 5
	defaultReadBufferSize     = 4 * 1024
)

type Options struct {
	KeepAliveTimeout             time.Duration
	ReadTimeout                  time.Duration
	WriteTimeout                 time.Duration
	IdleTimeout                  time.Duration
	RedirectTrailingSlash        bool
	MaxRequestBodySize           int
	MaxKeepBodySize              int
	GetOnly                      bool
	DisableKeepalive             bool
	RedirectFixedPath            bool
	HandleMethodNotAllowed       bool
	UseRawPath                   bool
	RemoveExtraSlash             bool
	UnescapePathValues           bool
	DisablePreParseMultipartForm bool
	NoDefaultDate                bool
	NoDefaultContentType         bool
	StreamRequestBody            bool
	NoDefaultServerHeader        bool
	DisablePrintRoute            bool
	Network                      string
	Addr                         string
	BasePath                     string
	ExitWaitTimeout              time.Duration
	TLS                          *tls.Config
	H2C                          bool
	ReadBufferSize               int
	ALPN                         bool
	Tracers                      []interface{}
	TraceLevel                   interface{}
	ListenConfig                 *net.ListenConfig
	BindConfig                   interface{}
	ValidateConfig               interface{}
	CustomBinder                 interface{}
	CustomValidator              interface{}

	// TransporterNewer is the function to create a transporter.
	TransporterNewer    func(opt *Options) network.Transporter
	AltTransporterNewer func(opt *Options) network.Transporter

	// In netpoll library, OnAccept is called after connection accepted
	// but before adding it to epoll. OnConnect is called after adding it to epoll.
	// The difference is that onConnect can get data but OnAccept cannot.
	// If you'd like to check whether the peer IP is in the blacklist, you can use OnAccept.
	// In go net, OnAccept is executed after connection accepted but before establishing
	// tls connection. OnConnect is executed after establishing tls connection.
	OnAccept  func(conn net.Conn) context.Context
	OnConnect func(ctx context.Context, conn network.Conn) context.Context

	// Registry is used for service registry.
	Registry registry.Registry
	// RegistryInfo is base info used for service registry.
	RegistryInfo *registry.Info
	// Enable automatically HTML template reloading mechanism.

	AutoReloadRender bool
	// If AutoReloadInterval is set to 0(default).
	// The HTML template will reload according to files' changing event
	// otherwise it will reload after AutoReloadInterval.
	AutoReloadInterval time.Duration

	// Header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// responses to other clients expecting case-sensitive header names.
	//
	// By default, request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool
}

func (o *Options) Apply(opts []Option) {
	for _, op := range opts {
		op.F(o)
	}
}

func NewOptions(opts []Option) *Options {
	options := &Options{
		// Keep-alive timeout. When idle connection exceeds this time,
		// server will send keep-alive packets to ensure it's a validated
		// connection.
		//
		// NOTE: Usually there is no need to care about this value, just
		// care about IdleTimeout.
		KeepAliveTimeout: defaultKeepAliveTimeout,

		// the timeout of reading from low-level library
		ReadTimeout: defaultReadTimeout,

		// When there is no request during the idleTimeout, the connection
		// will be closed by server.
		// Default to ReadTimeout. Zero means no timeout.
		IdleTimeout: defaultReadTimeout,

		// Enables automatic redirection if the current route can't be matched but a
		// handler for the path with (without) the trailing slash exists.
		// For example if /foo/ is requested but a route only exists for /foo, the
		// client is redirected to /foo with http status code 301 for GET requests
		// and 308 for all other request methods.
		RedirectTrailingSlash: true,

		// If enabled, the router tries to fix the current request path, if no
		// handle is registered for it.
		// First superfluous path elements like ../ or // are removed.
		// Afterwards the router does a case-insensitive lookup of the cleaned path.
		// If a handle can be found for this route, the router makes a redirection
		// to the corrected path with status code 301 for GET requests and 308 for
		// all other request methods.
		// For example /FOO and /..//Foo could be redirected to /foo.
		// RedirectTrailingSlash is independent of this option.
		RedirectFixedPath: false,

		// If enabled, the router checks if another method is allowed for the
		// current route, if the current request can not be routed.
		// If this is the case, the request is answered with 'Method Not Allowed'
		// and HTTP status code 405.
		// If no other Method is allowed, the request is delegated to the NotFound
		// handler.
		HandleMethodNotAllowed: false,

		// If enabled, the url.RawPath will be used to find parameters.
		UseRawPath: false,

		// RemoveExtraSlash a parameter can be parsed from the URL even with extra slashes.
		RemoveExtraSlash: false,

		// If true, the path value will be unescaped.
		// If UseRawPath is false (by default), the UnescapePathValues effectively is true,
		// as url.Path gonna be used, which is already unescaped.
		UnescapePathValues: true,

		// ContinueHandler is called after receiving the Expect 100 Continue Header
		//
		// https://www.w3.org/Protocols/rfc2616/rfc2616-sec8.html#sec8.2.3
		// https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html#sec10.1.1
		// Using ContinueHandler a server can make decisioning on whether or not
		// to read a potentially large request body based on the headers
		//
		// The default is to automatically read request bodies of Expect 100 Continue requests
		// like they are normal requests
		DisablePreParseMultipartForm: false,

		// When set to true, causes the default Content-Type header to be excluded from the response.
		NoDefaultContentType: false,

		// When set to true, causes the default date header to be excluded from the response.
		NoDefaultDate: false,

		// Routes info printing is not disabled by default
		// Disabled when set to True
		DisablePrintRoute: false,

		// "tcp", "udp", "unix"(unix domain socket)
		Network: defaultNetwork,

		// listen address
		Addr: defaultAddr,

		// basePath
		BasePath: defaultBasePath,

		// Define the max request body size. If the body Size exceeds this value,
		// an error will be returned
		MaxRequestBodySize: defaultMaxRequestBodySize,

		// max reserved body buffer size when reset Request & Request
		// If the body size exceeds this value, then the buffer won't be put to
		// sync.Pool to prevent OOM
		MaxKeepBodySize: defaultMaxRequestBodySize,

		// only accept GET request
		GetOnly: false,

		DisableKeepalive: false,

		// request body stream switch
		StreamRequestBody: false,

		NoDefaultServerHeader: false,

		// graceful shutdown wait time
		ExitWaitTimeout: defaultWaitExitTimeout,

		// tls config
		TLS: nil,

		// Set init read buffer size. Usually there is no need to set it.
		ReadBufferSize: defaultReadBufferSize,

		// ALPN switch
		ALPN: false,

		// H2C switch
		H2C: false,

		// tracers
		Tracers: []interface{}{},

		// trace level, default LevelDetailed
		TraceLevel: new(interface{}),

		Registry: registry.NoopRegistry,

		// Disabled header names' normalization, default false
		DisableHeaderNamesNormalizing: false,
	}
	options.Apply(opts)
	return options
}
