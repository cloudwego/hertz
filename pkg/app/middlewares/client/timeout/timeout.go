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
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik Dubbelboer, FastHTTP Authors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package timeout

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/protocol"
)

var (
	ErrTimeout  = errors.New(errors.ErrTimeout, errors.ErrorTypePublic, "ctx timeout middleware")
	errorChPool sync.Pool
)

func CtxTimeout(next client.Endpoint) client.Endpoint {
	return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
		if d, ok := ctx.Deadline(); ok {
			if time.Since(d) > 0 {
				return ErrTimeout
			}
		}

		var ch chan error
		chv := errorChPool.Get()
		if chv == nil {
			chv = make(chan error, 1)
		}
		ch = chv.(chan error)

		reqCopy := protocol.AcquireRequest()
		req.CopyToSkipBody(reqCopy)
		protocol.SwapRequestBody(req, reqCopy)
		respCopy := protocol.AcquireResponse()
		if resp != nil {
			respCopy.SkipBody = resp.SkipBody
		}

		var mu sync.Mutex
		var timeout, responded bool

		go func() {
			errNext := next(ctx, reqCopy, respCopy)
			mu.Lock()
			{
				if !timeout {
					if resp != nil {
						respCopy.CopyToSkipBody(resp)
						protocol.SwapResponseBody(resp, respCopy)
					}
					protocol.SwapRequestBody(reqCopy, req)
					ch <- errNext
					responded = true
				}
			}
			mu.Unlock()

			protocol.ReleaseResponse(respCopy)
			protocol.ReleaseRequest(reqCopy)
		}()

		select {
		case err = <-ch:
		case <-ctx.Done():
			mu.Lock()
			{
				if responded {
					err = <-ch
				} else {
					timeout = true
					err = ErrTimeout
				}
			}
			mu.Unlock()
		}

		errorChPool.Put(chv)
		return err
	}
}
