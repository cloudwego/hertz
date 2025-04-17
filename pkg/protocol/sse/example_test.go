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
	"time"

	"github.com/cloudwego/hertz/internal/testutils"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"
)

// Example demonstrates a simple SSE server and client interaction.
func Example() {
	// --- SSE Server ---
	opt := config.NewOptions([]config.Option{})
	opt.Addr = "127.0.0.1:0"
	engine := route.NewEngine(opt)
	engine.GET("/", func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(c)
		for i := 0; i < 5; i++ {
			w.WriteEvent(fmt.Sprintf("id-%d", i), "message", []byte("hello\n\nworld"))
			time.Sleep(10 * time.Millisecond)
		}
	})
	go engine.Run()
	defer engine.Close()
	// Wait for server to start
	time.Sleep(20 * time.Millisecond)
	opt.Addr = testutils.GetListenerAddr(engine)

	// --- SSE Client ---
	c, _ := client.NewClient()
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	req.SetRequestURI("http://" + opt.Addr + "/")
	req.SetMethod("GET")
	AddAcceptMIME(req) // optional for most SSE servers
	if err := c.Do(context.Background(), req, resp); err != nil {
		panic(err)
	}
	r, err := NewReader(resp)
	if err != nil {
		panic(err)
	}
	err = r.ForEach(func(e *Event) error {
		println("Event:", e.String())
		return nil
	})
	if err != nil {
		panic(err)
	}
	// Output:
	//
}
