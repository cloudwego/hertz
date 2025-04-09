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

package testutils

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route"
)

func TestGetListener(t *testing.T) {
	msg := "world"
	e := route.NewEngine(&config.Options{Network: "tcp", Addr: "127.0.0.1:0"})
	e.GET("/hello", func(ctx context.Context, c *app.RequestContext) {
		c.String(consts.StatusOK, msg)
	})
	defer e.Shutdown(context.Background())

	go e.Run()
	time.Sleep(20 * time.Millisecond)

	type AppServer struct {
		*route.Engine
	}
	if GetURL(e, "") != GetURL(&AppServer{e}, "") {
		t.Fatal("ne")
	}

	resp, err := http.Get(GetURL(e, "/hello"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(body); s != msg {
		t.Fatal(s, "!=", msg)
	}
}
