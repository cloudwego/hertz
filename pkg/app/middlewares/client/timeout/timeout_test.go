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

package timeout

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/config"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route"
)

func TestTimeout(t *testing.T) {
	body := "foo"
	c, _ := client.NewClient()
	c.Use(CtxTimeout)
	opt := config.NewOptions([]config.Option{})
	opt.Addr = ":10099"
	opt.Network = "tcp"
	engine := route.NewEngine(opt)

	engine.GET("/baz", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(time.Second * 2)
		ctx.WriteString(body) //nolint:errcheck
	})
	go engine.Run()
	time.Sleep(time.Second)

	ctxA, cancelA := context.WithTimeout(context.Background(), time.Second*3)
	defer func() {
		cancelA()
	}()
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	req.SetRequestURI("http://127.0.0.1:10099/baz")
	err := c.Do(ctxA, req, resp)
	assert.Nil(t, err)
	assert.DeepEqual(t, body, string(resp.Body()))

	ctxB, cancelB := context.WithTimeout(context.Background(), time.Second)
	defer func() {
		cancelB()
	}()
	err = c.Do(ctxB, req, resp)
	assert.DeepEqual(t, ErrTimeout, err)
}

func TestHostClientMaxConnsWithDeadline(t *testing.T) {
	var (
		emptyBodyCount uint8
		timeout        = 50 * time.Millisecond
		wg             sync.WaitGroup
	)
	opt := config.NewOptions([]config.Option{})

	opt.Addr = "unix-test-10015"
	opt.Network = "unix"
	engine := route.NewEngine(opt)

	engine.POST("/baz", func(c context.Context, ctx *app.RequestContext) {
		if len(ctx.Request.Body()) == 0 {
			emptyBodyCount++
		}

		ctx.WriteString("foo") //nolint:errcheck
	})
	go engine.Run()
	defer func() {
		engine.Close()
	}()
	time.Sleep(time.Millisecond * 500)

	c, _ := client.NewClient(client.WithDialFunc(func(addr string) (network.Conn, error) {
		d := standard.NewDialer()
		return d.DialConnection("unix", opt.Addr, timeout, nil)
	}))
	c.Use(CtxTimeout)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := protocol.AcquireRequest()
			req.SetRequestURI("http://foobar/baz")
			req.Header.SetMethod(consts.MethodPost)
			req.SetBodyString("bar")
			resp := protocol.AcquireResponse()

			for {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer func() {
					cancel()
				}()
				if err := c.Do(ctx, req, resp); err != nil {
					if err.Error() == errs.ErrNoFreeConns.Error() {
						time.Sleep(time.Millisecond * 500)
						continue
					}
					assert.Nil(t, err)
				}
				break
			}

			assert.DeepEqual(t, resp.StatusCode(), consts.StatusOK)

			body := resp.Body()
			assert.DeepEqual(t, "foo", string(body))
		}()
	}
	wg.Wait()
	assert.DeepEqual(t, int(emptyBodyCount), 0)
}
