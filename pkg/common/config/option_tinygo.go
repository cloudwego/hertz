// Copyright 2022 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//go:build tinygo
// +build tinygo

package config

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server/registry"
	"github.com/cloudwego/hertz/pkg/network"
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
}
