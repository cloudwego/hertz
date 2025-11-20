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

package sse

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"
)

// Example demonstrates a simple SSE server and client interaction.
func Example() {
	// --- SSE Server ---
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	opt := config.NewOptions([]config.Option{})
	opt.Listener = ln
	engine := route.NewEngine(opt)
	engine.GET("/", func(ctx context.Context, c *app.RequestContext) {
		println("Server Got LastEventID", GetLastEventID(&c.Request))
		w := NewWriter(c)
		for i := 0; i < 5; i++ {
			w.WriteEvent(fmt.Sprintf("id-%d", i), "message", []byte("hello\n\nworld"))
			time.Sleep(10 * time.Millisecond)
		}
		// [optional] it writes 0\r\n\r\n to indicate the end of chunked response
		// hertz will do it after handler returns
		w.Close()
	})
	go engine.Run()
	defer engine.Close()
	time.Sleep(20 * time.Millisecond) // wait for server to start
	opt.Addr = ln.Addr().String()

	// --- SSE Client ---
	c, _ := client.NewClient()
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	req.SetRequestURI("http://" + opt.Addr + "/")
	req.SetMethod("GET")
	req.SetHeader(LastEventIDHeader, "id-0")

	// adds `text/event-stream` to http `Accept` header
	// may required for some Model Context Protocol(MCP) servers
	AddAcceptMIME(req)

	if err := c.Do(context.Background(), req, resp); err != nil {
		panic(err)
	}
	r, err := NewReader(resp)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		// cancel can be used to force ForEach returns by closing the remote connection
		_ = cancel
	}()
	err = r.ForEach(ctx, func(e *Event) error {
		println("Event:", e.String())
		return nil
	})
	if err != nil {
		panic(err)
	}
	println("Client LastEventID", r.LastEventID())
	// Output:
	//
}
