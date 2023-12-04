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

package http1

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/internal/nocopy"
	"github.com/cloudwego/hertz/pkg/app/client/retry"
	"github.com/cloudwego/hertz/pkg/common/config"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/timer"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/dialer"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/client"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/proxy"
	reqI "github.com/cloudwego/hertz/pkg/protocol/http1/req"
	respI "github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

var (
	errConnectionClosed = errs.NewPublic("the server closed connection before returning the first response byte. " +
		"Make sure the server returns 'Connection: close' response header before closing the connection")

	errTimeout = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "host client")
)

// HostClient balances http requests among hosts listed in Addr.
//
// HostClient may be used for balancing load among multiple upstream hosts.
// While multiple addresses passed to HostClient.Addr may be used for balancing
// load among them, it would be better using LBClient instead, since HostClient
// may unevenly balance load among upstream hosts.
//
// It is forbidden copying HostClient instances. Create new instances instead.
//
// It is safe calling HostClient methods from concurrently running goroutines.
type HostClient struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	*ClientOptions

	// Comma-separated list of upstream HTTP server host addresses,
	// which are passed to Dialer in a round-robin manner.
	//
	// Each address may contain port if default dialer is used.
	// For example,
	//
	//    - foobar.com:80
	//    - foobar.com:443
	//    - foobar.com:8080
	Addr     string
	IsTLS    bool
	ProxyURI *protocol.URI

	clientName  atomic.Value
	lastUseTime uint32

	connsLock  sync.Mutex
	connsCount int
	conns      []*clientConn
	connsWait  *wantConnQueue

	addrsLock sync.Mutex
	addrs     []string
	addrIdx   uint32

	tlsConfigMap     map[string]*tls.Config
	tlsConfigMapLock sync.Mutex

	pendingRequests int32

	connsCleanerRun bool

	closed chan struct{}
}

func (c *HostClient) SetDynamicConfig(dc *client.DynamicConfig) {
	c.Addr = dc.Addr
	c.ProxyURI = dc.ProxyURI
	c.IsTLS = dc.IsTLS

	// start observation after setting addr to avoid race
	if c.StateObserve != nil {
		go func() {
			t := time.NewTicker(c.ObservationInterval)
			for {
				select {
				case <-c.closed:
					return
				case <-t.C:
					c.StateObserve(c)
				}
			}
		}()
	}
}

type clientConn struct {
	c network.Conn

	createdTime time.Time
	lastUseTime time.Time
}

var startTimeUnix = time.Now().Unix()

// LastUseTime returns time the client was last used
func (c *HostClient) LastUseTime() time.Time {
	n := atomic.LoadUint32(&c.lastUseTime)
	return time.Unix(startTimeUnix+int64(n), 0)
}

// Get returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
func (c *HostClient) Get(ctx context.Context, dst []byte, url string) (statusCode int, body []byte, err error) {
	return client.GetURL(ctx, dst, url, c)
}

func (c *HostClient) ConnectionCount() (count int) {
	c.connsLock.Lock()
	count = len(c.conns)
	c.connsLock.Unlock()
	return
}

func (c *HostClient) WantConnectionCount() (count int) {
	return c.connsWait.len()
}

func (c *HostClient) ConnPoolState() config.ConnPoolState {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	cps := config.ConnPoolState{
		PoolConnNum:  len(c.conns),
		TotalConnNum: c.connsCount,
		Addr:         c.Addr,
	}

	if c.connsWait != nil {
		cps.WaitConnNum = c.connsWait.len()
	}
	return cps
}

// GetTimeout returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// errTimeout error is returned if url contents couldn't be fetched
// during the given timeout.
func (c *HostClient) GetTimeout(ctx context.Context, dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return client.GetURLTimeout(ctx, dst, url, timeout, c)
}

// GetDeadline returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// errTimeout error is returned if url contents couldn't be fetched
// until the given deadline.
func (c *HostClient) GetDeadline(ctx context.Context, dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return client.GetURLDeadline(ctx, dst, url, deadline, c)
}

// Post sends POST request to the given url with the given POST arguments.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// Empty POST body is sent if postArgs is nil.
func (c *HostClient) Post(ctx context.Context, dst []byte, url string, postArgs *protocol.Args) (statusCode int, body []byte, err error) {
	return client.PostURL(ctx, dst, url, postArgs, c)
}

// A wantConnQueue is a queue of wantConns.
//
// inspired by net/http/transport.go
type wantConnQueue struct {
	// This is a queue, not a deque.
	// It is split into two stages - head[headPos:] and tail.
	// popFront is trivial (headPos++) on the first stage, and
	// pushBack is trivial (append) on the second stage.
	// If the first stage is empty, popFront can swap the
	// first and second stages to remedy the situation.
	//
	// This two-stage split is analogous to the use of two lists
	// in Okasaki's purely functional queue but without the
	// overhead of reversing the list when swapping stages.
	head    []*wantConn
	headPos int
	tail    []*wantConn
}

// A wantConn records state about a wanted connection
// (that is, an active call to getConn).
// The conn may be gotten by dialing or by finding an idle connection,
// or a cancellation may make the conn no longer wanted.
// These three options are racing against each other and use
// wantConn to coordinate and agree about the winning outcome.
//
// inspired by net/http/transport.go
type wantConn struct {
	ready chan struct{}
	mu    sync.Mutex // protects conn, err, close(ready)
	conn  *clientConn
	err   error
}

// DoTimeout performs the given request and waits for response during
// the given timeout duration.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// errTimeout is returned if the response wasn't returned during
// the given timeout.
//
// ErrNoFreeConns is returned if all HostClient.MaxConns connections
// to the host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
//
// Warning: DoTimeout does not terminate the request itself. The request will
// continue in the background and the response will be discarded.
// If requests take too long and the connection pool gets filled up please
// try setting a ReadTimeout.
func (c *HostClient) DoTimeout(ctx context.Context, req *protocol.Request, resp *protocol.Response, timeout time.Duration) error {
	return client.DoTimeout(ctx, req, resp, timeout, c)
}

// DoDeadline performs the given request and waits for response until
// the given deadline.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// errTimeout is returned if the response wasn't returned until
// the given deadline.
//
// ErrNoFreeConns is returned if all HostClient.MaxConns connections
// to the host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) DoDeadline(ctx context.Context, req *protocol.Request, resp *protocol.Response, deadline time.Time) error {
	return client.DoDeadline(ctx, req, resp, deadline, c)
}

// DoRedirects performs the given http request and fills the given http response,
// following up to maxRedirectsCount redirects. When the redirect count exceeds
// maxRedirectsCount, ErrTooManyRedirects is returned.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// Response is ignored if resp is nil.
//
// ErrNoFreeConns is returned if all DefaultMaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) DoRedirects(ctx context.Context, req *protocol.Request, resp *protocol.Response, maxRedirectsCount int) error {
	_, _, err := client.DoRequestFollowRedirects(ctx, req, resp, req.URI().String(), maxRedirectsCount, c)
	return err
}

// Do performs the given http request and sets the corresponding response.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrNoFreeConns is returned if all HostClient.MaxConns connections
// to the host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	var (
		err                error
		canIdempotentRetry bool
		isDefaultRetryFunc                    = true
		attempts           uint               = 0
		connAttempts       uint               = 0
		maxAttempts        uint               = 1
		isRequestRetryable client.RetryIfFunc = client.DefaultRetryIf
	)
	retryCfg := c.ClientOptions.RetryConfig
	if retryCfg != nil {
		maxAttempts = retryCfg.MaxAttemptTimes
	}

	if c.ClientOptions.RetryIfFunc != nil {
		isRequestRetryable = c.ClientOptions.RetryIfFunc
		// if the user has provided a custom retry function, the canIdempotentRetry has no meaning anymore.
		// User will have full control over the retry logic through the custom retry function.
		isDefaultRetryFunc = false
	}

	atomic.AddInt32(&c.pendingRequests, 1)
	req.Options().StartRequest()
	for {
		select {
		case <-ctx.Done():
			req.CloseBodyStream() //nolint:errcheck
			return ctx.Err()
		default:
		}

		canIdempotentRetry, err = c.do(req, resp)
		// If there is no custom retry and err is equal to nil, the loop simply exits.
		if err == nil && isDefaultRetryFunc {
			if connAttempts != 0 {
				hlog.SystemLogger().Warnf("Client connection attempt times: %d, url: %s. "+
					"This is mainly because the connection in pool is closed by peer in advance. "+
					"If this number is too high which indicates that long-connection are basically unavailable, "+
					"try to change the request to short-connection.\n", connAttempts, req.URI().FullURI())
			}
			break
		}

		// This connection is closed by the peer when it is in the connection pool.
		//
		// This case is possible if the server closes the idle
		// keep-alive connection on timeout.
		//
		// Apache and nginx usually do this.
		if canIdempotentRetry && client.DefaultRetryIf(req, resp, err) && errors.Is(err, errs.ErrBadPoolConn) {
			connAttempts++
			continue
		}

		if isDefaultRetryFunc {
			break
		}

		attempts++
		if attempts >= maxAttempts {
			break
		}

		// Check whether this request should be retried
		if !isRequestRetryable(req, resp, err) {
			break
		}

		wait := retry.Delay(attempts, err, retryCfg)
		// Retry after wait time
		time.Sleep(wait)
	}
	atomic.AddInt32(&c.pendingRequests, -1)

	if err == io.EOF {
		err = errConnectionClosed
	}
	return err
}

// PendingRequests returns the current number of requests the client
// is executing.
//
// This function may be used for balancing load among multiple HostClient
// instances.
func (c *HostClient) PendingRequests() int {
	return int(atomic.LoadInt32(&c.pendingRequests))
}

func (c *HostClient) do(req *protocol.Request, resp *protocol.Response) (bool, error) {
	nilResp := false
	if resp == nil {
		nilResp = true
		resp = protocol.AcquireResponse()
	}

	canIdempotentRetry, err := c.doNonNilReqResp(req, resp)

	if nilResp {
		protocol.ReleaseResponse(resp)
	}

	return canIdempotentRetry, err
}

type requestConfig struct {
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func (c *HostClient) preHandleConfig(o *config.RequestOptions) requestConfig {
	rc := requestConfig{
		dialTimeout:  c.DialTimeout,
		readTimeout:  c.ReadTimeout,
		writeTimeout: c.WriteTimeout,
	}
	if o.ReadTimeout() > 0 {
		rc.readTimeout = o.ReadTimeout()
	}

	if o.WriteTimeout() > 0 {
		rc.writeTimeout = o.WriteTimeout()
	}

	if o.DialTimeout() > 0 {
		rc.dialTimeout = o.DialTimeout()
	}

	return rc
}

func updateReqTimeout(reqTimeout, compareTimeout time.Duration, before time.Time) (shouldCloseConn bool, timeout time.Duration) {
	if reqTimeout <= 0 {
		return false, compareTimeout
	}
	left := reqTimeout - time.Since(before)
	if left <= 0 {
		return true, 0
	}

	if compareTimeout <= 0 {
		return false, left
	}

	if left > compareTimeout {
		return false, compareTimeout
	}

	return false, left
}

func (c *HostClient) doNonNilReqResp(req *protocol.Request, resp *protocol.Response) (bool, error) {
	if req == nil {
		panic("BUG: req cannot be nil")
	}
	if resp == nil {
		panic("BUG: resp cannot be nil")
	}

	atomic.StoreUint32(&c.lastUseTime, uint32(time.Now().Unix()-startTimeUnix))

	rc := c.preHandleConfig(req.Options())

	// Free up resources occupied by response before sending the request,
	// so the GC may reclaim these resources (e.g. response body).
	// backing up SkipBody in case it was set explicitly
	customSkipBody := resp.SkipBody
	resp.Reset()
	resp.SkipBody = customSkipBody

	if c.DisablePathNormalizing {
		req.URI().DisablePathNormalizing = true
	}
	reqTimeout := req.Options().RequestTimeout()
	begin := req.Options().StartTime()

	dialTimeout := rc.dialTimeout
	if (reqTimeout > 0 && reqTimeout < dialTimeout) || dialTimeout == 0 {
		dialTimeout = reqTimeout
	}
	cc, inPool, err := c.acquireConn(dialTimeout)
	// if getting connection error, fast fail
	if err != nil {
		return false, err
	}
	conn := cc.c

	usingProxy := false
	if c.ProxyURI != nil && bytes.Equal(req.Scheme(), bytestr.StrHTTP) {
		usingProxy = true
		proxy.SetProxyAuthHeader(&req.Header, c.ProxyURI)
	}

	resp.ParseNetAddr(conn)

	shouldClose, timeout := updateReqTimeout(reqTimeout, rc.writeTimeout, begin)
	if shouldClose {
		c.closeConn(cc)
		return false, errTimeout
	}

	if err = conn.SetWriteTimeout(timeout); err != nil {
		c.closeConn(cc)
		// try another connection if retry is enabled
		return true, err
	}

	resetConnection := false
	if c.MaxConnDuration > 0 && time.Since(cc.createdTime) > c.MaxConnDuration && !req.ConnectionClose() {
		req.SetConnectionClose()
		resetConnection = true
	}

	userAgentOld := req.Header.UserAgent()
	if len(userAgentOld) == 0 {
		req.Header.SetUserAgentBytes(c.getClientName())
	}
	zw := c.acquireWriter(conn)

	if !usingProxy {
		err = reqI.Write(req, zw)
	} else {
		err = reqI.ProxyWrite(req, zw)
	}
	if resetConnection {
		req.Header.ResetConnectionClose()
	}

	if err == nil {
		err = zw.Flush()
	}
	// error happened when writing request, close the connection, and try another connection if retry is enabled
	if err != nil {
		defer c.closeConn(cc)

		errNorm, ok := conn.(network.ErrorNormalization)
		if ok {
			err = errNorm.ToHertzError(err)
		}

		if !errors.Is(err, errs.ErrConnectionClosed) {
			return true, err
		}

		// set a protection timeout to avoid infinite loop.
		if conn.SetReadTimeout(time.Second) != nil {
			return true, err
		}

		// Only if the connection is closed while writing the request. Try to parse the response and return.
		// In this case, the request/response is considered as successful.
		// Otherwise, return the former error.
		zr := c.acquireReader(conn)
		defer zr.Release()
		if respI.ReadHeaderAndLimitBody(resp, zr, c.MaxResponseBodySize) == nil {
			return false, nil
		}

		if inPool {
			err = errs.ErrBadPoolConn
		}

		return true, err
	}

	shouldClose, timeout = updateReqTimeout(reqTimeout, rc.readTimeout, begin)
	if shouldClose {
		c.closeConn(cc)
		return false, errTimeout
	}

	// Set Deadline every time, since golang has fixed the performance issue
	// See https://github.com/golang/go/issues/15133#issuecomment-271571395 for details
	if err = conn.SetReadTimeout(timeout); err != nil {
		c.closeConn(cc)
		// try another connection if retry is enabled
		return true, err
	}

	if customSkipBody || req.Header.IsHead() || req.Header.IsConnect() {
		resp.SkipBody = true
	}
	if c.DisableHeaderNamesNormalizing {
		resp.Header.DisableNormalizing()
	}
	zr := c.acquireReader(conn)

	// errs.ErrBadPoolConn error are returned when the
	// 1 byte peek read fails, and we're actually anticipating a response.
	// Usually this is just due to the inherent keep-alive shut down race,
	// where the server closed the connection at the same time the client
	// wrote. The underlying err field is usually io.EOF or some
	// ECONNRESET sort of thing which varies by platform.
	_, err = zr.Peek(1)
	if err != nil {
		zr.Release() //nolint:errcheck
		c.closeConn(cc)
		if inPool && (err == io.EOF || err == syscall.ECONNRESET) {
			return true, errs.ErrBadPoolConn
		}
		// if this is not a pooled connection,
		// we should not retry to avoid getting stuck in an endless retry loop.
		errNorm, ok := conn.(network.ErrorNormalization)
		if ok {
			err = errNorm.ToHertzError(err)
		}
		return false, err
	}

	// init here for passing in ReadBodyStream's closure
	// and this value will be assigned after reading Response's Header
	//
	// This is to solve the circular dependency problem of Response and BodyStream
	shouldCloseConn := false

	if !c.ResponseBodyStream {
		err = respI.ReadHeaderAndLimitBody(resp, zr, c.MaxResponseBodySize)
	} else {
		err = respI.ReadBodyStream(resp, zr, c.MaxResponseBodySize, func(shouldClose bool) error {
			if shouldCloseConn || shouldClose {
				c.closeConn(cc)
			} else {
				c.releaseConn(cc)
			}
			return nil
		})
	}

	if err != nil {
		zr.Release() //nolint:errcheck
		c.closeConn(cc)
		// Don't retry in case of ErrBodyTooLarge since we will just get the same again.
		retry := !errors.Is(err, errs.ErrBodyTooLarge)
		return retry, err
	}

	zr.Release() //nolint:errcheck

	shouldCloseConn = resetConnection || req.ConnectionClose() || resp.ConnectionClose()

	// In stream mode, we still can close/release the connection immediately if there is no content on the wire.
	if c.ResponseBodyStream && resp.BodyStream() != protocol.NoResponseBody {
		return false, err
	}

	if shouldCloseConn {
		c.closeConn(cc)
	} else {
		c.releaseConn(cc)
	}

	return false, err
}

func (c *HostClient) Close() error {
	close(c.closed)
	return nil
}

// SetMaxConns sets up the maximum number of connections which may be established to all hosts listed in Addr.
func (c *HostClient) SetMaxConns(newMaxConns int) {
	c.connsLock.Lock()
	c.MaxConns = newMaxConns
	c.connsLock.Unlock()
}

func (c *HostClient) acquireConn(dialTimeout time.Duration) (cc *clientConn, inPool bool, err error) {
	createConn := false
	startCleaner := false

	var n int
	c.connsLock.Lock()
	n = len(c.conns)
	if n == 0 {
		maxConns := c.MaxConns
		if maxConns <= 0 {
			maxConns = consts.DefaultMaxConnsPerHost
		}
		if c.connsCount < maxConns {
			c.connsCount++
			createConn = true
			if !c.connsCleanerRun {
				startCleaner = true
				c.connsCleanerRun = true
			}
		}
	} else {
		n--
		cc = c.conns[n]
		c.conns[n] = nil
		c.conns = c.conns[:n]
	}
	c.connsLock.Unlock()

	if cc != nil {
		return cc, true, nil
	}
	if !createConn {
		if c.MaxConnWaitTimeout <= 0 {
			return nil, true, errs.ErrNoFreeConns
		}

		timeout := c.MaxConnWaitTimeout

		// wait for a free connection
		tc := timer.AcquireTimer(timeout)
		defer timer.ReleaseTimer(tc)

		w := &wantConn{
			ready: make(chan struct{}, 1),
		}
		defer func() {
			if err != nil {
				w.cancel(c, err)
			}
		}()

		// Note: In the case of setting MaxConnWaitTimeout, if the number
		// of connections in the connection pool exceeds the maximum
		// number of connections and needs to establish a connection while
		// waiting, the dialtimeout on the hostclient is used instead of
		// the dialtimeout in request options.
		c.queueForIdle(w)

		select {
		case <-w.ready:
			return w.conn, true, w.err
		case <-tc.C:
			return nil, true, errs.ErrNoFreeConns
		}
	}

	if startCleaner {
		go c.connsCleaner()
	}

	conn, err := c.dialHostHard(dialTimeout)
	if err != nil {
		c.decConnsCount()
		return nil, false, err
	}
	cc = acquireClientConn(conn)

	return cc, false, nil
}

func (c *HostClient) queueForIdle(w *wantConn) {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	if c.connsWait == nil {
		c.connsWait = &wantConnQueue{}
	}
	c.connsWait.clearFront()
	c.connsWait.pushBack(w)
}

func (c *HostClient) dialConnFor(w *wantConn) {
	conn, err := c.dialHostHard(c.DialTimeout)
	if err != nil {
		w.tryDeliver(nil, err)
		c.decConnsCount()
		return
	}

	cc := acquireClientConn(conn)
	delivered := w.tryDeliver(cc, nil)
	if !delivered {
		// not delivered, return idle connection
		c.releaseConn(cc)
	}
}

// CloseIdleConnections closes any connections which were previously
// connected from previous requests but are now sitting idle in a
// "keep-alive" state. It does not interrupt any connections currently
// in use.
func (c *HostClient) CloseIdleConnections() {
	c.connsLock.Lock()
	scratch := append([]*clientConn{}, c.conns...)
	for i := range c.conns {
		c.conns[i] = nil
	}
	c.conns = c.conns[:0]
	c.connsLock.Unlock()

	for _, cc := range scratch {
		c.closeConn(cc)
	}
}

func (c *HostClient) ShouldRemove() bool {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	return c.connsCount == 0
}

func (c *HostClient) connsCleaner() {
	var (
		scratch             []*clientConn
		maxIdleConnDuration = c.MaxIdleConnDuration
	)
	if maxIdleConnDuration <= 0 {
		maxIdleConnDuration = consts.DefaultMaxIdleConnDuration
	}
	for {
		currentTime := time.Now()

		// Determine idle connections to be closed.
		c.connsLock.Lock()
		conns := c.conns
		n := len(conns)
		i := 0

		for i < n && currentTime.Sub(conns[i].lastUseTime) > maxIdleConnDuration {
			i++
		}
		sleepFor := maxIdleConnDuration
		if i < n {
			// + 1 so we actually sleep past the expiration time and not up to it.
			// Otherwise the > check above would still fail.
			sleepFor = maxIdleConnDuration - currentTime.Sub(conns[i].lastUseTime) + 1
		}
		scratch = append(scratch[:0], conns[:i]...)
		if i > 0 {
			m := copy(conns, conns[i:])
			for i = m; i < n; i++ {
				conns[i] = nil
			}
			c.conns = conns[:m]
		}
		c.connsLock.Unlock()

		// Close idle connections.
		for i, cc := range scratch {
			c.closeConn(cc)
			scratch[i] = nil
		}

		// Determine whether to stop the connsCleaner.
		c.connsLock.Lock()
		mustStop := c.connsCount == 0
		if mustStop {
			c.connsCleanerRun = false
		}
		c.connsLock.Unlock()
		if mustStop {
			break
		}

		time.Sleep(sleepFor)
	}
}

func (c *HostClient) closeConn(cc *clientConn) {
	c.decConnsCount()
	cc.c.Close()
	releaseClientConn(cc)
}

func (c *HostClient) decConnsCount() {
	if c.MaxConnWaitTimeout <= 0 {
		c.connsLock.Lock()
		c.connsCount--
		c.connsLock.Unlock()
		return
	}

	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	dialed := false
	if q := c.connsWait; q != nil && q.len() > 0 {
		for q.len() > 0 {
			w := q.popFront()
			if w.waiting() {
				go c.dialConnFor(w)
				dialed = true
				break
			}
		}
	}
	if !dialed {
		c.connsCount--
	}
}

func acquireClientConn(conn network.Conn) *clientConn {
	v := clientConnPool.Get()
	if v == nil {
		v = &clientConn{}
	}
	cc := v.(*clientConn)
	cc.c = conn
	cc.createdTime = time.Now()
	return cc
}

func releaseClientConn(cc *clientConn) {
	// Reset all fields.
	*cc = clientConn{}
	clientConnPool.Put(cc)
}

var clientConnPool sync.Pool

func (c *HostClient) releaseConn(cc *clientConn) {
	cc.lastUseTime = time.Now()
	if c.MaxConnWaitTimeout <= 0 {
		c.connsLock.Lock()
		c.conns = append(c.conns, cc)
		c.connsLock.Unlock()
		return
	}

	// try to deliver an idle connection to a *wantConn
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	delivered := false
	if q := c.connsWait; q != nil && q.len() > 0 {
		for q.len() > 0 {
			w := q.popFront()
			if w.waiting() {
				delivered = w.tryDeliver(cc, nil)
				break
			}
		}
	}
	if !delivered {
		c.conns = append(c.conns, cc)
	}
}

func (c *HostClient) acquireWriter(conn network.Conn) network.Writer {
	return conn
}

func (c *HostClient) acquireReader(conn network.Conn) network.Reader {
	return conn
}

func newClientTLSConfig(c *tls.Config, addr string) *tls.Config {
	if c == nil {
		c = &tls.Config{}
	} else {
		c = c.Clone()
	}

	if c.ClientSessionCache == nil {
		c.ClientSessionCache = tls.NewLRUClientSessionCache(0)
	}

	if len(c.ServerName) == 0 {
		serverName := tlsServerName(addr)
		if serverName == "*" {
			c.InsecureSkipVerify = true
		} else {
			c.ServerName = serverName
		}
	}
	return c
}

func tlsServerName(addr string) string {
	if !strings.Contains(addr, ":") {
		return addr
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "*"
	}
	return host
}

func (c *HostClient) nextAddr() string {
	c.addrsLock.Lock()
	if c.addrs == nil {
		c.addrs = strings.Split(c.Addr, ",")
	}
	addr := c.addrs[0]
	if len(c.addrs) > 1 {
		addr = c.addrs[c.addrIdx%uint32(len(c.addrs))]
		c.addrIdx++
	}
	c.addrsLock.Unlock()
	return addr
}

func (c *HostClient) dialHostHard(dialTimeout time.Duration) (conn network.Conn, err error) {
	// attempt to dial all the available hosts before giving up.

	c.addrsLock.Lock()
	n := len(c.addrs)
	c.addrsLock.Unlock()

	if n == 0 {
		// It looks like c.addrs isn't initialized yet.
		n = 1
	}

	deadline := time.Now().Add(dialTimeout)
	for n > 0 {
		addr := c.nextAddr()
		tlsConfig := c.cachedTLSConfig(addr)
		conn, err = dialAddr(addr, c.Dialer, c.DialDualStack, tlsConfig, dialTimeout, c.ProxyURI, c.IsTLS)
		if err == nil {
			return conn, nil
		}
		if time.Since(deadline) >= 0 {
			break
		}
		n--
	}
	return nil, err
}

func (c *HostClient) cachedTLSConfig(addr string) *tls.Config {
	var cfgAddr string
	if c.ProxyURI != nil && bytes.Equal(c.ProxyURI.Scheme(), bytestr.StrHTTPS) {
		cfgAddr = bytesconv.B2s(c.ProxyURI.Host())
	}

	if c.IsTLS && cfgAddr == "" {
		cfgAddr = addr
	}

	if cfgAddr == "" {
		return nil
	}

	c.tlsConfigMapLock.Lock()
	if c.tlsConfigMap == nil {
		c.tlsConfigMap = make(map[string]*tls.Config)
	}
	cfg := c.tlsConfigMap[cfgAddr]
	if cfg == nil {
		cfg = newClientTLSConfig(c.TLSConfig, cfgAddr)
		c.tlsConfigMap[cfgAddr] = cfg
	}
	c.tlsConfigMapLock.Unlock()

	return cfg
}

func dialAddr(addr string, dial network.Dialer, dialDualStack bool, tlsConfig *tls.Config, timeout time.Duration, proxyURI *protocol.URI, isTLS bool) (network.Conn, error) {
	var conn network.Conn
	var err error
	if dial == nil {
		hlog.SystemLogger().Warn("HostClient: no dialer specified, trying to use default dialer")
		dial = dialer.DefaultDialer()
	}
	dialFunc := dial.DialConnection

	// addr has already been added port, no need to do it here
	if proxyURI != nil {
		// use tcp connection first, proxy will AddTLS to it
		conn, err = dialFunc("tcp", string(proxyURI.Host()), timeout, nil)
	} else {
		conn, err = dialFunc("tcp", addr, timeout, tlsConfig)
	}

	if err != nil {
		return nil, err
	}
	if conn == nil {
		panic("BUG: dial.DialConnection returned (nil, nil)")
	}

	if proxyURI != nil {
		conn, err = proxy.SetupProxy(conn, addr, proxyURI, tlsConfig, isTLS, dial)
	}

	// conn must be nil when got error, so doesn't need to close it
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *HostClient) getClientName() []byte {
	v := c.clientName.Load()
	var clientName []byte
	if v == nil {
		clientName = []byte(c.Name)
		if len(clientName) == 0 && !c.NoDefaultUserAgentHeader {
			clientName = bytestr.DefaultUserAgent
		}
		c.clientName.Store(clientName)
	} else {
		clientName = v.([]byte)
	}
	return clientName
}

// waiting reports whether w is still waiting for an answer (connection or error).
func (w *wantConn) waiting() bool {
	select {
	case <-w.ready:
		return false
	default:
		return true
	}
}

// tryDeliver attempts to deliver conn, err to w and reports whether it succeeded.
func (w *wantConn) tryDeliver(conn *clientConn, err error) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil || w.err != nil {
		return false
	}
	w.conn = conn
	w.err = err
	if w.conn == nil && w.err == nil {
		panic("hertz: internal error: misuse of tryDeliver")
	}
	close(w.ready)
	return true
}

// cancel marks w as no longer wanting a result (for example, due to cancellation).
// If a connection has been delivered already, cancel returns it with c.releaseConn.
func (w *wantConn) cancel(c *HostClient, err error) {
	w.mu.Lock()
	if w.conn == nil && w.err == nil {
		close(w.ready) // catch misbehavior in future delivery
	}

	conn := w.conn
	w.conn = nil
	w.err = err
	w.mu.Unlock()

	if conn != nil {
		c.releaseConn(conn)
	}
}

// len returns the number of items in the queue.
func (q *wantConnQueue) len() int {
	return len(q.head) - q.headPos + len(q.tail)
}

// pushBack adds w to the back of the queue.
func (q *wantConnQueue) pushBack(w *wantConn) {
	q.tail = append(q.tail, w)
}

// popFront removes and returns the wantConn at the front of the queue.
func (q *wantConnQueue) popFront() *wantConn {
	if q.headPos >= len(q.head) {
		if len(q.tail) == 0 {
			return nil
		}
		// Pick up tail as new head, clear tail.
		q.head, q.headPos, q.tail = q.tail, 0, q.head[:0]
	}

	w := q.head[q.headPos]
	q.head[q.headPos] = nil
	q.headPos++
	return w
}

// peekFront returns the wantConn at the front of the queue without removing it.
func (q *wantConnQueue) peekFront() *wantConn {
	if q.headPos < len(q.head) {
		return q.head[q.headPos]
	}
	if len(q.tail) > 0 {
		return q.tail[0]
	}
	return nil
}

// cleanFront pops any wantConns that are no longer waiting from the head of the
// queue, reporting whether any were popped.
func (q *wantConnQueue) clearFront() (cleaned bool) {
	for {
		w := q.peekFront()
		if w == nil || w.waiting() {
			return cleaned
		}
		q.popFront()
		cleaned = true
	}
}

func NewHostClient(c *ClientOptions) client.HostClient {
	hc := &HostClient{
		ClientOptions: c,
		closed:        make(chan struct{}),
	}

	return hc
}

type ClientOptions struct {
	// Client name. Used in User-Agent request header.
	Name string

	// NoDefaultUserAgentHeader when set to true, causes the default
	// User-Agent header to be excluded from the Request.
	NoDefaultUserAgentHeader bool

	// Callback for establishing new connection to the host.
	//
	// Default Dialer is used if not set.
	Dialer network.Dialer

	// Timeout for establishing new connections to hosts.
	//
	// Default DialTimeout is used if not set.
	DialTimeout time.Duration

	// Attempt to connect to both ipv4 and ipv6 host addresses
	// if set to true.
	//
	// This option is used only if default TCP dialer is used,
	// i.e. if Dialer is blank.
	//
	// By default client connects only to ipv4 addresses,
	// since unfortunately ipv6 remains broken in many networks worldwide :)
	DialDualStack bool

	// Whether to use TLS (aka SSL or HTTPS) for host connections.
	// Optional TLS config.
	TLSConfig *tls.Config

	// Maximum number of connections which may be established to all hosts
	// listed in Addr.
	//
	// You can change this value while the HostClient is being used
	// using HostClient.SetMaxConns(value)
	//
	// DefaultMaxConnsPerHost is used if not set.
	MaxConns int

	// Keep-alive connections are closed after this duration.
	//
	// By default connection duration is unlimited.
	MaxConnDuration time.Duration

	// Idle keep-alive connections are closed after this duration.
	//
	// By default idle connections are closed
	// after DefaultMaxIdleConnDuration.
	MaxIdleConnDuration time.Duration

	// Maximum duration for full response reading (including body).
	//
	// By default response read timeout is unlimited.
	ReadTimeout time.Duration

	// Maximum duration for full request writing (including body).
	//
	// By default request write timeout is unlimited.
	WriteTimeout time.Duration

	// Maximum response body size.
	//
	// The client returns errBodyTooLarge if this limit is greater than 0
	// and response body is greater than the limit.
	//
	// By default response body size is unlimited.
	MaxResponseBodySize int

	// Header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// responses to other clients expecting case-sensitive header names.
	//
	// By default request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool

	// Path values are sent as-is without normalization
	//
	// Disabled path normalization may be useful for proxying incoming requests
	// to servers that are expecting paths to be forwarded as-is.
	//
	// By default path values are normalized, i.e.
	// extra slashes are removed, special characters are encoded.
	DisablePathNormalizing bool

	// Maximum duration for waiting for a free connection.
	//
	// By default will not wait, return ErrNoFreeConns immediately
	MaxConnWaitTimeout time.Duration

	// ResponseBodyStream enables response body streaming
	ResponseBodyStream bool

	// All configurations related to retry
	RetryConfig *retry.Config

	RetryIfFunc client.RetryIfFunc

	// Observe hostclient state
	StateObserve config.HostClientStateFunc

	// StateObserve execution interval
	ObservationInterval time.Duration
}
