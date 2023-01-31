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

//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris

package server

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	c "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"golang.org/x/sys/unix"
)

func TestReusePorts(t *testing.T) {
	cfg := &net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEADDR, 1)
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		})
	}}
	ha := New(WithHostPorts("localhost:10093"), WithListenConfig(cfg), WithTransport(standard.NewTransporter))
	hb := New(WithHostPorts("localhost:10093"), WithListenConfig(cfg), WithTransport(standard.NewTransporter))
	hc := New(WithHostPorts("localhost:10093"), WithListenConfig(cfg))
	hd := New(WithHostPorts("localhost:10093"), WithListenConfig(cfg))
	ha.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, utils.H{"ping": "pong"})
	})
	hc.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, utils.H{"ping": "pong"})
	})
	hd.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, utils.H{"ping": "pong"})
	})
	hb.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, utils.H{"ping": "pong"})
	})
	go ha.Run()
	go hb.Run()
	go hc.Run()
	go hd.Run()
	time.Sleep(time.Second)

	client, _ := c.NewClient()
	for i := 0; i < 1000; i++ {
		statusCode, body, err := client.Get(context.Background(), nil, "http://localhost:10093/ping")
		assert.Nil(t, err)
		assert.DeepEqual(t, consts.StatusOK, statusCode)
		assert.DeepEqual(t, "{\"ping\":\"pong\"}", string(body))
	}
}

func TestHertz_Spin(t *testing.T) {
	engine := New(WithHostPorts("127.0.0.1:6668"))
	engine.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(time.Second * 2)
		path := ctx.Request.URI().PathOriginal()
		ctx.SetBodyString(string(path))
	})
	engine.GET("/test2", func(c context.Context, ctx *app.RequestContext) {})

	testint := uint32(0)
	engine.Engine.OnShutdown = append(engine.OnShutdown, func(ctx context.Context) {
		atomic.StoreUint32(&testint, 1)
	})

	go engine.Spin()
	time.Sleep(time.Millisecond)

	hc := http.Client{Timeout: time.Second}
	var err error
	var resp *http.Response
	ch := make(chan struct{})
	ch2 := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()
		for range ticker.C {
			_, err := hc.Get("http://127.0.0.1:6668/test2")
			t.Logf("[%v]begin listening\n", time.Now())
			if err != nil {
				t.Logf("[%v]listening closed: %v", time.Now(), err)
				ch2 <- struct{}{}
				break
			}
		}
	}()
	go func() {
		t.Logf("[%v]begin request\n", time.Now())
		resp, err = http.Get("http://127.0.0.1:6668/test")
		t.Logf("[%v]end request\n", time.Now())
		ch <- struct{}{}
	}()

	time.Sleep(time.Second * 1)
	pid := strconv.Itoa(os.Getpid())
	cmd := exec.Command("kill", "-SIGHUP", pid)
	t.Logf("[%v]begin SIGHUP\n", time.Now())
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	t.Logf("[%v]end SIGHUP\n", time.Now())
	<-ch
	assert.Nil(t, err)
	assert.NotNil(t, resp)
	assert.DeepEqual(t, uint32(1), atomic.LoadUint32(&testint))

	<-ch2
}
