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

package suite

import (
	"context"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/tracer"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const (
	// must be the same with the ALPN nextProto values
	HTTP1 = "http/1.1"
	HTTP2 = "h2"
	// HTTP3Draft29 is the ALPN protocol negotiated during the TLS handshake, for QUIC draft 29.
	HTTP3Draft29 = "h3-29"
	// HTTP3 is the ALPN protocol negotiated during the TLS handshake, for QUIC v1 and v2.
	HTTP3 = "h3"
)

// Core is the core interface that promises to be provided for the protocol layer extensions
type Core interface {
	// IsRunning Check whether engine is running or not
	IsRunning() bool
	// A RequestContext pool ready for protocol server impl
	GetCtxPool() *sync.Pool
	// Business logic entrance
	// After pre-read works, protocol server may call this method
	// to introduce the middlewares and handlers
	ServeHTTP(c context.Context, ctx *app.RequestContext)
	// GetTracer for tracing requirement
	GetTracer() tracer.Controller
}

type ServerFactory interface {
	New(core Core) (server protocol.Server, err error)
}

type StreamServerFactory interface {
	New(core Core) (server protocol.StreamServer, err error)
}

type Config struct {
	altServerConfig *altServerConfig
	configMap       map[string]ServerFactory
	streamConfigMap map[string]StreamServerFactory
}

type ServerMap map[string]protocol.Server

type StreamServerMap map[string]protocol.StreamServer

type altServerConfig struct {
	targetProtocol   string
	setAltHeaderFunc func(ctx context.Context, reqCtx *app.RequestContext)
}

type coreWrapper struct {
	Core
	beforeHandler func(c context.Context, ctx *app.RequestContext)
}

func (c *coreWrapper) ServeHTTP(ctx context.Context, reqCtx *app.RequestContext) {
	c.beforeHandler(ctx, reqCtx)
	c.Core.ServeHTTP(ctx, reqCtx)
}

// SetAltHeader will set response header "Alt-Svc" for the target protocol, altHeader will be the value of the header.
// Protocols other than the target protocol will carry the altHeader in the request header.
func (c *Config) SetAltHeader(target, altHeader string) {
	c.altServerConfig = &altServerConfig{
		targetProtocol: target,
		setAltHeaderFunc: func(ctx context.Context, reqCtx *app.RequestContext) {
			reqCtx.Response.Header.Add(consts.HeaderAltSvc, altHeader)
		},
	}
}

func (c *Config) Add(protocol string, factory interface{}) {
	switch factory := factory.(type) {
	case ServerFactory:
		if fac := c.configMap[protocol]; fac != nil {
			hlog.SystemLogger().Warnf("ServerFactory of protocol: %s will be overridden by customized function", protocol)
		}
		c.configMap[protocol] = factory
	case StreamServerFactory:
		if fac := c.streamConfigMap[protocol]; fac != nil {
			hlog.SystemLogger().Warnf("StreamServerFactory of protocol: %s will be overridden by customized function", protocol)
		}
		c.streamConfigMap[protocol] = factory
	default:
		hlog.SystemLogger().Fatalf("Unsupported factory type: %T", factory)
	}
}

func (c *Config) Get(name string) ServerFactory {
	return c.configMap[name]
}

func (c *Config) Delete(protocol string) {
	delete(c.configMap, protocol)
}

func (c *Config) Load(core Core, protocol string) (server protocol.Server, err error) {
	if c.configMap[protocol] == nil {
		return nil, errors.NewPrivate("HERTZ: Load server error, not support protocol: " + protocol)
	}
	if c.altServerConfig == nil || c.altServerConfig.targetProtocol == protocol {
		return c.configMap[protocol].New(core)
	}
	return c.configMap[protocol].New(&coreWrapper{Core: core, beforeHandler: c.altServerConfig.setAltHeaderFunc})
}

func (c *Config) LoadAll(core Core) (serverMap ServerMap, streamServerMap StreamServerMap, err error) {
	serverMap = make(ServerMap)
	var wrappedCore *coreWrapper
	if c.altServerConfig != nil {
		wrappedCore = &coreWrapper{Core: core, beforeHandler: c.altServerConfig.setAltHeaderFunc}
	}
	var server protocol.Server
	for proto := range c.configMap {
		if c.altServerConfig != nil && c.altServerConfig.targetProtocol != proto {
			core = wrappedCore
		}
		if server, err = c.configMap[proto].New(core); err != nil {
			return nil, nil, err
		} else {
			serverMap[proto] = server
		}
	}
	streamServerMap = make(StreamServerMap)
	var streamServer protocol.StreamServer
	for proto := range c.streamConfigMap {
		if c.altServerConfig != nil && c.altServerConfig.targetProtocol != proto {
			core = wrappedCore
		}
		if streamServer, err = c.streamConfigMap[proto].New(core); err != nil {
			return nil, nil, err
		} else {
			streamServerMap[proto] = streamServer
		}
	}
	return serverMap, streamServerMap, nil
}

// New return an empty Config suite, use .Add() to add protocol impl
func New() *Config {
	c := &Config{
		configMap:       make(map[string]ServerFactory),
		streamConfigMap: make(map[string]StreamServerFactory),
	}

	return c
}
