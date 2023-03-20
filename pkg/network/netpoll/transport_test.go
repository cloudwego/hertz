// Copyright 2023 CloudWeGo Authors
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
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/network"
	"golang.org/x/sys/unix"
)

func TestTransport(t *testing.T) {
	const nw = "tcp"
	const addr = "localhost:10103"
	t.Run("TestDefault", func(t *testing.T) {
		var onConnFlag, onAcceptFlag, onDataFlag int32
		transporter := NewTransporter(&config.Options{
			Addr:    addr,
			Network: nw,
			OnConnect: func(ctx context.Context, conn network.Conn) context.Context {
				atomic.StoreInt32(&onConnFlag, 1)
				return ctx
			},
			WriteTimeout: time.Second,
			OnAccept: func(conn net.Conn) context.Context {
				atomic.StoreInt32(&onAcceptFlag, 1)
				return context.Background()
			},
		})
		go transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
			atomic.StoreInt32(&onDataFlag, 1)
			return nil
		})
		defer transporter.Close()
		time.Sleep(100 * time.Millisecond)

		dial := NewDialer()
		conn, err := dial.DialConnection(nw, addr, time.Second, nil)
		assert.Nil(t, err)
		_, err = conn.Write([]byte("123"))
		assert.Nil(t, err)
		time.Sleep(100 * time.Millisecond)

		assert.Assert(t, atomic.LoadInt32(&onConnFlag) == 1)
		assert.Assert(t, atomic.LoadInt32(&onAcceptFlag) == 1)
		assert.Assert(t, atomic.LoadInt32(&onDataFlag) == 1)
	})

	t.Run("TestListenConfig", func(t *testing.T) {
		listenCfg := &net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
		}}
		transporter := NewTransporter(&config.Options{
			Addr:         addr,
			Network:      nw,
			ListenConfig: listenCfg,
		})
		go transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
			return nil
		})
		defer transporter.Close()
	})

	t.Run("TestExceptionCase", func(t *testing.T) {
		assert.Panic(t, func() { // listen err
			transporter := NewTransporter(&config.Options{
				Network: "unknow",
			})
			transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
				return nil
			})
		})
	})
}
