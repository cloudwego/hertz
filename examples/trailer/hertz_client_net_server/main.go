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
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func main() {
	http.HandleFunc("/trailer", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Printf("server: %q\n", body)
		fmt.Println("server: ", r.Trailer.Get("AAB"))
		fmt.Println("server: ", r.Trailer.Get("Hertz"))
		fmt.Println()

		w.Header().Set("Trailer", "Aaa, Hertz")
		io.WriteString(w, "Hello World\n")
		w.Header().Set("AAA", "hertz")
		w.Header().Set("Hertz", "trailer_test")

		// the header will not be parsed by hertz
		w.Header().Set(http.TrailerPrefix+"HertzNotDeclare", "trailer_test")
	})

	go http.ListenAndServe("127.0.0.1:8080", nil)

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

func visitSingle(k, v []byte) {
	fmt.Printf("%q: %q\n", k, v)
}
