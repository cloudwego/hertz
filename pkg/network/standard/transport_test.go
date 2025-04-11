/*
 * Copyright 2025 CloudWeGo Authors
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
	"io"
	"net"
	"testing"
	"time"

	internalNetwork "github.com/cloudwego/hertz/internal/network"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network"
)

func assertWriteRead(t *testing.T, c io.ReadWriter, w, r string) {
	t.Helper()
	_, err := c.Write([]byte(w))
	if err != nil {
		t.Fatal("write err", err)
	}
	b := make([]byte, len(r))
	_, err = io.ReadFull(c, b)
	if err != nil {
		t.Fatal("read err", err)
	}
	if s := string(b); s != r {
		t.Fatal("read", r)
	}
}

func TestTransporter(t *testing.T) {
	handlerExit := make(chan struct{})
	req := "hello"
	resp := "world"
	trans := NewTransporter(&config.Options{Network: "tcp", Addr: "127.0.0.1:0", SenseClientDisconnection: true}).(*transport)
	go trans.ListenAndServe(func(ctx context.Context, conn interface{}) error {
		_, isStatefulConn := conn.(internalNetwork.StatefulConn)
		if !isStatefulConn {
			t.Fatal("SenseClientDisconnection configure failed")
		}
		c := conn.(network.Conn)
		defer c.Close()
		assertWriteRead(t, c, resp, req)
		<-handlerExit
		return nil
	})
	for trans.Listener() == nil { // wait server up
		time.Sleep(5 * time.Millisecond)
	}

	// dial and test
	c, err := net.Dial("tcp", trans.Listener().Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	assertWriteRead(t, c, req, resp)

	checkActiveConn := func(n int) {
		if v := trans.updateActive(0); v != int32(n) {
			t.Helper()
			t.Fatal("trans active conn", v)
		}
	}
	checkActiveConn(1)

	// make sure shutdownTimeout will be reset after this test
	defer func(old time.Duration) { shutdownTimeout = old }(shutdownTimeout)
	defer func(old time.Duration) { shutdownTicker = old }(shutdownTicker)

	shutdownTicker = time.Millisecond // shorter for saving test time

	// case: wait util shutdownTimeout
	shutdownTimeout = time.Millisecond
	if err := trans.Shutdown(context.Background()); err != errShutdownTimeout {
		t.Fatal(err)
	}

	// case: ctx done
	shutdownTimeout = time.Second // long enough
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = trans.Shutdown(ctx)
	if err != ctx.Err() {
		t.Fatal(err)
	}

	// case: even after listener is closed, handler may still active
	checkActiveConn(1)

	// case: Shutdown blocks at ticker and check periodically.
	shutdownTimeout = time.Second // long enough
	go trans.Shutdown(context.Background())
	time.Sleep(30 * time.Millisecond) // make sure Shutdown blocks at for loop
	close(handlerExit)                // signal handler to return
	time.Sleep(10 * time.Millisecond) // wait handler returns, and active conn to be updated.
	checkActiveConn(0)
}
