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

package ut

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route"
)

func newTestEngine() *route.Engine {
	opt := config.NewOptions([]config.Option{})
	return route.NewEngine(opt)
}

func TestPerformRequest(t *testing.T) {
	router := newTestEngine()
	router.PUT("/hey/:user", func(ctx context.Context, c *app.RequestContext) {
		user := c.Param("user")
		if string(c.Request.Body()) == "1" {
			assert.DeepEqual(t, "close", c.Request.Header.Get("Connection"))
			c.Response.SetConnectionClose()
			c.JSON(consts.StatusCreated, map[string]string{"hi": user})
		} else if string(c.Request.Body()) == "" {
			c.AbortWithMsg("unauthorized", consts.StatusUnauthorized)
		} else {
			assert.DeepEqual(t, "PUT /hey/dy HTTP/1.1\r\nContent-Type: application/x-www-form-urlencoded\r\nTransfer-Encoding: chunked\r\n\r\n", string(c.Request.Header.Header()))
			c.String(consts.StatusAccepted, "body:%v", string(c.Request.Body()))
		}
	})
	router.GET("/her/header", func(ctx context.Context, c *app.RequestContext) {
		assert.DeepEqual(t, "application/json", string(c.GetHeader("Content-Type")))
		assert.DeepEqual(t, 1, c.Request.Header.ContentLength())
		assert.DeepEqual(t, "a", c.Request.Header.Get("dummy"))
	})

	// valid user
	w := PerformRequest(router, "PUT", "/hey/dy", &Body{bytes.NewBufferString("1"), 1},
		Header{"Connection", "close"})
	resp := w.Result()
	assert.DeepEqual(t, consts.StatusCreated, resp.StatusCode())
	assert.DeepEqual(t, "{\"hi\":\"dy\"}", string(resp.Body()))
	assert.DeepEqual(t, "application/json; charset=utf-8", string(resp.Header.ContentType()))
	assert.DeepEqual(t, true, resp.Header.ConnectionClose())

	// unauthorized user
	w = PerformRequest(router, "PUT", "/hey/dy", nil)
	_ = w.Result()
	resp = w.Result()
	assert.DeepEqual(t, consts.StatusUnauthorized, resp.StatusCode())
	assert.DeepEqual(t, "unauthorized", string(resp.Body()))
	assert.DeepEqual(t, "text/plain; charset=utf-8", string(resp.Header.ContentType()))
	assert.DeepEqual(t, 12, resp.Header.ContentLength())

	// special header
	PerformRequest(router, "GET", "/hey/header", nil,
		Header{"content-type", "application/json"},
		Header{"content-length", "1"},
		Header{"dummy", "a"},
		Header{"dummy", "b"},
	)

	// not found
	w = PerformRequest(router, "GET", "/hey", nil)
	resp = w.Result()
	assert.DeepEqual(t, consts.StatusNotFound, resp.StatusCode())

	// fake body
	w = PerformRequest(router, "GET", "/hey", nil)
	_, err := w.WriteString(", faker")
	resp = w.Result()
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusNotFound, resp.StatusCode())
	assert.DeepEqual(t, "404 page not found, faker", string(resp.Body()))

	// chunked body
	body := bytes.NewReader(createChunkedBody([]byte("hello world!")))
	w = PerformRequest(router, "PUT", "/hey/dy", &Body{body, -1})
	resp = w.Result()
	assert.DeepEqual(t, consts.StatusAccepted, resp.StatusCode())
	assert.DeepEqual(t, "body:1\r\nh\r\n2\r\nel\r\n3\r\nlo \r\n4\r\nworl\r\n2\r\nd!\r\n0\r\n\r\n", string(resp.Body()))
}

func createChunkedBody(body []byte) []byte {
	var b []byte
	chunkSize := 1
	for len(body) > 0 {
		if chunkSize > len(body) {
			chunkSize = len(body)
		}
		b = append(b, []byte(fmt.Sprintf("%x\r\n", chunkSize))...)
		b = append(b, body[:chunkSize]...)
		b = append(b, []byte("\r\n")...)
		body = body[chunkSize:]
		chunkSize++
	}
	return append(b, []byte("0\r\n\r\n")...)
}
