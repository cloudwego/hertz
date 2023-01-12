/*
 * Copyright 2022 CloudWeGo Authors
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
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

type stream struct {
	io.Reader
	trailer http.Header
}

func (s stream) Read(p []byte) (n int, err error) {
	n, err = s.Reader.Read(p)
	if err == io.EOF {
		// the trailer will not be parsed by hertz
		s.trailer.Set("HertzNotDeclare", "test")
	}

	return
}

func main() {
	h := server.Default(server.WithHostPorts("127.0.0.1:8080"), server.WithStreamBody(true))

	h.POST("/trailer", handler)

	go h.Spin()
	time.Sleep(time.Second)

	trailer := make(map[string][]string)
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8080/trailer", stream{bytes.NewBufferString("ping"), trailer})
	req.TransferEncoding = []string{"chunked"}
	req.Trailer = trailer
	req.Trailer.Set("Hertz", "test")
	req.Trailer.Set("AAB", "hertz")

	resp, _ := http.DefaultClient.Do(req)

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("client: ", string(body))
	fmt.Println("client: ", resp.Trailer.Get("AAA"))
	fmt.Println("client: ", resp.Trailer.Get("Hertz"))
}

func handler(ctx context.Context, c *app.RequestContext) {
	fmt.Printf("server: %q\n", c.Request.Body())
	c.Request.Header.Trailer.VisitAll(visitSingle)
	fmt.Println("server: ", c.Request.Header.Trailer.Get("AAB"))
	fmt.Println("server: ", c.Request.Header.Trailer.Get("Hertz"))
	fmt.Println()

	bs := bytes.NewReader([]byte("Hello World"))
	c.SetBodyStream(bs, -1)
	c.Response.Header.Trailer.Set("AAA", "hertz")
	c.Response.Header.Trailer.Set("Hertz", "trailer_test")
}

func visitSingle(k, v []byte) {
	fmt.Printf("%q: %q\n", k, v)
}
