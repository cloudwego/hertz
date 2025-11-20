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

package server

import (
	"bufio"
	"context"
	"net"
	"testing"

	"github.com/cloudwego/hertz/internal/testutils"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

func BenchmarkServerHelloWorld(b *testing.B) {
	ln := testutils.NewTestListener(b)
	defer ln.Close()

	h := Default(WithListener(ln), WithTransport(standard.NewTransporter))
	h.GET("/hello", func(c context.Context, ctx *app.RequestContext) {
		ctx.SetBodyString("hello world")
	})

	go h.Run()
	waitEngineRunning(h)
	defer h.Engine.Close()

	addr := ln.Addr().String()

	// Pre-create connection pool with keep-alive
	const poolSize = 10
	connPool := make([]net.Conn, poolSize)
	readerPool := make([]*bufio.Reader, poolSize)
	for i := 0; i < poolSize; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatalf("failed to dial: %s", err)
		}
		connPool[i] = conn
		readerPool[i] = bufio.NewReader(conn)
		defer conn.Close()
	}

	request := []byte("GET /hello HTTP/1.1\r\nHost: localhost\r\nConnection: keep-alive\r\n\r\n")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn := connPool[i%poolSize]
		reader := readerPool[i%poolSize]
		_, err := conn.Write(request)
		if err != nil {
			b.Fatalf("write error: %s", err)
		}
		_, err = reader.Peek(1)
		if err != nil {
			b.Fatal(err)
		}
		_, err = reader.Discard(reader.Buffered())
		if err != nil {
			b.Fatal(err)
		}
	}
}
