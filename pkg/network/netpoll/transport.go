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

//go:build !windows
// +build !windows

package netpoll

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/netpoll"
)

func init() {
	// disable netpoll's log
	netpoll.SetLoggerOutput(io.Discard)
}

type ctxCancelKeyStruct struct{}

var ctxCancelKey = ctxCancelKeyStruct{}

func cancelContext(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, ctxCancelKey, cancel)
	return ctx
}

type transporter struct {
	senseClientDisconnection bool
	network                  string
	addr                     string
	keepAliveTimeout         time.Duration
	readTimeout              time.Duration
	writeTimeout             time.Duration
	listenConfig             *net.ListenConfig
	OnAccept                 func(conn net.Conn) context.Context
	OnConnect                func(ctx context.Context, conn network.Conn) context.Context

	mu sync.RWMutex
	ln net.Listener
	el netpoll.EventLoop
}

// For transporter switch
func NewTransporter(options *config.Options) network.Transporter {
	return &transporter{
		senseClientDisconnection: options.SenseClientDisconnection,
		network:                  options.Network,
		addr:                     options.Addr,
		keepAliveTimeout:         options.KeepAliveTimeout,
		readTimeout:              options.ReadTimeout,
		writeTimeout:             options.WriteTimeout,
		listenConfig:             options.ListenConfig,
		OnAccept:                 options.OnAccept,
		OnConnect:                options.OnConnect,
	}
}

func (t *transporter) Listener() net.Listener {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ln
}

// ListenAndServe binds listen address and keep serving, until an error occurs
// or the transport shutdowns
func (t *transporter) ListenAndServe(onReq network.OnData) (err error) {
	network.UnlinkUdsFile(t.network, t.addr) //nolint:errcheck

	t.mu.Lock()
	if t.listenConfig != nil {
		t.ln, err = t.listenConfig.Listen(context.Background(), t.network, t.addr)
	} else {
		t.ln, err = net.Listen(t.network, t.addr)
	}
	ln := t.ln
	t.mu.Unlock()

	if err != nil {
		panic("create netpoll listener fail: " + err.Error())
	}

	// Initialize custom option for EventLoop
	opts := []netpoll.Option{
		netpoll.WithIdleTimeout(t.keepAliveTimeout),
		netpoll.WithOnPrepare(func(conn netpoll.Connection) context.Context {
			conn.SetReadTimeout(t.readTimeout) // nolint:errcheck
			if t.writeTimeout > 0 {
				conn.SetWriteTimeout(t.writeTimeout)
			}
			ctx := context.Background()
			if t.OnAccept != nil {
				ctx = t.OnAccept(newConn(conn))
			}
			if t.senseClientDisconnection {
				ctx = cancelContext(ctx)
			}
			return ctx
		}),
	}

	if t.OnConnect != nil {
		opts = append(opts, netpoll.WithOnConnect(func(ctx context.Context, conn netpoll.Connection) context.Context {
			return t.OnConnect(ctx, newConn(conn))
		}))
	}

	if t.senseClientDisconnection {
		opts = append(opts, netpoll.WithOnDisconnect(func(ctx context.Context, connection netpoll.Connection) {
			cancelFunc, ok := ctx.Value(ctxCancelKey).(context.CancelFunc)
			if cancelFunc != nil && ok {
				cancelFunc()
			}
		}))
	}

	// Create EventLoop
	t.mu.Lock()
	t.el, err = netpoll.NewEventLoop(func(ctx context.Context, connection netpoll.Connection) error {
		return onReq(ctx, newConn(connection))
	}, opts...)
	eventLoop := t.el
	t.mu.Unlock()
	if err != nil {
		panic("create netpoll event-loop fail")
	}

	// Start Server
	hlog.SystemLogger().Infof("HTTP server listening on address=%s", ln.Addr().String())
	err = eventLoop.Serve(ln)
	if err != nil {
		panic("netpoll server exit")
	}

	return nil
}

// Close forces transport to close immediately (no wait timeout)
func (t *transporter) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	return t.Shutdown(ctx)
}

// Shutdown will trigger listener stop and graceful shutdown
// It will wait all connections close until reaching ctx.Deadline()
func (t *transporter) Shutdown(ctx context.Context) error {
	defer func() {
		network.UnlinkUdsFile(t.network, t.addr) //nolint:errcheck
		t.mu.RUnlock()
	}()
	t.mu.RLock()
	if t.el == nil {
		return nil
	}
	return t.el.Shutdown(ctx)
}
