/*
 *	Copyright 2022 CloudWeGo Authors
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
 * Copyright 2017 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

package http2

import (
	"bufio"
	"bytes"
	"container/list"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	mathrand "math/rand"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/internal/nocopy"
	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/compress"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/dialer"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/client"
	"github.com/cloudwego/hertz/pkg/protocol/http1/ext"
	"github.com/cloudwego/hertz/pkg/protocol/http2/hpack"
	"golang.org/x/net/http/httpguts"
)

const (
	// transportDefaultConnFlow is how many connection-level flow control
	// tokens we give the server at start-up, past the default 64k.
	transportDefaultConnFlow = 1 << 30

	// transportDefaultStreamFlow is how many stream-level flow
	// control tokens we announce to the peer, and how many bytes
	// we buffer per stream.
	transportDefaultStreamFlow = 4 << 20

	// transportDefaultStreamMinRefresh is the minimum number of bytes we'll send
	// a stream-level WINDOW_UPDATE for at a time.
	transportDefaultStreamMinRefresh = 4 << 10

	// initialMaxConcurrentStreams is a connections maxConcurrentStreams until
	// it's received servers initial SETTINGS frame, which corresponds with the
	// spec's minimum recommended value.
	initialMaxConcurrentStreams = 100

	// defaultMaxConcurrentStreams is a connections default maxConcurrentStreams
	// if the server doesn't include one in its initial SETTINGS frame.
	defaultMaxConcurrentStreams = 1000
)

var noBody io.ReadCloser = ioutil.NopCloser(bytes.NewReader(nil))

type missingBody struct{}

func (missingBody) Close() error             { return nil }
func (missingBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

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

	*ClientOption

	lck   sync.Mutex
	conns list.List

	addrsLock sync.Mutex
	addrs     []string
	addrIdx   uint32

	tlsConfigMap     map[string]*tls.Config
	tlsConfigMapLock sync.Mutex
}

func (hc *HostClient) SetDynamicConfig(dc *client.DynamicConfig) {
	hc.Addr = dc.Addr
	hc.IsTLS = dc.IsTLS
}

// Get returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
func (hc *HostClient) Get(ctx context.Context, dst []byte, url string) (statusCode int, body []byte, err error) {
	return client.GetURL(ctx, dst, url, hc)
}

// shouldRetryRequest is called by RoundTrip when a request fails to get
// response headers. It is always called with a non-nil error.
// It returns either a request to retry (either the same request, or a
// modified clone), or an error if the request can't be replayed.
func shouldRetryRequest(req *protocol.Request, err error) (*protocol.Request, error) {
	if !canRetryError(err) {
		return nil, err
	}
	// If the Body is nil (or http.NoBody), it's safe to reuse
	// this request and its Body.
	if req.BodyStream() == ext.NoBody && len(req.Body()) == 0 {
		return req, nil
	}

	// The Request.Body can't reset back to the beginning, but we
	// don't seem to have started to read from it yet, so reuse
	// the request directly.
	if err == errClientConnUnusable {
		return req, nil
	}

	return nil, fmt.Errorf("http2: Transport: cannot retry err [%v] after Request.Body was written; define Request.GetBody to avoid this error", err)
}

func canRetryError(err error) bool {
	if err == errClientConnUnusable || err == errClientConnGotGoAway {
		return true
	}
	if se, ok := err.(StreamError); ok {
		if se.Code == ErrCodeProtocol && se.Cause == errFromPeer {
			// See golang/go#47635, golang/go#42777
			return true
		}
		return se.Code == ErrCodeRefusedStream
	}
	return false
}

func (hc *HostClient) Do(ctx context.Context, req *protocol.Request, rsp *protocol.Response) error {
	if !bytes.Equal(req.URI().Scheme(), bytestr.StrHTTPS) && !hc.AllowHTTP {
		return errors.New("http2: unsupported scheme")
	}
	for retry := 0; ; retry++ {
		cc, err := hc.acquireConn()
		if err != nil {
			hlog.Errorf("http2: transport failed to get client conn for %s: %v", hc.Addr, err)
			return err
		}
		err = cc.RoundTrip(ctx, req, rsp)
		if err != nil && retry <= hc.MaxIdemponentCallAttempts {
			if req, err = shouldRetryRequest(req, err); err == nil {
				// After the first retry, do exponential backoff with 10% jitter.
				if retry == 0 {
					continue
				}
				backoff := float64(uint(1) << (uint(retry) - 1))
				backoff += backoff * (0.1 * mathrand.Float64())
				select {
				case <-time.After(time.Second * time.Duration(backoff)):
					continue
				case <-ctx.Done():
					err = ctx.Err()
				}
			}
		}
		if err != nil {
			hlog.Errorf("RoundTrip failure: %v", err)
			return err
		}
		return nil
	}
}

func (cc *clientConn) RoundTrip(ctx context.Context, req *protocol.Request, rsp *protocol.Response) error {
	cs := &clientStream{
		cc:                   cc,
		ctx:                  ctx,
		isHead:               bytes.Equal(req.Method(), bytestr.StrHead),
		reqBodyContentLength: actualContentLength(req),
		reqBody:              req.BodyStream(),
		peerClosed:           make(chan struct{}),
		abort:                make(chan struct{}),
		respHeaderRecv:       make(chan struct{}),
		donec:                make(chan struct{}),
		res:                  rsp,
	}

	if cs.reqBody == ext.NoBody && len(req.Body()) > 0 {
		cs.reqBody = bytes.NewReader(req.Body())
	}

	go cs.doRequest(req)

	waitDone := func() error {
		select {
		case <-cs.donec:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	handleResponseHeaders := func() error {
		res := cs.res
		if res.StatusCode() > 299 {
			// On error or status code 3xx, 4xx, 5xx, etc abort any
			// ongoing write, assuming that the server doesn't care
			// about our request body. If the server replied with 1xx or
			// 2xx, however, then assume the server DOES potentially
			// want our body (e.g. full-duplex streaming:
			// golang.org/issue/13444). If it turns out the server
			// doesn't, they'll RST_STREAM us soon enough. This is a
			// heuristic to avoid adding knobs to Transport. Hopefully
			// we can keep it.
			cs.abortRequestBodyWrite()
		}

		if res.BodyStream() == noBody && actualContentLength(req) == 0 {
			// If there isn't a request or response body still being
			// written, then wait for the stream to be closed before
			// RoundTrip returns.
			if err := waitDone(); err != nil {
				return err
			}
		}
		return nil
	}

	for {
		select {
		case <-cs.respHeaderRecv:
			return handleResponseHeaders()
		case <-cs.abort:
			select {
			case <-cs.respHeaderRecv:
				// If both cs.respHeaderRecv and cs.abort are signaling,
				// pick respHeaderRecv. The server probably wrote the
				// response and immediately reset the stream.
				// golang.org/issue/49645
				return handleResponseHeaders()
			default:
				waitDone()
				return cs.abortErr
			}
		case <-ctx.Done():
			err := ctx.Err()
			cs.abortStream(err)
			return err
		}
	}
}

func (hc *HostClient) onConnectionDropped(c *clientConn) {
	hc.lck.Lock()
	defer hc.lck.Unlock()
	for e := hc.conns.Front(); e != nil; e = e.Next() {
		if e.Value.(*clientConn) == c {
			hc.conns.Remove(e)
			break
		}
	}
}

func (hc *HostClient) createConn() (*clientConn, *list.Element, error) {
	conn, err := hc.dialHostHard()
	if err != nil {
		return nil, nil, err
	}
	c, err := hc.newClientConn(conn, false)
	if err != nil {
		return nil, nil, err
	}
	return c, hc.conns.PushFront(c), nil
}

func (hc *HostClient) acquireConn() (*clientConn, error) {
	var c *clientConn
	var err error

	hc.lck.Lock()
	defer hc.lck.Unlock()

	var next *list.Element
	for e := hc.conns.Front(); c == nil; e = next {
		if e != nil {
			c = e.Value.(*clientConn)
		} else {
			c, e, err = hc.createConn()
			if err != nil {
				return nil, err
			}
		}

		if !c.ReserveNewRequest() {
			c = nil
			next = e.Next()
		}

		if c != nil && c.closed {
			next = e.Next()
			hc.conns.Remove(e)
			c = nil
		}
	}

	return c, nil
}

func (hc *HostClient) dialHostHard() (conn net.Conn, err error) {
	hc.addrsLock.Lock()
	n := len(hc.addrs)
	hc.addrsLock.Unlock()

	if n == 0 {
		// It looks like c.addrs isn't initialized yet.
		n = 1
	}

	timeout := hc.DialTimeout
	deadline := time.Now().Add(timeout)
	for n > 0 {
		addr := hc.nextAddr()
		tlsConfig := hc.cachedTLSConfig(addr)
		conn, err = dialAddr(addr, hc.Dialer, tlsConfig, timeout, hc.IsTLS)
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

func dialAddr(addr string, dial network.Dialer, tlsConfig *tls.Config, timeout time.Duration, isTLS bool) (net.Conn, error) {
	var conn net.Conn
	var err error
	if dial == nil {
		hlog.Warnf("HERTZ: HostClient: no dialer specified, trying to use default dialer")
		dial = dialer.DefaultDialer()
	}
	conn, err = dial.DialConnection("tcp", addr, timeout, tlsConfig)
	if err != nil {
		return nil, err
	}

	if conn == nil {
		panic("BUG: DialFunc returned (nil, nil)")
	}

	if dial == nil && isTLS {
		p, ok := reflect.ValueOf(conn).Interface().(network.ConnTLSer)
		if !ok {
			return nil, fmt.Errorf("http2: unexpected connection %p; want TLSConn", p)
		}
		err = p.Handshake()
		if err != nil {
			return nil, err
		}
		if p.ConnectionState().NegotiatedProtocol != NextProtoTLS {
			return nil, fmt.Errorf("http2: unexpected ALPN protocol %p; want %q", p, NextProtoTLS)
		}
	}

	return conn, nil
}

func (hc *HostClient) cachedTLSConfig(addr string) *tls.Config {
	if !hc.IsTLS {
		return nil
	}

	cfgAddr := addr

	hc.tlsConfigMapLock.Lock()
	if hc.tlsConfigMap == nil {
		hc.tlsConfigMap = make(map[string]*tls.Config)
	}
	cfg := hc.tlsConfigMap[cfgAddr]
	if cfg == nil {
		cfg = newClientTLSConfig(hc.TLSConfig, cfgAddr)
		hc.tlsConfigMap[cfgAddr] = cfg
	}
	hc.tlsConfigMapLock.Unlock()

	return cfg
}

func (hc *HostClient) CloseIdleConnections() {
	hc.lck.Lock()
	defer hc.lck.Unlock()
	for e := hc.conns.Front(); e != nil; e = e.Next() {
		c := e.Value.(*clientConn)
		c.closeIfIdle()
	}
}

func (hc *HostClient) ShouldRemove() bool {
	return hc.ConnectionCount() == 0
}

func (hc *HostClient) ConnectionCount() int {
	hc.lck.Lock()
	defer hc.lck.Unlock()
	return hc.conns.Len()
}

func strSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func newClientTLSConfig(c *tls.Config, addr string) *tls.Config {
	if c == nil {
		c = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	} else {
		c = c.Clone()
	}

	if len(c.ServerName) == 0 {
		serverName := tlsServerName(addr)
		if serverName == "*" {
			c.InsecureSkipVerify = true
		} else {
			c.ServerName = serverName
		}
	}

	if !strSliceContains(c.NextProtos, NextProtoTLS) {
		c.NextProtos = append([]string{NextProtoTLS}, c.NextProtos...)
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

func (hc *HostClient) nextAddr() string {
	hc.addrsLock.Lock()
	if hc.addrs == nil {
		hc.addrs = strings.Split(hc.Addr, ",")
	}
	addr := hc.addrs[0]
	if len(hc.addrs) > 1 {
		addr = hc.addrs[hc.addrIdx%uint32(len(hc.addrs))]
		hc.addrIdx++
	}
	hc.addrsLock.Unlock()
	return addr
}

func (hc *HostClient) newClientConn(c net.Conn, singleUse bool) (*clientConn, error) {
	cc := &clientConn{}
	cc.tconn = &h2Conn{c}
	cc.createdTime = time.Now()
	cc.readerDone = make(chan struct{})
	cc.nextStreamID = 1
	cc.maxFrameSize = 16 << 10                            // spec default
	cc.initialWindowSize = 65535                          // spec default
	cc.maxConcurrentStreams = initialMaxConcurrentStreams // "infinite", per spec. Use a smaller value until we have received server settings.
	cc.peerMaxHeaderListSize = 0xffffffffffffffff         // "infinite", per spec. Use 2^64-1 instead.
	cc.streams = make(map[uint32]*clientStream)
	cc.singleUse = singleUse
	cc.wantSettingsAck = true
	cc.pings = make(map[[8]byte]chan struct{})
	cc.reqHeaderMu = make(chan struct{}, 1)
	cc.hc = hc
	if d := hc.MaxIdleConnDuration; d != 0 {
		cc.idleTimeout = d
		cc.idleTimer = time.AfterFunc(d, cc.onIdleTimeout)
	}

	if VerboseLogs {
		hlog.Infof("http2: host client creating client conn %p to %v", cc, c.RemoteAddr())
	}

	cc.cond = sync.NewCond(&cc.mu)
	cc.flow.add(int32(initialWindowSize))

	// TODO: adjust this writer size to account for frame size +
	// MTU + crypto/tls record padding.
	cc.bw = bufio.NewWriter(stickyErrWriter{
		conn:    c,
		timeout: hc.WriteByteTimeout,
		err:     &cc.werr,
	})
	cc.br = bufio.NewReader(c)
	cc.fr = NewFramer(cc.bw, c)
	cc.fr.ReadMetaHeaders = hpack.NewDecoder(initialHeaderTableSize, nil)
	cc.fr.MaxHeaderListSize = hc.MaxHeaderListSize

	// TODO: SetMaxDynamicTableSize, SetMaxDynamicTableSizeLimit on
	// henc in response to SETTINGS frames?
	cc.henc = hpack.NewEncoder(&cc.hbuf)

	if hc.AllowHTTP {
		cc.nextStreamID = 3
	}

	initialSettings := []Setting{
		{ID: SettingEnablePush, Val: 0},
		{ID: SettingInitialWindowSize, Val: transportDefaultStreamFlow},
	}
	if max := hc.MaxHeaderListSize; max != 0 {
		initialSettings = append(initialSettings, Setting{ID: SettingMaxHeaderListSize, Val: max})
	}

	cc.bw.Write(clientPreface)
	cc.fr.WriteSettings(initialSettings...)
	cc.fr.WriteWindowUpdate(0, transportDefaultConnFlow)
	cc.inflow.add(transportDefaultConnFlow + initialWindowSize)
	cc.bw.Flush()
	if cc.werr != nil {
		cc.Close()
		return nil, cc.werr
	}

	go cc.readLoop()
	return cc, nil
}

// clientConn is the state of a single HTTP/2 client connection to an
// HTTP/2 server.
type clientConn struct {
	tconn net.Conn // usually *tls.Conn, except specialized impls
	hc    *HostClient

	// readLoop goroutine fields:
	readerDone chan struct{} // closed on error
	readerErr  error         // set before readerDone is closed

	idleTimeout time.Duration // or 0 for never
	idleTimer   *time.Timer

	mu              sync.Mutex // guards following
	cond            *sync.Cond // hold mu; broadcast on flow/closed changes
	flow            flow       // our conn-level flow control quota (cs.flow is per stream)
	inflow          flow       // peer's conn-level flow control
	doNotReuse      bool       // whether conn is marked to not be reused for any future requests
	closing         bool
	closed          bool
	seenSettings    bool                     // true if we've seen a settings frame, false otherwise
	wantSettingsAck bool                     // we sent a SETTINGS frame and haven't heard back
	goAway          *GoAwayFrame             // if non-nil, the GoAwayFrame we received
	goAwayDebug     string                   // goAway frame's debug data, retained as a string
	streams         map[uint32]*clientStream // client-initiated
	streamsReserved int                      // incr by ReserveNewRequest; decr on RoundTrip
	nextStreamID    uint32
	pendingRequests int                       // requests blocked and waiting to be sent because len(streams) == maxConcurrentStreams
	pings           map[[8]byte]chan struct{} // in flight ping data to notification channel
	br              *bufio.Reader
	lastActive      time.Time
	lastIdle        time.Time // time last idle
	createdTime     time.Time
	// Settings from peer: (also guarded by wmu)
	maxFrameSize          uint32
	maxConcurrentStreams  uint32
	peerMaxHeaderListSize uint64
	initialWindowSize     uint32

	// reqHeaderMu is a 1-element semaphore channel controlling access to sending new requests.
	// Write to reqHeaderMu to lock it, read from it to unlock.
	// Lock reqmu BEFORE mu or wmu.
	reqHeaderMu chan struct{}

	// wmu is held while writing.
	// Acquire BEFORE mu when holding both, to avoid blocking mu on network writes.
	// Only acquire both at the same time when changing peer settings.
	wmu  sync.Mutex
	bw   *bufio.Writer
	fr   *Framer
	werr error        // first write error that has occurred
	hbuf bytes.Buffer // HPACK encoder writes into this
	henc *hpack.Encoder

	singleUse bool

	// PingTimeout is the timeout after which the connection will be closed
	// if a response to Ping is not received.
	// Defaults to 15s.
	PingTimeout time.Duration

	// WriteByteTimeout is the timeout after which the connection will be
	// closed no data can be written to it. The timeout begins when data is
	// available to write, and is extended whenever any bytes are written.
	WriteByteTimeout time.Duration

	// ReadIdleTimeout is the timeout after which a health check using ping
	// frame will be carried out if no frame is received on the connection.
	// Note that a ping response will is considered a received frame, so if
	// there is no other traffic on the connection, the health check will
	// be performed every ReadIdleTimeout interval.
	// If zero, no health check is performed.
	ReadIdleTimeout time.Duration
}

// clientStream is the state for a single HTTP/2 stream. One of these
// is created for each Transport.RoundTrip call.
type clientStream struct {
	cc *clientConn

	// Fields of Request that we may access even after the response body is closed.
	ctx context.Context

	ID            uint32
	bufPipe       pipe // buffered pipe with the flow-controlled response payload
	requestedGzip bool
	isHead        bool

	abortOnce sync.Once
	abort     chan struct{} // closed to signal stream should end immediately
	abortErr  error         // set if abort is closed

	peerClosed chan struct{} // closed when the peer sends an END_STREAM flag
	donec      chan struct{} // closed after the stream is in the closed state
	on100      chan struct{} // buffered; written to if a 100 is received

	respHeaderRecv chan struct{}      // closed when headers are received
	res            *protocol.Response // set if respHeaderRecv is closed

	flow        flow  // guarded by cc.mu
	inflow      flow  // guarded by cc.mu
	bytesRemain int64 // -1 means unknown; owned by transportResponseBody.Read
	readErr     error // sticky read error; owned by transportResponseBody.Read

	reqBody              io.Reader
	reqBodyContentLength int64 // -1 means unknown
	reqBodyClosed        bool  // body has been closed; guarded by cc.mu

	// owned by writeRequest:
	sentEndStream bool // sent an END_STREAM flag to the peer
	sentHeaders   bool

	// owned by clientConnReadLoop:
	firstByte   bool  // got the first response byte
	num1xx      uint8 // number of 1xx responses seen
	readClosed  bool  // peer sent an END_STREAM flag
	readAborted bool  // read loop reset the stream
}

var got1xxFuncForTests func(int, *protocol.ResponseHeader) error

// get1xxTraceFunc returns the value of request's httptrace.ClientTrace.Got1xxResponse func,
// if any. It returns nil if not set or if the Go version is too old.
func (cs *clientStream) get1xxTraceFunc() func(int, *protocol.ResponseHeader) error {
	if fn := got1xxFuncForTests; fn != nil {
		return fn
	}
	return nil
}

func (cs *clientStream) abortStream(err error) {
	cs.cc.mu.Lock()
	defer cs.cc.mu.Unlock()
	cs.abortStreamLocked(err)
}

func (cs *clientStream) abortStreamLocked(err error) {
	cs.abortOnce.Do(func() {
		cs.abortErr = err
		close(cs.abort)
	})
	if cs.reqBody != nil && !cs.reqBodyClosed {
		if bsc, ok := cs.reqBody.(io.Closer); ok {
			bsc.Close()
		}
		cs.reqBodyClosed = true
	}
	// TODO: Clean up tests where cs.cc.cond is nil.
	if cs.cc.cond != nil {
		// Wake up writeRequestBody if it is waiting on flow control.
		cs.cc.cond.Broadcast()
	}
}

func (cs *clientStream) abortRequestBodyWrite() {
	cc := cs.cc
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cs.reqBody != nil && !cs.reqBodyClosed {
		if bsc, ok := cs.reqBody.(io.Closer); ok {
			bsc.Close()
		}
		cs.reqBodyClosed = true
		cc.cond.Broadcast()
	}
}

type stickyErrWriter struct {
	conn    net.Conn
	timeout time.Duration
	err     *error
}

func (sew stickyErrWriter) Write(p []byte) (n int, err error) {
	if *sew.err != nil {
		return 0, *sew.err
	}
	for {
		if sew.timeout != 0 {
			sew.conn.SetWriteDeadline(time.Now().Add(sew.timeout))
		}
		nn, err := sew.conn.Write(p[n:])
		n += nn
		if n < len(p) && nn > 0 && errors.Is(err, os.ErrDeadlineExceeded) {
			// Keep extending the deadline so long as we're making progress.
			continue
		}
		if sew.timeout != 0 {
			sew.conn.SetWriteDeadline(time.Time{})
		}
		*sew.err = err
		return n, err
	}
}

func (cc *clientConn) healthCheck() {
	pingTimeout := cc.hc.PingTimeout
	// We don't need to periodically ping in the health check, because the readLoop of ClientConn will
	// trigger the healthCheck again if there is no frame received.
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	err := cc.Ping(ctx)
	if err != nil {
		cc.closeForLostPing()
		cc.hc.onConnectionDropped(cc)
		return
	}
}

// SetDoNotReuse marks cc as not reusable for future HTTP requests.
func (cc *clientConn) SetDoNotReuse() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.doNotReuse = true
}

func (cc *clientConn) setGoAway(f *GoAwayFrame) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	old := cc.goAway
	cc.goAway = f

	// Merge the previous and current GoAway error frames.
	if cc.goAwayDebug == "" {
		cc.goAwayDebug = string(f.DebugData())
	}
	if old != nil && old.ErrCode != ErrCodeNo {
		cc.goAway.ErrCode = old.ErrCode
	}
	last := f.LastStreamID
	for streamID, cs := range cc.streams {
		if streamID > last {
			cs.abortStreamLocked(errClientConnGotGoAway)
		}
	}
}

// CanTakeNewRequest reports whether the connection can take a new request,
// meaning it has not been closed or received or sent a GOAWAY.
//
// If the caller is going to immediately make a new request on this
// connection, use ReserveNewRequest instead.
func (cc *clientConn) CanTakeNewRequest() bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.canTakeNewRequestLocked()
}

// ReserveNewRequest is like CanTakeNewRequest but also reserves a
// concurrent stream in cc. The reservation is decremented on the
// next call to RoundTrip.
func (cc *clientConn) ReserveNewRequest() bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if st := cc.idleStateLocked(); !st.canTakeNewRequest {
		return false
	}
	cc.streamsReserved++
	return true
}

// ClientConnState describes the state of a ClientConn.
type ClientConnState struct {
	// Closed is whether the connection is closed.
	Closed bool

	// Closing is whether the connection is in the process of
	// closing. It may be closing due to shutdown, being a
	// single-use connection, being marked as DoNotReuse, or
	// having received a GOAWAY frame.
	Closing bool

	// StreamsActive is how many streams are active.
	StreamsActive int

	// StreamsReserved is how many streams have been reserved via
	// ClientConn.ReserveNewRequest.
	StreamsReserved int

	// StreamsPending is how many requests have been sent in excess
	// of the peer's advertised MaxConcurrentStreams setting and
	// are waiting for other streams to complete.
	StreamsPending int

	// MaxConcurrentStreams is how many concurrent streams the
	// peer advertised as acceptable. Zero means no SETTINGS
	// frame has been received yet.
	MaxConcurrentStreams uint32

	// LastIdle, if non-zero, is when the connection last
	// transitioned to idle state.
	LastIdle time.Time
}

// State returns a snapshot of cc's state.
func (cc *clientConn) State() ClientConnState {
	cc.wmu.Lock()
	maxConcurrent := cc.maxConcurrentStreams
	if !cc.seenSettings {
		maxConcurrent = 0
	}
	cc.wmu.Unlock()

	cc.mu.Lock()
	defer cc.mu.Unlock()
	return ClientConnState{
		Closed:               cc.closed,
		Closing:              cc.closing || cc.singleUse || cc.doNotReuse || cc.goAway != nil,
		StreamsActive:        len(cc.streams),
		StreamsReserved:      cc.streamsReserved,
		StreamsPending:       cc.pendingRequests,
		LastIdle:             cc.lastIdle,
		MaxConcurrentStreams: maxConcurrent,
	}
}

// clientConnIdleState describes the suitability of a client
// connection to initiate a new RoundTrip request.
type clientConnIdleState struct {
	canTakeNewRequest bool
}

func (cc *clientConn) idleStateLocked() (st clientConnIdleState) {
	if cc.singleUse && cc.nextStreamID > 1 {
		return
	}
	var maxConcurrentOkay bool
	if cc.hc.StrictMaxConcurrentStreams {
		// We'll tell the caller we can take a new request to
		// prevent the caller from dialing a new TCP
		// connection, but then we'll block later before
		// writing it.
		maxConcurrentOkay = true
	} else {
		maxConcurrentOkay = int64(len(cc.streams)+cc.streamsReserved+1) <= int64(cc.maxConcurrentStreams)
	}

	st.canTakeNewRequest = cc.goAway == nil && !cc.closed && !cc.closing && maxConcurrentOkay &&
		!cc.doNotReuse &&
		int64(cc.nextStreamID)+2*int64(cc.pendingRequests) < math.MaxInt32 &&
		!cc.tooIdleLocked()
	return
}

func (cc *clientConn) canTakeNewRequestLocked() bool {
	st := cc.idleStateLocked()
	return st.canTakeNewRequest
}

// tooIdleLocked reports whether this connection has been been sitting idle
// for too much wall time.
func (cc *clientConn) tooIdleLocked() bool {
	// The Round(0) strips the monontonic clock reading so the
	// times are compared based on their wall time. We don't want
	// to reuse a connection that's been sitting idle during
	// VM/laptop suspend if monotonic time was also frozen.
	return cc.idleTimeout != 0 && !cc.lastIdle.IsZero() && time.Since(cc.lastIdle.Round(0)) > cc.idleTimeout
}

// onIdleTimeout is called from a time.AfterFunc goroutine. It will
// only be called when we're idle, but because we're coming from a new
// goroutine, there could be a new request coming in at the same time,
// so this simply calls the synchronized closeIfIdle to shut down this
// connection. The timer could just call closeIfIdle, but this is more
// clear.
func (cc *clientConn) onIdleTimeout() {
	cc.closeIfIdle()
}

func (cc *clientConn) closeIfIdle() {
	cc.mu.Lock()
	if len(cc.streams) > 0 || cc.streamsReserved > 0 {
		cc.mu.Unlock()
		return
	}
	cc.closed = true
	nextID := cc.nextStreamID
	// TODO: do clients send GOAWAY too? maybe? Just Close:
	cc.mu.Unlock()

	if VerboseLogs {
		hlog.Infof("http2: Transport closing idle conn %p (forSingleUse=%v, maxStream=%v)", cc, cc.singleUse, nextID-2)
	}

	cc.tconn.Close()
}

var shutdownEnterWaitStateHook = func() {}

// Shutdown gracefully closes the client connection, waiting for running streams to complete.
func (cc *clientConn) Shutdown(ctx context.Context) error {
	if err := cc.sendGoAway(); err != nil {
		return err
	}
	// Wait for all in-flight streams to complete or connection to close
	done := make(chan error, 1)
	cancelled := false // guarded by cc.mu
	go func() {
		defer func() {
			if r := recover(); r != nil {
				hlog.Errorf("HERTZ: h2 client shutdown panic: %v", r)
			}
		}()

		cc.mu.Lock()
		defer cc.mu.Unlock()
		for {
			if len(cc.streams) == 0 || cc.closed {
				cc.closed = true
				done <- cc.tconn.Close()
				break
			}
			if cancelled {
				break
			}
			cc.cond.Wait()
		}
	}()
	shutdownEnterWaitStateHook()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		cc.mu.Lock()
		// Free the goroutine above
		cancelled = true
		cc.cond.Broadcast()
		cc.mu.Unlock()
		return ctx.Err()
	}
}

func (cc *clientConn) sendGoAway() error {
	cc.mu.Lock()
	closing := cc.closing
	cc.closing = true
	maxStreamID := cc.nextStreamID
	cc.mu.Unlock()
	if closing {
		// GOAWAY sent already
		return nil
	}

	cc.wmu.Lock()
	defer cc.wmu.Unlock()
	// Send a graceful shutdown frame to server
	if err := cc.fr.WriteGoAway(maxStreamID, ErrCodeNo, nil); err != nil {
		return err
	}
	if err := cc.bw.Flush(); err != nil {
		return err
	}
	// Prevent new requests
	return nil
}

// closes the client connection immediately. In-flight requests are interrupted.
// err is sent to streams.
func (cc *clientConn) closeForError(err error) error {
	cc.mu.Lock()
	cc.closed = true
	for _, cs := range cc.streams {
		cs.abortStreamLocked(err)
	}
	defer cc.cond.Broadcast()
	defer cc.mu.Unlock()

	return cc.tconn.Close()
}

// Close closes the client connection immediately.
//
// In-flight requests are interrupted. For a graceful shutdown, use Shutdown instead.
func (cc *clientConn) Close() error {
	err := errors.New("http2: client connection force closed via ClientConn.Close")
	return cc.closeForError(err)
}

// closes the client connection immediately. In-flight requests are interrupted.
func (cc *clientConn) closeForLostPing() error {
	err := errors.New("http2: client connection lost")
	return cc.closeForError(err)
}

// checkConnHeaders checks whether req has any invalid connection-level headers.
// per RFC 7540 section 8.1.2.2: Connection-Specific Header Fields.
// Certain headers are special-cased as okay but not transmitted later.
func checkConnHeaders(req *protocol.Request) error {
	if v := req.Header.Get("Upgrade"); v != "" {
		return fmt.Errorf("http2: invalid Upgrade request header: %q", req.Header.Get("Upgrade"))
	}
	if vv := req.Header.Get("Transfer-Encoding"); len(vv) > 0 && vv != "chunked" {
		return fmt.Errorf("http2: invalid Transfer-Encoding request header: %q", vv)
	}
	if vv := req.Header.Get("Connection"); len(vv) > 0 && strings.ToLower(vv) != "close" && strings.ToLower(vv) != "keep-alive" {
		return fmt.Errorf("http2: invalid Connection request header: %q", vv)
	}
	return nil
}

// actualContentLength returns a sanitized version of
// req.ContentLength, where 0 actually means zero (not unknown) and -1
// means unknown.
func actualContentLength(req *protocol.Request) int64 {
	if req.BodyStream() == ext.NoBody {
		return int64(len(req.Body()))
	}

	if req.Header.ContentLength() != 0 {
		return int64(req.Header.ContentLength())
	}
	return -1
}

func (cc *clientConn) decrStreamReservations() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.decrStreamReservationsLocked()
}

func (cc *clientConn) decrStreamReservationsLocked() {
	if cc.streamsReserved > 0 {
		cc.streamsReserved--
	}
}

// doRequest runs for the duration of the request lifetime.
//
// It sends the request and performs post-request cleanup (closing Request.Body, etc.).
func (cs *clientStream) doRequest(req *protocol.Request) {
	err := cs.writeRequest(req)
	cs.cleanupWriteRequest(err)
}

// writeRequest sends a request.
//
// It returns nil after the request is written, the response read,
// and the request stream is half-closed by the peer.
//
// It returns non-nil if the request ends otherwise.
// If the returned error is StreamError, the error Code may be used in resetting the stream.
func (cs *clientStream) writeRequest(req *protocol.Request) (err error) {
	cc := cs.cc
	ctx := cs.ctx

	if err := checkConnHeaders(req); err != nil {
		return err
	}

	// Acquire the new-request lock by writing to reqHeaderMu.
	// This lock guards the critical section covering allocating a new stream ID
	// (requires mu) and creating the stream (requires wmu).
	if cc.reqHeaderMu == nil {
		panic("RoundTrip on uninitialized clientConn") // for tests
	}
	select {
	case cc.reqHeaderMu <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	cc.mu.Lock()
	if cc.idleTimer != nil {
		cc.idleTimer.Stop()
	}
	cc.decrStreamReservationsLocked()
	if err := cc.awaitOpenSlotForStreamLocked(cs); err != nil {
		cc.mu.Unlock()
		<-cc.reqHeaderMu
		return err
	}
	cc.addStreamLocked(cs) // assigns stream ID
	if req.Header.ConnectionClose() {
		cc.doNotReuse = true
	}
	cc.mu.Unlock()

	// Past this point (where we send request headers), it is possible for
	// RoundTrip to return successfully. Since the RoundTrip contract permits
	// the caller to "mutate or reuse" the Request after closing the Response's Body,
	// we must take care when referencing the Request from here on.
	err = cs.encodeAndWriteHeaders(req)
	<-cc.reqHeaderMu
	if err != nil {
		return err
	}

	hasBody := cs.reqBodyContentLength != 0
	if !hasBody {
		cs.sentEndStream = true
	} else {
		if err = cs.writeRequestBody(req); err != nil {
			if err != errStopReqBodyWrite {
				return err
			}
		} else {
			cs.sentEndStream = true
		}
	}

	// Wait until the peer half-closes its end of the stream,
	// or until the request is aborted (via context, error, or otherwise),
	// whichever comes first.
	for {
		select {
		case <-cs.peerClosed:
			return nil
		case <-cs.abort:
			return cs.abortErr
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (cs *clientStream) encodeAndWriteHeaders(req *protocol.Request) error {
	cc := cs.cc
	ctx := cs.ctx

	cc.wmu.Lock()
	defer cc.wmu.Unlock()

	// If the request was canceled while waiting for cc.mu, just quit.
	select {
	case <-cs.abort:
		return cs.abortErr
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	contentLen := actualContentLength(req)
	hasBody := contentLen != 0
	hdrs, err := cc.encodeHeaders(req, cs.requestedGzip, contentLen)
	if err != nil {
		return err
	}

	// Write the request.
	endStream := !hasBody
	cs.sentHeaders = true
	err = cc.writeHeaders(cs.ID, endStream, int(cc.maxFrameSize), hdrs)
	return err
}

// cleanupWriteRequest performs post-request tasks.
//
// If err (the result of writeRequest) is non-nil and the stream is not closed,
// cleanupWriteRequest will send a reset to the peer.
func (cs *clientStream) cleanupWriteRequest(err error) {
	cc := cs.cc

	if cs.ID == 0 {
		// We were canceled before creating the stream, so return our reservation.
		cc.decrStreamReservations()
	}

	// TODO: write h12Compare test showing whether
	// Request.Body is closed by the Transport,
	// and in multiple cases: server replies <=299 and >299
	// while still writing request body
	cc.mu.Lock()
	bodyClosed := cs.reqBodyClosed
	cs.reqBodyClosed = true
	cc.mu.Unlock()
	if !bodyClosed && cs.reqBody != nil {
		if bsc, ok := cs.reqBody.(io.Closer); ok {
			bsc.Close()
		}
	}

	if err != nil && cs.sentEndStream {
		// If the connection is closed immediately after the response is read,
		// we may be aborted before finishing up here. If the stream was closed
		// cleanly on both sides, there is no error.
		select {
		case <-cs.peerClosed:
			err = nil
		default:
		}
	}
	if err != nil {
		cs.abortStream(err) // possibly redundant, but harmless
		if cs.sentHeaders {
			if se, ok := err.(StreamError); ok {
				if se.Cause != errFromPeer {
					cc.writeStreamReset(cs.ID, se.Code, err)
				}
			} else {
				cc.writeStreamReset(cs.ID, ErrCodeCancel, err)
			}
		}
		cs.bufPipe.CloseWithError(err) // no-op if already closed
	} else {
		if cs.sentHeaders && !cs.sentEndStream {
			cc.writeStreamReset(cs.ID, ErrCodeNo, nil)
		}
		cs.bufPipe.CloseWithError(errRequestCanceled)
	}
	if cs.ID != 0 {
		cc.forgetStreamID(cs.ID)
	}

	cc.wmu.Lock()
	werr := cc.werr
	cc.wmu.Unlock()
	if werr != nil {
		cc.Close()
	}

	close(cs.donec)
}

// awaitOpenSlotForStream waits until len(streams) < maxConcurrentStreams.
// Must hold cc.mu.
func (cc *clientConn) awaitOpenSlotForStreamLocked(cs *clientStream) error {
	for {
		cc.lastActive = time.Now()
		if cc.closed || !cc.canTakeNewRequestLocked() {
			return errClientConnUnusable
		}
		cc.lastIdle = time.Time{}
		if int64(len(cc.streams)) < int64(cc.maxConcurrentStreams) {
			return nil
		}
		cc.pendingRequests++
		cc.cond.Wait()
		cc.pendingRequests--
		select {
		case <-cs.abort:
			return cs.abortErr
		default:
		}
	}
}

// requires cc.wmu be held
func (cc *clientConn) writeHeaders(streamID uint32, endStream bool, maxFrameSize int, hdrs []byte) error {
	first := true // first frame written (HEADERS is first, then CONTINUATION)
	for len(hdrs) > 0 && cc.werr == nil {
		chunk := hdrs
		if len(chunk) > maxFrameSize {
			chunk = chunk[:maxFrameSize]
		}
		hdrs = hdrs[len(chunk):]
		endHeaders := len(hdrs) == 0
		if first {
			cc.fr.WriteHeaders(HeadersFrameParam{
				StreamID:      streamID,
				BlockFragment: chunk,
				EndStream:     endStream,
				EndHeaders:    endHeaders,
			})
			first = false
		} else {
			cc.fr.WriteContinuation(streamID, endHeaders, chunk)
		}
	}
	cc.bw.Flush()
	return cc.werr
}

// frameScratchBufferLen returns the length of a buffer to use for
// outgoing request bodies to read/write to/from.
//
// It returns max(1, min(peer's advertised max frame size,
// Request.ContentLength+1, 512KB)).
func (cs *clientStream) frameScratchBufferLen(maxFrameSize int) int {
	const max = 512 << 10
	n := int64(maxFrameSize)
	if n > max {
		n = max
	}
	if cl := cs.reqBodyContentLength; cl != -1 && cl+1 < n {
		// Add an extra byte past the declared content-length to
		// give the caller's Request.Body io.Reader a chance to
		// give us more bytes than they declared, so we can catch it
		// early.
		n = cl + 1
	}
	if n < 1 {
		return 1
	}
	return int(n) // doesn't truncate; max is 512K
}

var bufPool sync.Pool // of *[]byte

func (cs *clientStream) writeRequestBody(req *protocol.Request) (err error) {
	cc := cs.cc
	body := cs.reqBody
	sentEnd := false // whether we sent the final DATA frame w/ END_STREAM

	remainLen := cs.reqBodyContentLength
	hasContentLen := remainLen != -1

	cc.mu.Lock()
	maxFrameSize := int(cc.maxFrameSize)
	cc.mu.Unlock()

	// Scratch buffer for reading into & writing from.
	scratchLen := cs.frameScratchBufferLen(maxFrameSize)
	var buf []byte
	if bp, ok := bufPool.Get().(*[]byte); ok && len(*bp) >= scratchLen {
		defer bufPool.Put(bp)
		buf = *bp
	} else {
		buf = make([]byte, scratchLen)
		defer bufPool.Put(&buf)
	}

	var sawEOF bool
	for !sawEOF {
		n, err := body.Read(buf[:])
		if hasContentLen {
			remainLen -= int64(n)
			if remainLen == 0 && err == nil {
				// The request body's Content-Length was predeclared and
				// we just finished reading it all, but the underlying io.Reader
				// returned the final chunk with a nil error (which is one of
				// the two valid things a Reader can do at EOF). Because we'd prefer
				// to send the END_STREAM bit early, double-check that we're actually
				// at EOF. Subsequent reads should return (0, EOF) at this point.
				// If either value is different, we return an error in one of two ways below.
				var scratch [1]byte
				var n1 int
				n1, err = body.Read(scratch[:])
				remainLen -= int64(n1)
			}
			if remainLen < 0 {
				err = errReqBodyTooLong
				return err
			}
		}
		if err != nil {
			cc.mu.Lock()
			bodyClosed := cs.reqBodyClosed
			cc.mu.Unlock()
			switch {
			case bodyClosed:
				return errStopReqBodyWrite
			case err == io.EOF:
				sawEOF = true
				err = nil
			default:
				return err
			}
		}

		remain := buf[:n]
		for len(remain) > 0 && err == nil {
			var allowed int32
			allowed, err = cs.awaitFlowControl(len(remain))
			if err != nil {
				return err
			}
			cc.wmu.Lock()
			data := remain[:allowed]
			remain = remain[allowed:]
			sentEnd = sawEOF && len(remain) == 0
			err = cc.fr.WriteData(cs.ID, sentEnd, data)
			if err == nil {
				// TODO(bradfitz): this flush is for latency, not bandwidth.
				// Most requests won't need this. Make this opt-in or
				// opt-out?  Use some heuristic on the body type? Nagel-like
				// timers?  Based on 'n'? Only last chunk of this for loop,
				// unless flow control tokens are low? For now, always.
				// If we change this, see comment below.
				err = cc.bw.Flush()
			}
			cc.wmu.Unlock()
		}
		if err != nil {
			return err
		}
	}

	if sentEnd {
		// Already sent END_STREAM (which implies we have no
		// trailers) and flushed, because currently all
		// WriteData frames above get a flush. So we're done.
		return nil
	}

	// Since the RoundTrip contract permits the caller to "mutate or reuse"
	// a request after the Response's Body is closed, verify that this hasn't
	// happened before accessing the trailers.
	cc.mu.Lock()
	err = cs.abortErr
	cc.mu.Unlock()
	if err != nil {
		return err
	}

	cc.wmu.Lock()
	defer cc.wmu.Unlock()

	err = cc.fr.WriteData(cs.ID, true, nil)
	if ferr := cc.bw.Flush(); ferr != nil && err == nil {
		err = ferr
	}

	return err
}

// awaitFlowControl waits for [1, min(maxBytes, cc.cs.maxFrameSize)] flow
// control tokens from the server.
// It returns either the non-zero number of tokens taken or an error
// if the stream is dead.
func (cs *clientStream) awaitFlowControl(maxBytes int) (taken int32, err error) {
	cc := cs.cc
	ctx := cs.ctx
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for {
		if cc.closed {
			return 0, errClientConnClosed
		}
		if cs.reqBodyClosed {
			return 0, errStopReqBodyWrite
		}
		select {
		case <-cs.abort:
			return 0, cs.abortErr
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		if a := cs.flow.available(); a > 0 {
			take := a
			if int(take) > maxBytes {
				take = int32(maxBytes) // can't truncate int; take is int32
			}
			if take > int32(cc.maxFrameSize) {
				take = int32(cc.maxFrameSize)
			}
			cs.flow.take(take)
			return take, nil
		}
		cc.cond.Wait()
	}
}

// requires cc.wmu be held.
func (cc *clientConn) encodeHeaders(req *protocol.Request, addGzipHeader bool, contentLength int64) ([]byte, error) {
	cc.hbuf.Reset()
	if len(req.URI().Host()) == 0 {
		return nil, errNilRequestURL
	}

	host, err := httpguts.PunycodeHostPort(string(req.Host()))
	if err != nil {
		return nil, err
	}

	var path string
	if !bytes.Equal(req.Method(), bytestr.StrConnect) {
		path = string(req.URI().RequestURI())
		if !validPseudoPath(path) {
			orig := path
			path = strings.TrimPrefix(path, bytesconv.B2s(req.URI().Scheme())+"://"+host)
			if !validPseudoPath(path) {
				return nil, fmt.Errorf("invalid request :path %q", orig)
			}
		}
	}

	// Check for any invalid headers and return an error before we
	// potentially pollute our hpack state. (We want to be able to
	// continue to reuse the hpack encoder for future requests)
	req.Header.VisitAll(func(k, v []byte) {
		if err != nil {
			return
		}
		if !httpguts.ValidHeaderFieldName(bytesconv.B2s(k)) {
			err = fmt.Errorf("invalid HTTP header name %q", k)
		}
		if !httpguts.ValidHeaderFieldValue(bytesconv.B2s(v)) {
			err = fmt.Errorf("invalid HTTP header value %q for header %q", v, k)
		}
	})

	if err != nil {
		return nil, err
	}

	enumerateHeaders := func(f func(name, value string)) {
		// 8.1.2.3 Request Pseudo-Header Fields
		// The :path pseudo-header field includes the path and query parts of the
		// target URI (the path-absolute production and optionally a '?' character
		// followed by the query production (see Sections 3.3 and 3.4 of
		// [RFC3986]).
		f(":authority", host)
		m := req.Method()
		f(":method", string(m))
		if !bytes.Equal(req.Method(), bytestr.StrConnect) {
			f(":path", path)
			f(":scheme", string(req.URI().Scheme()))
		}

		var didUA bool
		req.Header.VisitAll(func(k, v []byte) {
			if strings.ToLower(bytesconv.B2s(k)) == "host" || strings.ToLower(bytesconv.B2s(k)) == "content-length" {
				// Host is :authority, already sent.
				// Content-Length is automatic, set below.
				return
			} else if strings.ToLower(bytesconv.B2s(k)) == "connection" ||
				strings.ToLower(bytesconv.B2s(k)) == "proxy-connection" ||
				strings.ToLower(bytesconv.B2s(k)) == "transfer-encoding" ||
				strings.ToLower(bytesconv.B2s(k)) == "upgrade" ||
				strings.ToLower(bytesconv.B2s(k)) == "keep-alive" {
				// Per 8.1.2.2 Connection-Specific Header
				// Fields, don't send connection-specific
				// fields. We have already checked if any
				// are error-worthy so just ignore the rest.
				return
			} else if strings.ToLower(bytesconv.B2s(k)) == "user-agent" {
				// Match Go's http1 behavior: at most one
				// User-Agent. If set to nil or empty string,
				// then omit it. Otherwise if not mentioned,
				// include the default (below).
				didUA = true
				if len(v) < 1 {
					return
				}
			} else if strings.ToLower(bytesconv.B2s(k)) == "cookie" {
				// Per 8.1.2.5 To allow for better compression efficiency, the
				// Cookie header field MAY be split into separate header fields,
				// each with one or more cookie-pairs.

				if len(v) > 0 {
					for {
						p := bytes.IndexByte(v, ';')
						if p < 0 {
							break
						}
						f("cookie", string(v[:p]))
						p++
						for p+1 <= len(v) && v[p] == ' ' {
							p++
						}
						v = v[p:]
					}
					if len(v) > 0 {
						f("cookie", string(v))
					}
				}

				return
			}
			f(string(k), string(v))
		})

		if shouldSendReqContentLength(bytesconv.B2s(req.Method()), contentLength) {
			f("content-length", strconv.FormatInt(int64(contentLength), 10))
		}
		if addGzipHeader {
			f("accept-encoding", "gzip")
		}
		if !didUA {
			f("user-agent", string(bytestr.DefaultUserAgent))
		}
	}

	// Do a first pass over the headers counting bytes to ensure
	// we don't exceed cc.peerMaxHeaderListSize. This is done as a
	// separate pass before encoding the headers to prevent
	// modifying the hpack state.
	hlSize := uint64(0)
	enumerateHeaders(func(name, value string) {
		hf := hpack.HeaderField{Name: name, Value: value}
		hlSize += uint64(hf.Size())
	})

	if hlSize > cc.peerMaxHeaderListSize {
		return nil, errRequestHeaderListSize
	}

	// Header list size is ok. Write the headers.
	enumerateHeaders(func(name, value string) {
		name = strings.ToLower(name)
		cc.writeHeader(name, value)
	})

	return cc.hbuf.Bytes(), nil
}

func shouldSendReqContentLength(method string, contentLength int64) bool {
	if contentLength > 0 {
		return true
	}
	if contentLength < 0 {
		return false
	}
	// For zero bodies, whether we send a content-length depends on the method.
	// It also kinda doesn't matter for http2 either way, with END_STREAM.
	switch method {
	case "POST", "PUT", "PATCH":
		return true
	default:
		return false
	}
}

func (cc *clientConn) writeHeader(name, value string) {
	if VerboseLogs {
		hlog.Infof("http2: Transport encoding header %q = %q", name, value)
	}

	cc.henc.WriteField(hpack.HeaderField{Name: name, Value: value})
}

// requires cc.mu be held.
func (cc *clientConn) addStreamLocked(cs *clientStream) {
	cs.flow.add(int32(cc.initialWindowSize))
	cs.flow.setConnFlow(&cc.flow)
	cs.inflow.add(transportDefaultStreamFlow)
	cs.inflow.setConnFlow(&cc.inflow)
	cs.ID = cc.nextStreamID
	cc.nextStreamID += 2
	cc.streams[cs.ID] = cs
	if cs.ID == 0 {
		panic("assigned stream ID 0")
	}
}

func (cc *clientConn) forgetStreamID(id uint32) {
	cc.mu.Lock()
	slen := len(cc.streams)
	delete(cc.streams, id)
	if len(cc.streams) != slen-1 {
		panic("forgetting unknown stream id")
	}
	cc.lastActive = time.Now()
	if len(cc.streams) == 0 && cc.idleTimer != nil {
		cc.idleTimer.Reset(cc.idleTimeout)
		cc.lastIdle = time.Now()
	}
	// Wake up writeRequestBody via clientStream.awaitFlowControl and
	// wake up RoundTrip if there is a pending request.
	cc.cond.Broadcast()

	closeOnIdle := cc.singleUse || cc.doNotReuse || !cc.hc.KeepAlive
	if closeOnIdle && cc.streamsReserved == 0 && len(cc.streams) == 0 {
		if VerboseLogs {
			hlog.Infof("http2: Transport closing idle conn %p (forSingleUse=%v, maxStream=%v)", cc, cc.singleUse, cc.nextStreamID-2)
		}

		cc.closed = true
		defer cc.tconn.Close()
	}

	cc.mu.Unlock()
}

// clientConnReadLoop is the state owned by the clientConn's frame-reading readLoop.
type clientConnReadLoop struct {
	cc *clientConn
}

// readLoop runs in its own goroutine and reads and dispatches frames.
func (cc *clientConn) readLoop() {
	rl := &clientConnReadLoop{cc: cc}
	defer rl.cleanup()
	cc.readerErr = rl.run()
	if ce, ok := cc.readerErr.(ConnectionError); ok {
		cc.wmu.Lock()
		cc.fr.WriteGoAway(0, ErrCode(ce), nil)
		cc.wmu.Unlock()
	}
}

// GoAwayError is returned by the Transport when the server closes the
// TCP connection after sending a GOAWAY frame.
type GoAwayError struct {
	LastStreamID uint32
	ErrCode      ErrCode
	DebugData    string
}

func (e GoAwayError) Error() string {
	return fmt.Sprintf("http2: server sent GOAWAY and closed the connection; LastStreamID=%v, ErrCode=%v, debug=%q",
		e.LastStreamID, e.ErrCode, e.DebugData)
}

func isEOFOrNetReadError(err error) bool {
	if err == io.EOF {
		return true
	}
	ne, ok := err.(*net.OpError)
	return ok && ne.Op == "read"
}

func (rl *clientConnReadLoop) cleanup() {
	cc := rl.cc
	defer cc.tconn.Close()
	defer cc.hc.onConnectionDropped(cc)
	defer close(cc.readerDone)

	if cc.idleTimer != nil {
		cc.idleTimer.Stop()
	}

	// Close any response bodies if the server closes prematurely.
	// TODO: also do this if we've written the headers but not
	// gotten a response yet.
	err := cc.readerErr
	cc.mu.Lock()
	if cc.goAway != nil && isEOFOrNetReadError(err) {
		err = GoAwayError{
			LastStreamID: cc.goAway.LastStreamID,
			ErrCode:      cc.goAway.ErrCode,
			DebugData:    cc.goAwayDebug,
		}
	} else if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	cc.closed = true
	for _, cs := range cc.streams {
		select {
		case <-cs.peerClosed:
			// The server closed the stream before closing the conn,
			// so no need to interrupt it.
		default:
			cs.abortStreamLocked(err)
		}
	}
	cc.cond.Broadcast()
	cc.mu.Unlock()
}

func (rl *clientConnReadLoop) run() error {
	cc := rl.cc
	gotSettings := false
	readIdleTimeout := cc.hc.ReadIdleTimeout
	var t *time.Timer
	if readIdleTimeout != 0 {
		t = time.AfterFunc(readIdleTimeout, cc.healthCheck)
		defer t.Stop()
	}
	for {
		f, err := cc.fr.ReadFrame()
		if t != nil {
			t.Reset(readIdleTimeout)
		}

		if err != nil {
			if err != io.EOF {
				hlog.Errorf("http2: Transport readFrame error on conn %p: (%T) %v", cc, err, err)
			} else {
				hlog.Infof("http2: Transport readFrame error on conn %p: (%T) %v", cc, err, err)
			}
		}
		if se, ok := err.(StreamError); ok {
			if cs := rl.streamByID(se.StreamID); cs != nil {
				if se.Cause == nil {
					se.Cause = cc.fr.errDetail
				}
				rl.endStreamError(cs, se)
			}
			continue
		} else if err != nil {
			return err
		}

		if VerboseLogs {
			hlog.Infof("http2: Transport received %s", summarizeFrame(f))
		}

		if !gotSettings {
			if _, ok := f.(*SettingsFrame); !ok {
				hlog.Errorf("protocol error: received %T before a SETTINGS frame", f)
				return ConnectionError(ErrCodeProtocol)
			}
			gotSettings = true
		}

		switch f := f.(type) {
		case *MetaHeadersFrame:
			err = rl.processHeaders(f)
		case *DataFrame:
			err = rl.processData(f)
		case *GoAwayFrame:
			err = rl.processGoAway(f)
		case *RSTStreamFrame:
			err = rl.processResetStream(f)
		case *SettingsFrame:
			err = rl.processSettings(f)
		case *PushPromiseFrame:
			err = rl.processPushPromise(f)
		case *WindowUpdateFrame:
			err = rl.processWindowUpdate(f)
		case *PingFrame:
			err = rl.processPing(f)
		default:
			hlog.Errorf("Transport: unhandled response frame type %T", f)
		}
		if err != nil {
			hlog.Errorf("http2: Transport conn %p received error from processing frame %v: %v", cc, summarizeFrame(f), err)
			return err
		}
	}
}

func (rl *clientConnReadLoop) processHeaders(f *MetaHeadersFrame) error {
	cs := rl.streamByID(f.StreamID)
	if cs == nil {
		// We'd get here if we canceled a request while the
		// server had its response still in flight. So if this
		// was just something we canceled, ignore it.
		return nil
	}
	if cs.readClosed {
		rl.endStreamError(cs, StreamError{
			StreamID: f.StreamID,
			Code:     ErrCodeProtocol,
			Cause:    errors.New("protocol error: headers after END_STREAM"),
		})
		return nil
	}
	if !cs.firstByte {
		cs.firstByte = true
	}

	res, err := rl.handleResponse(cs, f)
	if err != nil {
		if _, ok := err.(ConnectionError); ok {
			return err
		}
		// Any other error type is a stream error.
		rl.endStreamError(cs, StreamError{
			StreamID: f.StreamID,
			Code:     ErrCodeProtocol,
			Cause:    err,
		})
		return nil // return nil from process* funcs to keep conn alive
	}

	if res == nil {
		// (nil, nil) special case. See handleResponse docs.
		return nil
	}
	cs.res = res
	close(cs.respHeaderRecv)
	if f.StreamEnded() {
		rl.endStream(cs)
	}
	return nil
}

// may return error types nil, or ConnectionError. Any other error value
// is a StreamError of type ErrCodeProtocol. The returned error in that case
// is the detail.
//
// As a special case, handleResponse may return (nil, nil) to skip the
// frame (currently only used for 1xx responses).
func (rl *clientConnReadLoop) handleResponse(cs *clientStream, f *MetaHeadersFrame) (*protocol.Response, error) {
	if f.Truncated {
		return nil, errResponseHeaderListSize
	}

	status := f.PseudoValue("status")
	if status == "" {
		return nil, errors.New("malformed response from server: missing status pseudo header")
	}
	statusCode, err := strconv.Atoi(status)
	if err != nil {
		return nil, errors.New("malformed response from server: malformed non-numeric status pseudo header")
	}

	regularFields := f.RegularFields()

	res := cs.res
	res.Header.Reset()
	res.SetStatusCode(statusCode)
	res.Header.SetContentLength(-1)
	for _, hf := range regularFields {
		res.Header.Add(hf.Name, hf.Value)
	}

	if statusCode >= 100 && statusCode <= 199 {
		if f.StreamEnded() {
			return nil, errors.New("1xx informational response with END_STREAM flag")
		}
		cs.num1xx++
		const max1xxResponses = 5 // arbitrary bound on number of informational responses, same as net/http
		if cs.num1xx > max1xxResponses {
			return nil, errors.New("http2: too many 1xx informational responses")
		}
		if fn := cs.get1xxTraceFunc(); fn != nil {
			if err := fn(statusCode, &res.Header); err != nil {
				return nil, err
			}
		}
		if statusCode == 100 {
			select {
			case cs.on100 <- struct{}{}:
			default:
			}
		}
		return nil, nil
	}

	if cs.isHead {
		res.SetBodyStream(noBody, res.Header.ContentLength())
		return res, nil
	}

	if f.StreamEnded() {
		if res.Header.ContentLength() > 0 {
			res.SetBodyStream(missingBody{}, res.Header.ContentLength())
		} else {
			res.SetBodyStream(noBody, 0)
		}
		return res, nil
	}

	cs.bufPipe.setBuffer(newDataBuffer(bytebufferpool.Get()))
	cs.bytesRemain = int64(res.Header.ContentLength())
	res.SetBodyStream(transportResponseBody{cs}, res.Header.ContentLength())

	if cs.requestedGzip && res.Header.Get("Content-Encoding") == "gzip" {
		res.Header.Del("Content-Encoding")
		res.Header.Del("Content-Length")
		res.Header.SetContentLength(-1)
		r, err := compress.AcquireGzipReader(res.BodyStream())
		if err != nil {
			return nil, err
		}
		res.SetBodyStream(r, -1)
	}

	return res, nil
}

// transportResponseBody is the concrete type of Transport.RoundTrip's
// Response.Body. It is an io.ReadCloser.
type transportResponseBody struct {
	cs *clientStream
}

func (b transportResponseBody) Read(p []byte) (n int, err error) {
	cs := b.cs
	cc := cs.cc

	if cs.readErr != nil {
		return 0, cs.readErr
	}
	n, err = b.cs.bufPipe.Read(p)
	if cs.bytesRemain != -1 {
		if int64(n) > cs.bytesRemain {
			n = int(cs.bytesRemain)
			if err == nil {
				err = errors.New("net/http: server replied with more than declared Content-Length; truncated")
				cs.abortStream(err)
			}
			cs.readErr = err
			return int(cs.bytesRemain), err
		}
		cs.bytesRemain -= int64(n)
		if err == io.EOF && cs.bytesRemain > 0 {
			err = io.ErrUnexpectedEOF
			cs.readErr = err
			return n, err
		}
	}
	if n == 0 {
		// No flow control tokens to send back.
		return
	}

	cc.mu.Lock()
	var connAdd, streamAdd int32
	// Check the conn-level first, before the stream-level.
	if v := cc.inflow.available(); v < transportDefaultConnFlow/2 {
		connAdd = transportDefaultConnFlow - v
		cc.inflow.add(connAdd)
	}
	if err == nil { // No need to refresh if the stream is over or failed.
		// Consider any buffered body data (read from the conn but not
		// consumed by the client) when computing flow control for this
		// stream.
		v := int(cs.inflow.available()) + cs.bufPipe.Len()
		if v < transportDefaultStreamFlow-transportDefaultStreamMinRefresh {
			streamAdd = int32(transportDefaultStreamFlow - v)
			cs.inflow.add(streamAdd)
		}
	}
	cc.mu.Unlock()

	if connAdd != 0 || streamAdd != 0 {
		cc.wmu.Lock()
		defer cc.wmu.Unlock()
		if connAdd > 0 {
			cc.fr.WriteWindowUpdate(0, uint32(connAdd))
		}
		if streamAdd > 0 {
			cc.fr.WriteWindowUpdate(cs.ID, uint32(streamAdd))
		}
		cc.bw.Flush()
	}
	return
}

var errClosedResponseBody = errors.New("http2: response body closed")

func (b transportResponseBody) Close() error {
	cs := b.cs
	cc := cs.cc

	unread := cs.bufPipe.Len()
	if unread > 0 {
		cc.mu.Lock()
		// Return connection-level flow control.
		if unread > 0 {
			cc.inflow.add(int32(unread))
		}
		cc.mu.Unlock()

		// TODO: Acquiring this mutex can block indefinitely.
		// Move flow control return to a goroutine?
		cc.wmu.Lock()
		// Return connection-level flow control.
		if unread > 0 {
			cc.fr.WriteWindowUpdate(0, uint32(unread))
		}
		cc.bw.Flush()
		cc.wmu.Unlock()
	}

	cs.bufPipe.BreakWithError(errClosedResponseBody)
	cs.abortStream(errClosedResponseBody)
	select {
	case <-cs.donec:
	case <-cs.ctx.Done():
		return cs.ctx.Err()
	}
	return nil
}

func (rl *clientConnReadLoop) processData(f *DataFrame) error {
	cc := rl.cc
	cs := rl.streamByID(f.StreamID)
	data := f.Data()
	if cs == nil {
		cc.mu.Lock()
		neverSent := cc.nextStreamID
		cc.mu.Unlock()
		if f.StreamID >= neverSent {
			// We never asked for this.
			hlog.Error("http2: Transport received unsolicited DATA frame; closing connection")
			return ConnectionError(ErrCodeProtocol)
		}
		// We probably did ask for this, but canceled. Just ignore it.
		// TODO: be stricter here? only silently ignore things which
		// we canceled, but not things which were closed normally
		// by the peer? Tough without accumulating too much state.

		// But at least return their flow control:
		if f.Length > 0 {
			cc.mu.Lock()
			cc.inflow.add(int32(f.Length))
			cc.mu.Unlock()

			cc.wmu.Lock()
			cc.fr.WriteWindowUpdate(0, uint32(f.Length))
			cc.bw.Flush()
			cc.wmu.Unlock()
		}
		return nil
	}
	if cs.readClosed {
		hlog.Error("protocol error: received DATA after END_STREAM")
		rl.endStreamError(cs, StreamError{
			StreamID: f.StreamID,
			Code:     ErrCodeProtocol,
		})
		return nil
	}
	if !cs.firstByte {
		hlog.Error("protocol error: received DATA before a HEADERS frame")
		rl.endStreamError(cs, StreamError{
			StreamID: f.StreamID,
			Code:     ErrCodeProtocol,
		})
		return nil
	}
	if f.Length > 0 {
		if cs.isHead && len(data) > 0 {
			hlog.Error("protocol error: received DATA on a HEAD request")
			rl.endStreamError(cs, StreamError{
				StreamID: f.StreamID,
				Code:     ErrCodeProtocol,
			})
			return nil
		}
		// Check connection-level flow control.
		cc.mu.Lock()
		if cs.inflow.available() >= int32(f.Length) {
			cs.inflow.take(int32(f.Length))
		} else {
			cc.mu.Unlock()
			return ConnectionError(ErrCodeFlowControl)
		}
		// Return any padded flow control now, since we won't
		// refund it later on body reads.
		var refund int
		if pad := int(f.Length) - len(data); pad > 0 {
			refund += pad
		}

		didReset := false
		var err error
		if len(data) > 0 {
			if _, err = cs.bufPipe.Write(data); err != nil {
				// Return len(data) now if the stream is already closed,
				// since data will never be read.
				didReset = true
				refund += len(data)
			}
		}

		if refund > 0 {
			cc.inflow.add(int32(refund))
			if !didReset {
				cs.inflow.add(int32(refund))
			}
		}
		cc.mu.Unlock()

		if refund > 0 {
			cc.wmu.Lock()
			cc.fr.WriteWindowUpdate(0, uint32(refund))
			if !didReset {
				cc.fr.WriteWindowUpdate(cs.ID, uint32(refund))
			}
			cc.bw.Flush()
			cc.wmu.Unlock()
		}

		if err != nil {
			rl.endStreamError(cs, err)
			return nil
		}
	}

	if f.StreamEnded() {
		rl.endStream(cs)
	}
	return nil
}

func (rl *clientConnReadLoop) endStream(cs *clientStream) {
	// TODO: check that any declared content-length matches, like
	// server.go's (*stream).endStream method.
	if !cs.readClosed {
		cs.readClosed = true
		cs.bufPipe.closeWithErrorAndCode(io.EOF, func() {})
		close(cs.peerClosed)
	}
}

func (rl *clientConnReadLoop) endStreamError(cs *clientStream, err error) {
	cs.readAborted = true
	cs.abortStream(err)
}

func (rl *clientConnReadLoop) streamByID(id uint32) *clientStream {
	rl.cc.mu.Lock()
	defer rl.cc.mu.Unlock()
	cs := rl.cc.streams[id]
	if cs != nil && !cs.readAborted {
		return cs
	}
	return nil
}

func (rl *clientConnReadLoop) processGoAway(f *GoAwayFrame) error {
	cc := rl.cc
	cc.hc.onConnectionDropped(cc)
	if f.ErrCode != 0 {
		// TODO: deal with GOAWAY more. particularly the error code
		hlog.Errorf("transport got GOAWAY with error code = %v", f.ErrCode)
	}
	cc.setGoAway(f)

	return nil
}

func (rl *clientConnReadLoop) processSettings(f *SettingsFrame) error {
	cc := rl.cc
	// Locking both mu and wmu here allows frame encoding to read settings with only wmu held.
	// Acquiring wmu when f.IsAck() is unnecessary, but convenient and mostly harmless.
	cc.wmu.Lock()
	defer cc.wmu.Unlock()

	if err := rl.processSettingsNoWrite(f); err != nil {
		return err
	}
	if !f.IsAck() {
		cc.fr.WriteSettingsAck()
		cc.bw.Flush()
	}
	return nil
}

func (rl *clientConnReadLoop) processSettingsNoWrite(f *SettingsFrame) error {
	cc := rl.cc
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if f.IsAck() {
		if cc.wantSettingsAck {
			cc.wantSettingsAck = false
			return nil
		}
		return ConnectionError(ErrCodeProtocol)
	}

	var seenMaxConcurrentStreams bool
	err := f.ForeachSetting(func(s Setting) error {
		switch s.ID {
		case SettingMaxFrameSize:
			cc.maxFrameSize = s.Val
		case SettingMaxConcurrentStreams:
			cc.maxConcurrentStreams = s.Val
			seenMaxConcurrentStreams = true
		case SettingMaxHeaderListSize:
			cc.peerMaxHeaderListSize = uint64(s.Val)
		case SettingInitialWindowSize:
			// Values above the maximum flow-control
			// window size of 2^31-1 MUST be treated as a
			// connection error (Section 5.4.1) of type
			// FLOW_CONTROL_ERROR.
			if s.Val > math.MaxInt32 {
				return ConnectionError(ErrCodeFlowControl)
			}

			// Adjust flow control of currently-open
			// frames by the difference of the old initial
			// window size and this one.
			delta := int32(s.Val) - int32(cc.initialWindowSize)
			for _, cs := range cc.streams {
				cs.flow.add(delta)
			}
			cc.cond.Broadcast()

			cc.initialWindowSize = s.Val
		default:
			// TODO(bradfitz): handle more settings? SETTINGS_HEADER_TABLE_SIZE probably.
			hlog.Errorf("Unhandled Setting: %v", s)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if !cc.seenSettings {
		if !seenMaxConcurrentStreams {
			// This was the servers initial SETTINGS frame and it
			// didn't contain a MAX_CONCURRENT_STREAMS field so
			// increase the number of concurrent streams this
			// connection can establish to our default.
			cc.maxConcurrentStreams = defaultMaxConcurrentStreams
		}
		cc.seenSettings = true
	}

	return nil
}

func (rl *clientConnReadLoop) processWindowUpdate(f *WindowUpdateFrame) error {
	cc := rl.cc
	cs := rl.streamByID(f.StreamID)
	if f.StreamID != 0 && cs == nil {
		return nil
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	fl := &cc.flow
	if cs != nil {
		fl = &cs.flow
	}
	if !fl.add(int32(f.Increment)) {
		return ConnectionError(ErrCodeFlowControl)
	}
	cc.cond.Broadcast()
	return nil
}

func (rl *clientConnReadLoop) processResetStream(f *RSTStreamFrame) error {
	cs := rl.streamByID(f.StreamID)
	if cs == nil {
		// TODO: return error if server tries to RST_STREAM an idle stream
		return nil
	}
	serr := streamError(cs.ID, f.ErrCode)
	serr.Cause = errFromPeer
	if f.ErrCode == ErrCodeProtocol {
		rl.cc.SetDoNotReuse()
	}
	cs.abortStream(serr)

	cs.bufPipe.CloseWithError(serr)
	return nil
}

// Ping sends a PING frame to the server and waits for the ack.
func (cc *clientConn) Ping(ctx context.Context) error {
	c := make(chan struct{})
	// Generate a random payload
	var p [8]byte
	for {
		if _, err := rand.Read(p[:]); err != nil {
			return err
		}
		cc.mu.Lock()
		// check for dup before insert
		if _, found := cc.pings[p]; !found {
			cc.pings[p] = c
			cc.mu.Unlock()
			break
		}
		cc.mu.Unlock()
	}
	errc := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				hlog.Errorf("HERTZ: h2 client ping panic: %v", r)
			}
		}()

		cc.wmu.Lock()
		defer cc.wmu.Unlock()
		if err := cc.fr.WritePing(false, p); err != nil {
			errc <- err
			return
		}
		if err := cc.bw.Flush(); err != nil {
			errc <- err
			return
		}
	}()
	select {
	case <-c:
		return nil
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-cc.readerDone:
		// connection closed
		return cc.readerErr
	}
}

func (rl *clientConnReadLoop) processPing(f *PingFrame) error {
	cc := rl.cc
	if f.IsAck() {
		cc.mu.Lock()
		defer cc.mu.Unlock()
		// If ack, notify listener if any
		if c, ok := cc.pings[f.Data]; ok {
			close(c)
			delete(cc.pings, f.Data)
		}
		return nil
	}
	cc.wmu.Lock()
	defer cc.wmu.Unlock()
	if err := cc.fr.WritePing(true, f.Data); err != nil {
		return err
	}
	return cc.bw.Flush()
}

func (rl *clientConnReadLoop) processPushPromise(f *PushPromiseFrame) error {
	// We told the peer we don't want them.
	// Spec says:
	// "PUSH_PROMISE MUST NOT be sent if the SETTINGS_ENABLE_PUSH
	// setting of the peer endpoint is set to 0. An endpoint that
	// has set this setting and has received acknowledgement MUST
	// treat the receipt of a PUSH_PROMISE frame as a connection
	// error (Section 5.4.1) of type PROTOCOL_ERROR."
	return ConnectionError(ErrCodeProtocol)
}

func (cc *clientConn) writeStreamReset(streamID uint32, code ErrCode, err error) {
	// TODO: map err to more interesting error codes, once the
	// HTTP community comes up with some. But currently for
	// RST_STREAM there's no equivalent to GOAWAY frame's debug
	// data, and the error codes are all pretty vague ("cancel").
	cc.wmu.Lock()
	cc.fr.WriteRSTStream(streamID, code)
	cc.bw.Flush()
	cc.wmu.Unlock()
}

func NewHostClient(c *ClientOption) client.HostClient {
	hc := &HostClient{
		ClientOption: c,
	}
	hc.conns.Init()
	return hc
}

type ClientOption struct {
	// Comma-separated list of upstream HTTP server host addresses,
	// which are passed to Dial in a round-robin manner.
	//
	// Each address may contain port if default dialer is used.
	// For example,
	//
	//    - foobar.com:80
	//    - foobar.com:443
	//    - foobar.com:8080
	Addr string

	// NoDefaultUserAgentHeader when set to true, causes the default
	// User-Agent header to be excluded from the Request.
	NoDefaultUserAgentHeader bool

	// Default Dialer is used if not set.
	Dialer network.Dialer

	// Timeout for establishing new connections to hosts.
	//
	// Default DialTimeout is used if not set.
	DialTimeout time.Duration

	// Whether to use TLS (aka SSL or HTTPS) for host connections.
	// Optional TLS config.
	TLSConfig *tls.Config

	IsTLS bool

	// Idle keep-alive connections are closed after this duration.
	//
	// By default idle connections are closed
	// after DefaultMaxIdleConnDuration.
	MaxIdleConnDuration time.Duration

	// Maximum number of attempts for idempotent calls
	//
	// DefaultMaxIdemponentCallAttempts is used if not set.
	MaxIdemponentCallAttempts int

	// RetryIf controls whether a retry should be attempted after an error.
	//
	// By default will use isIdempotent function
	RetryIf client.RetryIfFunc

	// ResponseBodyStream enables response body streaming
	ResponseBodyStream bool

	// Connection will close after each request when set this to true.
	KeepAlive bool

	// MaxHeaderListSize is the http2 SETTINGS_MAX_HEADER_LIST_SIZE to
	// send in the initial settings frame. It is how many bytes
	// of response headers are allowed. Unlike the http2 spec, zero here
	// means to use a default limit (currently 10MB). If you actually
	// want to advertise an unlimited value to the peer, Transport
	// interprets the highest possible value here (0xffffffff or 1<<32-1)
	// to mean no limit.
	MaxHeaderListSize uint32

	// AllowHTTP, if true, permits HTTP/2 requests using the insecure,
	// plain-text "http" scheme. Note that this does not enable h2c support.
	AllowHTTP bool

	// ReadIdleTimeout is the timeout after which a health check using ping
	// frame will be carried out if no frame is received on the connection.
	// Note that a ping response will is considered a received frame, so if
	// there is no other traffic on the connection, the health check will
	// be performed every ReadIdleTimeout interval.
	// If zero, no health check is performed.
	ReadIdleTimeout time.Duration

	// PingTimeout is the timeout after which the connection will be closed
	// if a response to Ping is not received.
	// Defaults to 15s.
	PingTimeout time.Duration

	// WriteByteTimeout is the timeout after which the connection will be
	// closed no data can be written to it. The timeout begins when data is
	// available to write, and is extended whenever any bytes are written.
	WriteByteTimeout time.Duration

	// StrictMaxConcurrentStreams controls whether the server's
	// SETTINGS_MAX_CONCURRENT_STREAMS should be respected
	// globally. If false, new TCP connections are created to the
	// server as needed to keep each under the per-connection
	// SETTINGS_MAX_CONCURRENT_STREAMS limit. If true, the
	// server's SETTINGS_MAX_CONCURRENT_STREAMS is interpreted as
	// a global limit and callers of RoundTrip block when needed,
	// waiting for their turn.
	StrictMaxConcurrentStreams bool
}
