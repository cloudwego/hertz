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

package standard

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/network"
)

type transport struct {
	// Per-connection buffer size for requests' reading.
	// This also limits the maximum header size.
	//
	// Increase this buffer if your clients send multi-KB RequestURIs
	// and/or multi-KB headers (for example, BIG cookies).
	//
	// Default buffer size is used if not set.
	readBufferSize   int
	network          string
	addr             string
	keepAliveTimeout time.Duration
	readTimeout      time.Duration
	handler          network.OnData
	tls              *tls.Config
	listenConfig     *net.ListenConfig
	OnAccept         func(conn net.Conn) context.Context
	OnConnect        func(ctx context.Context, conn network.Conn) context.Context

	// active connections. it +1 after accept and -1 after handler returns
	active int32

	mu sync.RWMutex
	ln net.Listener
}

func (t *transport) Listener() net.Listener {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ln
}

func (t *transport) serve() (err error) {
	network.UnlinkUdsFile(t.network, t.addr) //nolint:errcheck
	t.mu.Lock()
	if t.listenConfig != nil {
		t.ln, err = t.listenConfig.Listen(context.Background(), t.network, t.addr)
	} else {
		t.ln, err = net.Listen(t.network, t.addr)
	}
	// fix concurrency issue
	// normally listener must not be changed during serve()
	ln := t.ln
	t.mu.Unlock()
	if err != nil {
		return err
	}
	hlog.SystemLogger().Infof("HTTP server listening on address=%s", ln.Addr().String())
	for {
		ctx := context.Background()
		conn, err := ln.Accept()
		if err != nil {
			hlog.SystemLogger().Errorf("Error=%s", err.Error())
			return err
		}
		t.updateActive(1)

		if t.OnAccept != nil {
			ctx = t.OnAccept(conn)
		}

		var c network.Conn
		if t.tls != nil {
			c = newTLSConn(tls.Server(conn, t.tls), t.readBufferSize)
		} else {
			c = newConn(conn, t.readBufferSize)
		}

		if t.OnConnect != nil {
			ctx = t.OnConnect(ctx, c)
		}
		go func(ctx context.Context, conn network.Conn) {
			t.handler(ctx, conn)
			t.updateActive(-1)
		}(ctx, c)
	}
}

func (t *transport) updateActive(delta int32) int32 {
	return atomic.AddInt32(&t.active, delta)
}

func (t *transport) ListenAndServe(onData network.OnData) (err error) {
	t.handler = onData
	return t.serve()
}

func (t *transport) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	return t.Shutdown(ctx)
}

var (
	shutdownTimeout = 30 * time.Second
	shutdownTicker  = 10 * time.Millisecond

	errShutdownTimeout = errors.New("shutdown timeout")
)

func (t *transport) Shutdown(ctx context.Context) error {
	defer func() {
		network.UnlinkUdsFile(t.network, t.addr) //nolint:errcheck
	}()
	if ln := t.Listener(); ln != nil {
		_ = ln.Close()
	}

	tk := time.NewTicker(shutdownTicker)
	defer tk.Stop()

	// make sure t.active is updated correctly under concurrency
	<-tk.C

	// luckily the server is idle, no more active connections
	if t.updateActive(0) <= 0 {
		return nil
	}

	// check periodically to see if all connections closed
	t0 := time.Now()
	for {
		select {
		case now := <-tk.C:
			if t.updateActive(0) <= 0 {
				return nil
			}
			if now.Sub(t0) > shutdownTimeout {
				return errShutdownTimeout
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// For transporter switch
func NewTransporter(options *config.Options) network.Transporter {
	return &transport{
		readBufferSize:   options.ReadBufferSize,
		network:          options.Network,
		addr:             options.Addr,
		keepAliveTimeout: options.KeepAliveTimeout,
		readTimeout:      options.ReadTimeout,
		tls:              options.TLS,
		listenConfig:     options.ListenConfig,
		OnAccept:         options.OnAccept,
		OnConnect:        options.OnConnect,
	}
}
