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

package client

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/timer"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const defaultMaxRedirectsCount = 16

var (
	errTimeout          = errors.New(errors.ErrTimeout, errors.ErrorTypePublic, "host client")
	errMissingLocation  = errors.NewPublic("missing Location header for http redirect")
	errTooManyRedirects = errors.NewPublic("too many redirects detected when doing the request")

	clientURLResponseChPool sync.Pool

	errorChPool sync.Pool
)

type HostClient interface {
	Doer
	SetDynamicConfig(dc *DynamicConfig)
	CloseIdleConnections()
	ShouldRemove() bool
	ConnectionCount() int
}

type Doer interface {
	Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error
}

type HostClientConfig struct {
	DynamicConfig

	Name string

	NoDefaultUserAgentHeader      bool
	DialDualStack                 bool
	DisableHeaderNamesNormalizing bool
	DisablePathNormalizing        bool
	IsTLS                         bool

	TLSConfig *tls.Config

	MaxConns                  int
	MaxIdempotentCallAttempts int
	MaxResponseBodySize       int

	Dial               DialFunc
	ResponseBodyStream bool

	DialTimeout         time.Duration
	MaxIdleConnDuration time.Duration
	MaxConnDuration     time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	MaxConnWaitTimeout  time.Duration
}

// DynamicConfig is config set which will be confirmed when starts a request.
type DynamicConfig struct {
	Addr     string
	ProxyURI *protocol.URI
	IsTLS    bool
}

// DialFunc must establish connection to addr.
//
// There is no need in establishing TLS (SSL) connection for https.
// The client automatically converts connection to TLS
// if HostClient.IsTLS is set.
//
// TCP address passed to DialFunc always contains host and port.
// Example TCP addr values:
//
//   - foobar.com:80
//   - foobar.com:443
//   - foobar.com:8080
type DialFunc func(addr string) (network.Conn, error)

type clientURLResponse struct {
	statusCode int
	body       []byte
	err        error
}

func GetURL(ctx context.Context, dst []byte, url string, c Doer, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	req := protocol.AcquireRequest()
	req.SetOptions(requestOptions...)

	statusCode, body, err = doRequestFollowRedirectsBuffer(ctx, req, dst, url, c)

	protocol.ReleaseRequest(req)
	return statusCode, body, err
}

func GetURLTimeout(ctx context.Context, dst []byte, url string, timeout time.Duration, c Doer, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	deadline := time.Now().Add(timeout)
	return GetURLDeadline(ctx, dst, url, deadline, c, requestOptions...)
}

func GetURLDeadline(ctx context.Context, dst []byte, url string, deadline time.Time, c Doer, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	timeout := -time.Since(deadline)
	if timeout <= 0 {
		return 0, dst, errTimeout
	}

	var ch chan clientURLResponse
	chv := clientURLResponseChPool.Get()
	if chv == nil {
		chv = make(chan clientURLResponse, 1)
	}
	ch = chv.(chan clientURLResponse)

	req := protocol.AcquireRequest()
	req.SetOptions(requestOptions...)

	// Note that the request continues execution on errTimeout until
	// client-specific ReadTimeout exceeds. This helps to limit load
	// on slow hosts by MaxConns* concurrent requests.
	//
	// Without this 'hack' the load on slow host could exceed MaxConns*
	// concurrent requests, since timed out requests on client side
	// usually continue execution on the host.
	go func() {
		statusCodeCopy, bodyCopy, errCopy := doRequestFollowRedirectsBuffer(ctx, req, dst, url, c)
		ch <- clientURLResponse{
			statusCode: statusCodeCopy,
			body:       bodyCopy,
			err:        errCopy,
		}
	}()

	tc := timer.AcquireTimer(timeout)
	select {
	case resp := <-ch:
		protocol.ReleaseRequest(req)
		clientURLResponseChPool.Put(chv)
		statusCode = resp.statusCode
		body = resp.body
		err = resp.err
	case <-tc.C:
		body = dst
		err = errTimeout
	}
	timer.ReleaseTimer(tc)

	return statusCode, body, err
}

func PostURL(ctx context.Context, dst []byte, url string, postArgs *protocol.Args, c Doer, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	req := protocol.AcquireRequest()
	req.Header.SetMethodBytes(bytestr.StrPost)
	req.Header.SetContentTypeBytes(bytestr.StrPostArgsContentType)
	req.SetOptions(requestOptions...)

	if postArgs != nil {
		if _, err := postArgs.WriteTo(req.BodyWriter()); err != nil {
			return 0, nil, err
		}
	}

	statusCode, body, err = doRequestFollowRedirectsBuffer(ctx, req, dst, url, c)

	protocol.ReleaseRequest(req)
	return statusCode, body, err
}

func doRequestFollowRedirectsBuffer(ctx context.Context, req *protocol.Request, dst []byte, url string, c Doer) (statusCode int, body []byte, err error) {
	resp := protocol.AcquireResponse()
	bodyBuf := resp.BodyBuffer()
	oldBody := bodyBuf.B
	bodyBuf.B = dst

	statusCode, _, err = DoRequestFollowRedirects(ctx, req, resp, url, defaultMaxRedirectsCount, c)

	body = bodyBuf.B
	bodyBuf.B = oldBody
	protocol.ReleaseResponse(resp)

	return statusCode, body, err
}

func DoRequestFollowRedirects(ctx context.Context, req *protocol.Request, resp *protocol.Response, url string, maxRedirectsCount int, c Doer) (statusCode int, body []byte, err error) {
	redirectsCount := 0

	for {
		req.SetRequestURI(url)
		req.ParseURI()

		if err = c.Do(ctx, req, resp); err != nil {
			break
		}
		statusCode = resp.Header.StatusCode()
		if !StatusCodeIsRedirect(statusCode) {
			break
		}

		redirectsCount++
		if redirectsCount > maxRedirectsCount {
			err = errTooManyRedirects
			break
		}
		location := resp.Header.PeekLocation()
		if len(location) == 0 {
			err = errMissingLocation
			break
		}
		url = getRedirectURL(url, location)
	}

	return statusCode, body, err
}

// StatusCodeIsRedirect returns true if the status code indicates a redirect.
func StatusCodeIsRedirect(statusCode int) bool {
	return statusCode == consts.StatusMovedPermanently ||
		statusCode == consts.StatusFound ||
		statusCode == consts.StatusSeeOther ||
		statusCode == consts.StatusTemporaryRedirect ||
		statusCode == consts.StatusPermanentRedirect
}

func getRedirectURL(baseURL string, location []byte) string {
	u := protocol.AcquireURI()
	u.Update(baseURL)
	u.UpdateBytes(location)
	redirectURL := u.String()
	protocol.ReleaseURI(u)
	return redirectURL
}

func DoTimeout(ctx context.Context, req *protocol.Request, resp *protocol.Response, timeout time.Duration, c Doer) error {
	deadline := time.Now().Add(timeout)
	return DoDeadline(ctx, req, resp, deadline, c)
}

func DoDeadline(ctx context.Context, req *protocol.Request, resp *protocol.Response, deadline time.Time, c Doer) error {
	timeout := -time.Since(deadline)
	if timeout <= 0 {
		return errTimeout
	}

	var ch chan error
	chv := errorChPool.Get()
	if chv == nil {
		chv = make(chan error, 1)
	}
	ch = chv.(chan error)

	// Make req and resp copies, since on timeout they no longer
	// may be accessed.
	reqCopy := protocol.AcquireRequest()
	req.CopyToSkipBody(reqCopy)
	protocol.SwapRequestBody(req, reqCopy)
	respCopy := protocol.AcquireResponse()
	if resp != nil {
		// Not calling resp.copyToSkipBody(respCopy) here to avoid
		// unexpected messing with headers
		respCopy.SkipBody = resp.SkipBody
	}

	// Note that the request continues execution on errTimeout until
	// client-specific ReadTimeout exceeds. This helps limiting load
	// on slow hosts by MaxConns* concurrent requests.
	//
	// Without this 'hack' the load on slow host could exceed MaxConns*
	// concurrent requests, since timed out requests on client side
	// usually continue execution on the host.

	var mu sync.Mutex
	var timedout bool

	go func() {
		errDo := c.Do(ctx, reqCopy, respCopy)
		mu.Lock()
		if !timedout {
			if resp != nil {
				respCopy.CopyToSkipBody(resp)
				protocol.SwapResponseBody(resp, respCopy)
			}
			protocol.SwapRequestBody(reqCopy, req)
			ch <- errDo
		}
		mu.Unlock()

		protocol.ReleaseResponse(respCopy)
		protocol.ReleaseRequest(reqCopy)
	}()

	tc := timer.AcquireTimer(timeout)
	var err error
	select {
	case err = <-ch:
	case <-tc.C:
		mu.Lock()
		timedout = true
		err = errTimeout
		mu.Unlock()
	}
	timer.ReleaseTimer(tc)

	errorChPool.Put(chv)

	return err
}
