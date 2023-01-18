/*
 * Copyright 2023 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func main() {
	h := server.Default(server.WithHostPorts("127.0.0.1:8080"), server.WithStreamBody(true))

	h.POST("/trailer", handler)

	go h.Spin()
	time.Sleep(time.Second)

	c, _ := client.NewClient(client.WithResponseBodyStream(true))
	req := &protocol.Request{}
	resp := &protocol.Response{}
	req.SetMethod(consts.MethodPost)
	req.SetRequestURI("http://127.0.0.1:8080/trailer")

	bs := bytes.NewReader([]byte("ping"))
	req.SetBodyStream(bs, -1)
	req.Header.Trailer().Set("AAB", "hertz")
	req.Header.Trailer().Set("Hertz", "test")

	err := c.Do(context.Background(), req, resp)
	if err != nil {
		return
	}
	fmt.Println("client: ", string(resp.Body()))
	resp.Header.Trailer().VisitAll(visitSingle)
	fmt.Println("client: ", resp.Header.Trailer().Get("AAA"))
	fmt.Println("client: ", resp.Header.Trailer().Get("Hertz"))
}

func handler(ctx context.Context, c *app.RequestContext) {
	fmt.Printf("server: %q\n", c.Request.Body())
	c.Request.Header.Trailer().VisitAll(visitSingle)
	fmt.Println("server: ", c.Request.Header.Trailer().Get("AAB"))
	fmt.Println("server: ", c.Request.Header.Trailer().Get("Hertz"))
	fmt.Println()

	bs := bytes.NewReader([]byte("Hello World"))
	c.SetBodyStream(bs, -1)
	c.Response.Header.Trailer().Set("AAA", "hertz")
	c.Response.Header.Trailer().Set("Hertz", "trailer_test")
}

func visitSingle(k, v []byte) {
	fmt.Printf("%q: %q\n", k, v)
}
