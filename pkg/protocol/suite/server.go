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
)

const (
	// must be the same with the ALPN nextProto values
	HTTP1 = "http/1.1"
	HTTP2 = "h2"
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

type Config struct {
	configMap map[string]ServerFactory
}

type ServerMap map[string]protocol.Server

func (c *Config) Add(protocol string, factory ServerFactory) {
	if fac := c.configMap[protocol]; fac != nil {
		hlog.SystemLogger().Warnf("ServerFactory of protocol: %s will be overridden by customized function", protocol)
	}
	c.configMap[protocol] = factory
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
	return c.configMap[protocol].New(core)
}

func (c *Config) LoadAll(core Core) (serverMap ServerMap, err error) {
	serverMap = make(ServerMap)
	var server protocol.Server
	for protocol := range c.configMap {
		if server, err = c.configMap[protocol].New(core); err != nil {
			return nil, err
		} else {
			serverMap[protocol] = server
		}
	}
	return serverMap, nil
}

// New return an empty Config suite, use .Add() to add protocol impl
func New() *Config {
	c := &Config{configMap: make(map[string]ServerFactory)}
	return c
}
