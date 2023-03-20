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

package http1

import (
	"context"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/network/netpoll"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestGcBodyStream(t *testing.T) {
	srv := &http.Server{Addr: ":11001", Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for range [1024]int{} {
			w.Write([]byte("hello world\n"))
		}
	})}
	go srv.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer:             netpoll.NewDialer(),
			ResponseBodyStream: true,
		},
		Addr: "127.0.0.1:11001",
	}

	for i := 0; i < 10; i++ {
		req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
		req.SetRequestURI("http://127.0.0.1:11001")
		req.SetMethod(consts.MethodPost)
		err := c.Do(context.Background(), req, resp)
		if err != nil {
			t.Errorf("client Do error=%v", err.Error())
		}
	}

	runtime.GC()
	// wait for gc
	time.Sleep(100 * time.Millisecond)
	c.CloseIdleConnections()
	assert.DeepEqual(t, 0, c.ConnPoolState().TotalConnNum)
}
