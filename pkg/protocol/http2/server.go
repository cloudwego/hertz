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
 * Copyright 2014 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

// TODO: turn off the serve goroutine when idle, so
// an idle conn only has the readFrames goroutine active. (which could
// also be optimized probably to pin less memory in crypto/tls). This
// would involve tracking when the serve goroutine is active (atomic
// int32 read/CAS probably?) and starting it up when frames arrive,
// and shutting it down when all handlers exit. the occasional PING
// packets could use time.AfterFunc to call sc.wakeStartServeLoop()
// (which is a no-op if already running) and then queue the PING write
// as normal. The serve loop would then exit in most cases (if no
// Handlers running) and not be woken up again until the PING packet
// returns.

// TODO (maybe): add a mechanism for Handlers to going into
// half-closed-local mode (rw.(io.Closer) test?) but not exit their
// handler, and continue to be able to read from the
// Request.Body. This would be a somewhat semantic change from HTTP/1
// (or at least what we expose in net/http), so I'd probably want to
// add it there too. For now, this package says that returning from
// the Handler ServeHTTP function means you're both done reading and
// done writing, without a way to stop just one or the other.

package http2

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http2/hpack"
	"github.com/cloudwego/hertz/pkg/protocol/suite"
)

const (
	prefaceTimeout         = 10 * time.Second
	firstSettingsTimeout   = 2 * time.Second // should be in-flight with preface anyway
	handlerChunkWriteSize  = 4 << 10
	defaultMaxStreams      = 250 // TODO: make this 100 as the GFE seems to?
	maxQueuedControlFrames = 10000
)

var (
	errClientDisconnected = errors.New("client disconnected")
	errClosedBody         = errors.New("body closed by handler")
	errHandlerComplete    = errors.New("http2: request body closed due to handler exiting")
	errStreamClosed       = errors.New("http2: stream closed")
)

var responseWriterStatePool = sync.Pool{
	New: func() interface{} {
		rws := &responseWriterState{}
		rws.bw = bufio.NewWriterSize(chunkWriter{rws}, handlerChunkWriteSize)
		return rws
	},
}

// Test hooks.
var (
	testHookGetServerConn func(*serverConn)
	testHookOnPanicMu     *sync.Mutex // nil except in tests
	testHookOnPanic       func(sc *serverConn, panicVal interface{}) (rePanic bool)
)

type BaseEngine struct {
	Option
	Core suite.Core
}

// Server is an HTTP/2 server.
type Server struct {
	BaseEngine

	// MaxHandlers limits the number of http.Handler ServeHTTP goroutines
	// which may run at a time over all connections.
	// Negative or zero no limit.
	// TODO: implement
	MaxHandlers int

	// MaxConcurrentStreams optionally specifies the number of
	// concurrent streams that each client may have open at a
	// time. This is unrelated to the number of http.Handler goroutines
	// which may be active globally, which is MaxHandlers.
	// If zero, MaxConcurrentStreams defaults to at least 100, per
	// the HTTP/2 spec's recommendations.
	MaxConcurrentStreams uint32

	// MaxReadFrameSize optionally specifies the largest frame
	// this server is willing to read. A valid value is between
	// 16k and 16M, inclusive. If zero or otherwise invalid, a
	// default value is used.
	MaxReadFrameSize uint32

	// PermitProhibitedCipherSuites, if true, permits the use of
	// cipher suites prohibited by the HTTP/2 spec.
	PermitProhibitedCipherSuites bool

	// IdleTimeout specifies how long until idle clients should be
	// closed with a GOAWAY frame. PING frames are not considered
	// activity for the purposes of IdleTimeout.
	IdleTimeout time.Duration

	// MaxUploadBufferPerConnection is the size of the initial flow
	// control window for each connections. The HTTP/2 spec does not
	// allow this to be smaller than 65535 or larger than 2^32-1.
	// If the value is outside this range, a default value will be
	// used instead.
	MaxUploadBufferPerConnection int32

	// MaxUploadBufferPerStream is the size of the initial flow control
	// window for each stream. The HTTP/2 spec does not allow this to
	// be larger than 2^32-1. If the value is zero or larger than the
	// maximum, a default value will be used instead.
	MaxUploadBufferPerStream int32

	// NewWriteScheduler constructs a write scheduler for a connection.
	// If nil, a default scheduler is chosen.
	NewWriteScheduler func() WriteScheduler

	// Internal state. This is a pointer (rather than embedded directly)
	// so that we don't embed a Mutex in this struct, which will make the
	// struct non-copyable, which might break some callers.
	state *serverInternalState
}

type Option struct {
	DisableKeepalive bool
	ReadTimeout      time.Duration
}

func (s *Server) initialConnRecvWindowSize() int32 {
	if s.MaxUploadBufferPerConnection > initialWindowSize {
		return s.MaxUploadBufferPerConnection
	}
	return 1 << 20
}

func (s *Server) initialStreamRecvWindowSize() int32 {
	if s.MaxUploadBufferPerStream > 0 {
		return s.MaxUploadBufferPerStream
	}
	return 1 << 20
}

func (s *Server) maxReadFrameSize() uint32 {
	if v := s.MaxReadFrameSize; v >= minMaxFrameSize && v <= maxFrameSize {
		return v
	}
	return defaultMaxReadFrameSize
}

func (s *Server) maxConcurrentStreams() uint32 {
	if v := s.MaxConcurrentStreams; v > 0 {
		return v
	}
	return defaultMaxStreams
}

// maxQueuedControlFrames is the maximum number of control frames like
// SETTINGS, PING and RST_STREAM that will be queued for writing before
// the connection is closed to prevent memory exhaustion attacks.
func (s *Server) maxQueuedControlFrames() int {
	// TODO: if anybody asks, add a Server field, and remember to define the
	// behavior of negative values.
	return maxQueuedControlFrames
}

type serverInternalState struct {
	mu          sync.Mutex
	activeConns map[*serverConn]struct{}
}

func (s *serverInternalState) registerConn(sc *serverConn) {
	if s == nil {
		return // if the Server was used without calling ConfigureServer
	}
	s.mu.Lock()
	s.activeConns[sc] = struct{}{}
	s.mu.Unlock()
}

func (s *serverInternalState) unregisterConn(sc *serverConn) {
	if s == nil {
		return // if the Server was used without calling ConfigureServer
	}
	s.mu.Lock()
	delete(s.activeConns, sc)
	s.mu.Unlock()
}

// Serve serves HTTP/2 requests on the provided connection and
// blocks until the connection is no longer readable.
//
// onData starts speaking HTTP/2 assuming that c has not had any
// reads or writes. It writes its initial settings frame and expects
// to be able to read the preface and settings frame from the
// client. If c has a ConnectionState method like a *tls.Conn, the
// ConnectionState is used to verify the TLS ciphersuite and to set
// the Request.TLS field in Handlers.
//
// onData does not support h2c by itself. Any h2c support must be
// implemented in terms of providing a suitably-behaving net.Conn.
//
// The opts parameter is optional. If nil, default values are used.
func (s *Server) Serve(ctx context.Context, c network.Conn) error {
	sc := &serverConn{
		srv:                         s,
		engine:                      &s.BaseEngine,
		conn:                        &h2Conn{c},
		baseCtx:                     ctx,
		remoteAddrStr:               c.RemoteAddr().String(),
		bw:                          newBufferedWriter(c),
		streams:                     make(map[uint32]*stream),
		readFrameCh:                 make(chan readFrameResult),
		wantWriteFrameCh:            make(chan FrameWriteRequest, 8),
		serveMsgCh:                  make(chan interface{}, 8),
		wroteFrameCh:                make(chan frameWriteResult, 1), // buffered; one send in writeFrameAsync
		bodyReadCh:                  make(chan bodyReadMsg),         // buffering doesn't matter either way
		doneServing:                 make(chan struct{}),
		clientMaxStreams:            math.MaxUint32, // Section 6.5.2: "Initially, there is no limit to this value"
		advMaxStreams:               s.maxConcurrentStreams(),
		initialStreamSendWindowSize: initialWindowSize,
		maxFrameSize:                initialMaxFrameSize,
		headerTableSize:             initialHeaderTableSize,
		serveG:                      newGoroutineLock(),
		pushEnabled:                 true,
	}
	s.state.registerConn(sc)
	defer s.state.unregisterConn(sc)

	// The net/http package sets the write deadline from the
	// http.Server.WriteTimeout during the TLS handshake, but then
	// passes the connection off to us with the deadline already set.
	// Write deadlines are set per stream in serverConn.newStream.
	// Disarm the net.Conn write deadline here.

	//if sc.engine.WriteTimeout != 0 {
	//	sc.conn.SetWriteDeadline(time.Time{})
	//}

	if s.NewWriteScheduler != nil {
		sc.writeSched = s.NewWriteScheduler()
	} else {
		sc.writeSched = NewRandomWriteScheduler()
	}

	// These start at the RFC-specified defaults. If there is a higher
	// configured value for inflow, that will be updated when we send a
	// WINDOW_UPDATE shortly after sending SETTINGS.
	sc.flow.add(initialWindowSize)
	sc.inflow.add(initialWindowSize)
	sc.hpackEncoder = hpack.NewEncoder(&sc.headerWriteBuf)

	fr := NewFramer(sc.bw, c)
	fr.ReadMetaHeaders = hpack.NewDecoder(initialHeaderTableSize, nil)
	fr.MaxHeaderListSize = sc.maxHeaderListSize()
	fr.SetMaxReadFrameSize(s.maxReadFrameSize())
	sc.framer = fr

	if tc, ok := c.(connectionStater); ok {
		sc.tlsState = new(tls.ConnectionState)
		*sc.tlsState = tc.ConnectionState()
		// 9.2 Use of TLS Features
		// An implementation of HTTP/2 over TLS MUST use TLS
		// 1.2 or higher with the restrictions on feature set
		// and cipher suite described in this section. Due to
		// implementation limitations, it might not be
		// possible to fail TLS negotiation. An endpoint MUST
		// immediately terminate an HTTP/2 connection that
		// does not meet the TLS requirements described in
		// this section with a connection error (Section
		// 5.4.1) of type INADEQUATE_SECURITY.
		if sc.tlsState.Version < tls.VersionTLS12 {
			sc.rejectConn(ErrCodeInadequateSecurity, "TLS version too low")
			return nil
		}

		// if sc.tlsState.ServerName == "" {
		// Client must use SNI, but we don't enforce that anymore,
		// since it was causing problems when connecting to bare IP
		// addresses during development.
		//
		// TODO: optionally enforce? Or enforce at the time we receive
		// a new request, and verify the ServerName matches the :authority?
		// But that precludes proxy situations, perhaps.
		//
		// So for now, do nothing here again.
		// }

		if !s.PermitProhibitedCipherSuites && isBadCipher(sc.tlsState.CipherSuite) {
			// "Endpoints MAY choose to generate a connection error
			// (Section 5.4.1) of type INADEQUATE_SECURITY if one of
			// the prohibited cipher suites are negotiated."
			//
			// We choose that. In my opinion, the spec is weak
			// here. It also says both parties must support at least
			// TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 so there's no
			// excuses here. If we really must, we could allow an
			// "AllowInsecureWeakCiphers" option on the server later.
			// Let's see how it plays out first.
			sc.rejectConn(ErrCodeInadequateSecurity, fmt.Sprintf("Prohibited TLS 1.2 Cipher Suite: %x", sc.tlsState.CipherSuite))
			return nil
		}
	}

	if hook := testHookGetServerConn; hook != nil {
		hook(sc)
	}
	sc.serve()
	return nil
}

func (sc *serverConn) rejectConn(err ErrCode, debug string) {
	if VerboseLogs {
		hlog.Infof("HERTZ: HTTP2 server rejecting conn: %v, %s", err, debug)
	}
	// ignoring errors. hanging up anyway.
	sc.framer.WriteGoAway(0, err, []byte(debug))
	sc.bw.Flush()
	sc.conn.Close()
}

type serverConn struct {
	// Immutable:
	srv              *Server
	conn             net.Conn
	bw               *bufferedWriter // writing to conn
	baseCtx          context.Context
	framer           *Framer
	doneServing      chan struct{}          // closed when serverConn.serve ends
	readFrameCh      chan readFrameResult   // written by serverConn.readFrames
	wantWriteFrameCh chan FrameWriteRequest // from handlers -> serve
	wroteFrameCh     chan frameWriteResult  // from writeFrameAsync -> serve, tickles more frame writes
	bodyReadCh       chan bodyReadMsg       // from handlers -> serve
	serveMsgCh       chan interface{}       // misc messages & code to send to / run on the serve loop
	flow             flow                   // conn-wide (not stream-specific) outbound flow control
	inflow           flow                   // conn-wide inbound flow control
	tlsState         *tls.ConnectionState   // shared by all handlers, like net/http
	remoteAddrStr    string
	writeSched       WriteScheduler

	// Everything following is owned by the serve loop; use serveG.check():
	serveG                      goroutineLock // used to verify funcs are on serve()
	pushEnabled                 bool
	sawFirstSettings            bool // got the initial SETTINGS frame after the preface
	needToSendSettingsAck       bool
	unackedSettings             int    // how many SETTINGS have we sent without ACKs?
	queuedControlFrames         int    // control frames in the writeSched queue
	clientMaxStreams            uint32 // SETTINGS_MAX_CONCURRENT_STREAMS from client (our PUSH_PROMISE limit)
	advMaxStreams               uint32 // our SETTINGS_MAX_CONCURRENT_STREAMS advertised the client
	curClientStreams            uint32 // number of open streams initiated by the client
	curPushedStreams            uint32 // number of open streams initiated by server push
	maxClientStreamID           uint32 // max ever seen from client (odd), or 0 if there have been no client requests
	maxPushPromiseID            uint32 // ID of the last push promise (even), or 0 if there have been no pushes
	streams                     map[uint32]*stream
	initialStreamSendWindowSize int32
	maxFrameSize                int32
	headerTableSize             uint32
	peerMaxHeaderListSize       uint32            // zero means unknown (default)
	canonHeader                 map[string]string // http2-lower-case -> Go-Canonical-Case
	writingFrame                bool              // started writing a frame (on serve goroutine or separate)
	writingFrameAsync           bool              // started a frame on its own goroutine but haven't heard back on wroteFrameCh
	needsFrameFlush             bool              // last frame write wasn't a flush
	inGoAway                    bool              // we've started to or sent GOAWAY
	inFrameScheduleLoop         bool              // whether we're in the scheduleFrameWrite loop
	needToSendGoAway            bool              // we need to schedule a GOAWAY frame write
	goAwayCode                  ErrCode
	shutdownTimer               *time.Timer // nil until used
	idleTimer                   *time.Timer // nil if unused

	// Owned by the writeFrameAsync goroutine:
	headerWriteBuf bytes.Buffer
	hpackEncoder   *hpack.Encoder

	// Used by startGracefulShutdown.
	shutdownOnce sync.Once

	engine *BaseEngine
}

func (b *BaseEngine) AcquireReqCtx() *app.RequestContext {
	return b.Core.GetCtxPool().Get().(*app.RequestContext)
}

func (b *BaseEngine) ReleaseReqCtx(ctx *app.RequestContext) {
	b.Core.GetCtxPool().Put(ctx)
}

func (sc *serverConn) maxHeaderListSize() uint32 {
	n := http.DefaultMaxHeaderBytes
	// http2's count is in a slightly different unit and includes 32 bytes per pair.
	// So, take the net/http.Server value and pad it up a bit, assuming 10 headers.
	const perFieldOverhead = 32 // per http2 spec
	const typicalHeaders = 10   // conservative
	return uint32(n + typicalHeaders*perFieldOverhead)
}

func (sc *serverConn) curOpenStreams() uint32 {
	sc.serveG.check()
	return sc.curClientStreams + sc.curPushedStreams
}

func (sc *serverConn) Framer() *Framer  { return sc.framer }
func (sc *serverConn) CloseConn() error { return sc.conn.Close() }
func (sc *serverConn) Flush() error     { return sc.bw.Flush() }
func (sc *serverConn) HeaderEncoder() (*hpack.Encoder, *bytes.Buffer) {
	return sc.hpackEncoder, &sc.headerWriteBuf
}

func (sc *serverConn) state(streamID uint32) (streamState, *stream) {
	sc.serveG.check()
	// http://tools.ietf.org/html/rfc7540#section-5.1
	if st, ok := sc.streams[streamID]; ok {
		return st.state, st
	}
	// "The first use of a new stream identifier implicitly closes all
	// streams in the "idle" state that might have been initiated by
	// that peer with a lower-valued stream identifier. For example, if
	// a client sends a HEADERS frame on stream 7 without ever sending a
	// frame on stream 5, then stream 5 transitions to the "closed"
	// state when the first frame for stream 7 is sent or received."
	if streamID%2 == 1 {
		if streamID <= sc.maxClientStreamID {
			return stateClosed, nil
		}
	} else {
		if streamID <= sc.maxPushPromiseID {
			return stateClosed, nil
		}
	}
	return stateIdle, nil
}

// errno returns v's underlying uintptr, else 0.
//
// TODO: remove this helper function once http2 can use build
// tags. See comment in isClosedConnError.
func errno(v error) uintptr {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Uintptr {
		return uintptr(rv.Uint())
	}
	return 0
}

// isClosedConnError reports whether err is an error from use of a closed
// network connection.
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}

	// TODO: remove this string search and be more like the Windows
	// case below. That might involve modifying the standard library
	// to return better error types.
	str := err.Error()
	if strings.Contains(str, "use of closed network connection") {
		return true
	}

	// TODO(bradfitz): x/tools/cmd/bundle doesn't really support
	// build tags, so I can't make an http2_windows.go file with
	// Windows-specific stuff. Fix that and move this, once we
	// have a way to bundle this into std's net/http somehow.
	if runtime.GOOS == "windows" {
		if oe, ok := err.(*net.OpError); ok && oe.Op == "read" {
			if se, ok := oe.Err.(*os.SyscallError); ok && se.Syscall == "wsarecv" {
				const WSAECONNABORTED = 10053
				const WSAECONNRESET = 10054
				if n := errno(se.Err); n == WSAECONNRESET || n == WSAECONNABORTED {
					return true
				}
			}
		}
	}
	return false
}

func (sc *serverConn) condlogf(err error, format string, args ...interface{}) {
	if err == nil {
		return
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF || isClosedConnError(err) || err == errPrefaceTimeout {
		// Boring, expected errors.
		if VerboseLogs {
			hlog.Infof(format, args...)
		}
	} else {
		hlog.Errorf(format, args...)
	}
}

func (sc *serverConn) canonicalHeader(v string) string {
	sc.serveG.check()
	buildCommonHeaderMapsOnce()
	cv, ok := commonCanonHeader[v]
	if ok {
		return cv
	}
	cv, ok = sc.canonHeader[v]
	if ok {
		return cv
	}
	if sc.canonHeader == nil {
		sc.canonHeader = make(map[string]string)
	}
	cv = http.CanonicalHeaderKey(v)
	sc.canonHeader[v] = cv
	return cv
}

type readFrameResult struct {
	f   Frame // valid until readMore is called
	err error

	// readMore should be called once the consumer no longer needs or
	// retains f. After readMore, f is invalid and more frames can be
	// read.
	readMore func()
}

// readFrames is the loop that reads incoming frames.
// It takes care to only read one frame at a time, blocking until the
// consumer is done with the frame.
// It's run on its own goroutine.
func (sc *serverConn) readFrames() {
	gate := make(gate)
	gateDone := gate.Done
	for {
		f, err := sc.framer.ReadFrame()
		select {
		case sc.readFrameCh <- readFrameResult{f, err, gateDone}:
		case <-sc.doneServing:
			return
		}
		select {
		case <-gate:
		case <-sc.doneServing:
			return
		}
		if terminalReadFrameError(err) {
			return
		}
	}
}

// frameWriteResult is the message passed from writeFrameAsync to the serve goroutine.
type frameWriteResult struct {
	wr  FrameWriteRequest // what was written (or attempted)
	err error             // result of the writeFrame call
}

// writeFrameAsync runs in its own goroutine and writes a single frame
// and then reports when it's done.
// At most one goroutine can be running writeFrameAsync at a time per
// serverConn.
func (sc *serverConn) writeFrameAsync(wr FrameWriteRequest) {
	err := wr.write.writeFrame(sc)
	sc.wroteFrameCh <- frameWriteResult{wr, err}
}

func (sc *serverConn) closeAllStreamsOnConnClose() {
	sc.serveG.check()
	for _, st := range sc.streams {
		sc.closeStream(st, errClientDisconnected)
	}
}

func (sc *serverConn) stopShutdownTimer() {
	sc.serveG.check()
	if t := sc.shutdownTimer; t != nil {
		t.Stop()
	}
}

func (sc *serverConn) notePanic() {
	// Note: this is for serverConn.serve panicking, not http.Handler code.
	if testHookOnPanicMu != nil {
		testHookOnPanicMu.Lock()
		defer testHookOnPanicMu.Unlock()
	}
	if testHookOnPanic != nil {
		if e := recover(); e != nil {
			if testHookOnPanic(sc, e) {
				panic(e)
			}
		}
	}
}

func (sc *serverConn) serve() {
	sc.serveG.check()
	defer sc.notePanic()
	defer sc.conn.Close()
	defer sc.closeAllStreamsOnConnClose()
	defer sc.stopShutdownTimer()
	defer close(sc.doneServing) // unblocks handlers trying to send

	if VerboseLogs {
		hlog.Infof("HERTZ: HTTP2 server connection from %v", sc.conn.RemoteAddr())
	}

	sc.writeFrame(FrameWriteRequest{
		write: writeSettings{
			{SettingMaxFrameSize, sc.srv.maxReadFrameSize()},
			{SettingMaxConcurrentStreams, sc.advMaxStreams},
			{SettingMaxHeaderListSize, sc.maxHeaderListSize()},
			{SettingInitialWindowSize, uint32(sc.srv.initialStreamRecvWindowSize())},
		},
	})
	sc.unackedSettings++

	// Each connection starts with initialWindowSize inflow tokens.
	// If a higher value is configured, we add more tokens.
	if diff := sc.srv.initialConnRecvWindowSize() - initialWindowSize; diff > 0 {
		sc.sendWindowUpdate(nil, int(diff))
	}

	if err := sc.readPreface(); err != nil {
		sc.condlogf(err, "HERTZ: HTTP2 server: error reading preface from client %v: %v", sc.conn.RemoteAddr(), err)
		return
	}

	if sc.srv.IdleTimeout != 0 {
		sc.idleTimer = time.AfterFunc(sc.srv.IdleTimeout, sc.onIdleTimer)
		defer sc.idleTimer.Stop()
	}

	go sc.readFrames() // closed by defer sc.conn.Close above

	settingsTimer := time.AfterFunc(firstSettingsTimeout, sc.onSettingsTimer)
	defer settingsTimer.Stop()

	loopNum := 0
	for {
		loopNum++
		select {
		case wr := <-sc.wantWriteFrameCh:
			if se, ok := wr.write.(StreamError); ok {
				sc.resetStream(se)
				break
			}
			sc.writeFrame(wr)
		case res := <-sc.wroteFrameCh:
			sc.wroteFrame(res)
		case res := <-sc.readFrameCh:
			// Process any written frames before reading new frames from the client since a
			// written frame could have triggered a new stream to be started.
			if sc.writingFrameAsync {
				select {
				case wroteRes := <-sc.wroteFrameCh:
					sc.wroteFrame(wroteRes)
				default:
				}
			}
			if !sc.processFrameFromReader(res) {
				return
			}
			res.readMore()
			if settingsTimer != nil {
				settingsTimer.Stop()
				settingsTimer = nil
			}
		case m := <-sc.bodyReadCh:
			sc.noteBodyRead(m.st, m.n)
		case msg := <-sc.serveMsgCh:
			switch v := msg.(type) {
			case func(int):
				v(loopNum) // for testing
			case *serverMessage:
				switch v {
				case settingsTimerMsg:
					hlog.Errorf("HERTZ: Timeout waiting for SETTINGS frames from %v", sc.conn.RemoteAddr())
					return
				case idleTimerMsg:
					if VerboseLogs {
						hlog.Infof("HERTZ: Connection is idle")
					}
					sc.goAway(ErrCodeNo)
				case shutdownTimerMsg:
					if VerboseLogs {
						hlog.Infof("HERTZ: GOAWAY close timer fired; closing conn from %v", sc.conn.RemoteAddr())
					}
					return
				case gracefulShutdownMsg:
					sc.startGracefulShutdownInternal()
				default:
					panic("unknown timer")
				}
			case *startPushRequest:
				sc.startPush(v)
			default:
				panic(fmt.Sprintf("unexpected type %T", v))
			}
		}

		// If the peer is causing us to generate a lot of control frames,
		// but not reading them from us, assume they are trying to make us
		// run out of memory.
		if sc.queuedControlFrames > sc.srv.maxQueuedControlFrames() {
			if VerboseLogs {
				hlog.Infof("HERTZ: HTTP2 too many control frames in send queue, closing connection")
			}
			return
		}

		// Start the shutdown timer after sending a GOAWAY. When sending GOAWAY
		// with no error code (graceful shutdown), don't start the timer until
		// all open streams have been completed.
		sentGoAway := sc.inGoAway && !sc.needToSendGoAway && !sc.writingFrame
		gracefulShutdownComplete := sc.goAwayCode == ErrCodeNo && sc.curOpenStreams() == 0
		if sentGoAway && sc.shutdownTimer == nil && (sc.goAwayCode != ErrCodeNo || gracefulShutdownComplete) {
			sc.shutDownIn(goAwayTimeout)
		}
	}
}

type serverMessage int

// Message values sent to serveMsgCh.
var (
	settingsTimerMsg    = new(serverMessage)
	idleTimerMsg        = new(serverMessage)
	shutdownTimerMsg    = new(serverMessage)
	gracefulShutdownMsg = new(serverMessage)
)

func (sc *serverConn) onSettingsTimer() { sc.sendServeMsg(settingsTimerMsg) }
func (sc *serverConn) onIdleTimer()     { sc.sendServeMsg(idleTimerMsg) }
func (sc *serverConn) onShutdownTimer() { sc.sendServeMsg(shutdownTimerMsg) }

func (sc *serverConn) sendServeMsg(msg interface{}) {
	sc.serveG.checkNotOn() // NOT
	select {
	case sc.serveMsgCh <- msg:
	case <-sc.doneServing:
	}
}

var errPrefaceTimeout = errors.New("timeout waiting for client preface")

// readPreface reads the ClientPreface greeting from the peer or
// returns errPrefaceTimeout on timeout, or an error if the greeting
// is invalid.
func (sc *serverConn) readPreface() error {
	errc := make(chan error, 1)
	go func() {
		// Read the client preface
		buf := make([]byte, len(consts.ClientPreface))
		if _, err := io.ReadFull(sc.conn, buf); err != nil {
			errc <- err
		} else if !bytes.Equal(buf, bytestr.StrClientPreface) {
			errc <- fmt.Errorf("bogus greeting %q", buf)
		} else {
			errc <- nil
		}
	}()
	timer := time.NewTimer(prefaceTimeout) // TODO: configurable on *Server?
	defer timer.Stop()
	select {
	case <-timer.C:
		return errPrefaceTimeout
	case err := <-errc:
		if err == nil {
			if VerboseLogs {
				hlog.Infof("HERTZ: HTTP2 server: client %v said hello", sc.conn.RemoteAddr())
			}
		}
		return err
	}
}

var errChanPool = sync.Pool{
	New: func() interface{} { return make(chan error, 1) },
}

var writeDataPool = sync.Pool{
	New: func() interface{} { return new(writeData) },
}

// writeDataFromHandler writes DATA response frames from a handler on
// the given stream.
func (sc *serverConn) writeDataFromHandler(stream *stream, data []byte, endStream bool) error {
	ch := errChanPool.Get().(chan error)
	writeArg := writeDataPool.Get().(*writeData)
	*writeArg = writeData{stream.id, data, endStream}
	err := sc.writeFrameFromHandler(FrameWriteRequest{
		write:  writeArg,
		stream: stream,
		done:   ch,
	})
	if err != nil {
		return err
	}
	var frameWriteDone bool // the frame write is done (successfully or not)
	select {
	case err = <-ch:
		frameWriteDone = true
	case <-sc.doneServing:
		return errClientDisconnected
	case <-stream.cw:
		// If both ch and stream.cw were ready (as might
		// happen on the final Write after an http.Handler
		// ends), prefer the write result. Otherwise this
		// might just be us successfully closing the stream.
		// The writeFrameAsync and serve goroutines guarantee
		// that the ch send will happen before the stream.cw
		// close.
		select {
		case err = <-ch:
			frameWriteDone = true
		default:
			return errStreamClosed
		}
	}
	errChanPool.Put(ch)
	if frameWriteDone {
		writeDataPool.Put(writeArg)
	}
	return err
}

// writeFrameFromHandler sends wr to sc.wantWriteFrameCh, but aborts
// if the connection has gone away.
//
// This must not be run from the serve goroutine itself, else it might
// deadlock writing to sc.wantWriteFrameCh (which is only mildly
// buffered and is read by serve itself). If you're on the serve
// goroutine, call writeFrame instead.
func (sc *serverConn) writeFrameFromHandler(wr FrameWriteRequest) error {
	sc.serveG.checkNotOn() // NOT
	select {
	case sc.wantWriteFrameCh <- wr:
		return nil
	case <-sc.doneServing:
		// Serve loop is gone.
		// Client has closed their connection to the server.
		return errClientDisconnected
	}
}

// writeFrame schedules a frame to write and sends it if there's nothing
// already being written.
//
// There is no pushback here (the serve goroutine never blocks). It's
// the http.Handlers that block, waiting for their previous frames to
// make it onto the wire
//
// If you're not on the serve goroutine, use writeFrameFromHandler instead.
func (sc *serverConn) writeFrame(wr FrameWriteRequest) {
	sc.serveG.check()

	// If true, wr will not be written and wr.done will not be signaled.
	var ignoreWrite bool

	// We are not allowed to write frames on closed streams. RFC 7540 Section
	// 5.1.1 says: "An endpoint MUST NOT send frames other than PRIORITY on
	// a closed stream." Our server never sends PRIORITY, so that exception
	// does not apply.
	//
	// The serverConn might close an open stream while the stream's handler
	// is still running. For example, the server might close a stream when it
	// receives bad data from the client. If this happens, the handler might
	// attempt to write a frame after the stream has been closed (since the
	// handler hasn't yet been notified of the close). In this case, we simply
	// ignore the frame. The handler will notice that the stream is closed when
	// it waits for the frame to be written.
	//
	// As an exception to this rule, we allow sending RST_STREAM after close.
	// This allows us to immediately reject new streams without tracking any
	// state for those streams (except for the queued RST_STREAM frame). This
	// may result in duplicate RST_STREAMs in some cases, but the client should
	// ignore those.
	if wr.StreamID() != 0 {
		_, isReset := wr.write.(StreamError)
		if state, _ := sc.state(wr.StreamID()); state == stateClosed && !isReset {
			ignoreWrite = true
		}
	}

	// Don't send a 100-continue response if we've already sent headers.
	// See golang.org/issue/14030.
	switch wr.write.(type) {
	case *writeResHeaders:
		wr.stream.wroteHeaders = true
	case write100ContinueHeadersFrame:
		if wr.stream.wroteHeaders {
			// We do not need to notify wr.done because this frame is
			// never written with wr.done != nil.
			if wr.done != nil {
				panic("wr.done != nil for write100ContinueHeadersFrame")
			}
			ignoreWrite = true
		}
	}

	if !ignoreWrite {
		if wr.isControl() {
			sc.queuedControlFrames++
			// For extra safety, detect wraparounds, which should not happen,
			// and pull the plug.
			if sc.queuedControlFrames < 0 {
				sc.conn.Close()
			}
		}
		sc.writeSched.Push(wr)
	}
	sc.scheduleFrameWrite()
}

// startFrameWrite starts a goroutine to write wr (in a separate
// goroutine since that might block on the network), and updates the
// serve goroutine's state about the world, updated from info in wr.
func (sc *serverConn) startFrameWrite(wr FrameWriteRequest) {
	sc.serveG.check()
	if sc.writingFrame {
		panic("internal error: can only be writing one frame at a time")
	}

	st := wr.stream
	if st != nil {
		switch st.state {
		case stateHalfClosedLocal:
			switch wr.write.(type) {
			case StreamError, handlerPanicRST, writeWindowUpdate:
				// RFC 7540 Section 5.1 allows sending RST_STREAM, PRIORITY, and WINDOW_UPDATE
				// in this state. (We never send PRIORITY from the server, so that is not checked.)
			default:
				panic(fmt.Sprintf("internal error: attempt to send frame on a half-closed-local stream: %v", wr))
			}
		case stateClosed:
			panic(fmt.Sprintf("internal error: attempt to send frame on a closed stream: %v", wr))
		}
	}
	if wpp, ok := wr.write.(*writePushPromise); ok {
		var err error
		wpp.promisedID, err = wpp.allocatePromisedID()
		if err != nil {
			sc.writingFrameAsync = false
			wr.replyToWriter(err)
			return
		}
	}

	sc.writingFrame = true
	sc.needsFrameFlush = true
	if wr.write.staysWithinBuffer(sc.bw.Available()) {
		sc.writingFrameAsync = false
		err := wr.write.writeFrame(sc)
		sc.wroteFrame(frameWriteResult{wr, err})
	} else {
		sc.writingFrameAsync = true
		go sc.writeFrameAsync(wr)
	}
}

// errHandlerPanicked is the error given to any callers blocked in a read from
// Request.Body when the main goroutine panics. Since most handlers read in the
// main ServeHTTP goroutine, this will show up rarely.
var errHandlerPanicked = errors.New("http2: handler panicked")

// wroteFrame is called on the serve goroutine with the result of
// whatever happened on writeFrameAsync.
func (sc *serverConn) wroteFrame(res frameWriteResult) {
	sc.serveG.check()
	if !sc.writingFrame {
		panic("internal error: expected to be already writing a frame")
	}
	sc.writingFrame = false
	sc.writingFrameAsync = false

	wr := res.wr

	if writeEndsStream(wr.write) {
		st := wr.stream
		if st == nil {
			panic("internal error: expecting non-nil stream")
		}
		switch st.state {
		case stateOpen:
			// Here we would go to stateHalfClosedLocal in
			// theory, but since our handler is done and
			// the net/http package provides no mechanism
			// for closing a ResponseWriter while still
			// reading data (see possible TODO at top of
			// this file), we go into closed state here
			// anyway, after telling the peer we're
			// hanging up on them. We'll transition to
			// stateClosed after the RST_STREAM frame is
			// written.
			st.state = stateHalfClosedLocal
			// Section 8.1: a server MAY request that the client abort
			// transmission of a request without error by sending a
			// RST_STREAM with an error code of NO_ERROR after sending
			// a complete response.
			sc.resetStream(streamError(st.id, ErrCodeNo))
		case stateHalfClosedRemote:
			sc.closeStream(st, errHandlerComplete)
		}
	} else {
		switch v := wr.write.(type) {
		case StreamError:
			// st may be unknown if the RST_STREAM was generated to reject bad input.
			if st, ok := sc.streams[v.StreamID]; ok {
				sc.closeStream(st, v)
			}
		case handlerPanicRST:
			sc.closeStream(wr.stream, errHandlerPanicked)
		}
	}

	// Reply (if requested) to unblock the ServeHTTP goroutine.
	wr.replyToWriter(res.err)

	sc.scheduleFrameWrite()
}

// scheduleFrameWrite tickles the frame writing scheduler.
//
// If a frame is already being written, nothing happens. This will be called again
// when the frame is done being written.
//
// If a frame isn't being written and we need to send one, the best frame
// to send is selected by writeSched.
//
// If a frame isn't being written and there's nothing else to send, we
// flush the write buffer.
func (sc *serverConn) scheduleFrameWrite() {
	sc.serveG.check()
	if sc.writingFrame || sc.inFrameScheduleLoop {
		return
	}
	sc.inFrameScheduleLoop = true
	for !sc.writingFrameAsync {
		if sc.needToSendGoAway {
			sc.needToSendGoAway = false
			sc.startFrameWrite(FrameWriteRequest{
				write: &writeGoAway{
					maxStreamID: sc.maxClientStreamID,
					code:        sc.goAwayCode,
				},
			})
			continue
		}
		if sc.needToSendSettingsAck {
			sc.needToSendSettingsAck = false
			sc.startFrameWrite(FrameWriteRequest{write: writeSettingsAck{}})
			continue
		}
		if !sc.inGoAway || sc.goAwayCode == ErrCodeNo {
			if wr, ok := sc.writeSched.Pop(); ok {
				if wr.isControl() {
					sc.queuedControlFrames--
				}
				sc.startFrameWrite(wr)
				continue
			}
		}
		if sc.needsFrameFlush {
			sc.startFrameWrite(FrameWriteRequest{write: flushFrameWriter{}})
			sc.needsFrameFlush = false // after startFrameWrite, since it sets this true
			continue
		}
		break
	}
	sc.inFrameScheduleLoop = false
}

// startGracefulShutdown gracefully shuts down a connection. This
// sends GOAWAY with ErrCodeNo to tell the client we're gracefully
// shutting down. The connection isn't closed until all current
// streams are done.
//
// startGracefulShutdown returns immediately; it does not wait until
// the connection has shut down.
func (sc *serverConn) startGracefulShutdown() {
	sc.serveG.checkNotOn() // NOT
	sc.shutdownOnce.Do(func() { sc.sendServeMsg(gracefulShutdownMsg) })
}

// After sending GOAWAY, the connection will close after goAwayTimeout.
// If we close the connection immediately after sending GOAWAY, there may
// be unsent data in our kernel receive buffer, which will cause the kernel
// to send a TCP RST on close() instead of a FIN. This RST will abort the
// connection immediately, whether or not the client had received the GOAWAY.
//
// Ideally we should delay for at least 1 RTT + epsilon so the client has
// a chance to read the GOAWAY and stop sending messages. Measuring RTT
// is hard, so we approximate with 1 second. See golang.org/issue/18701.
//
// This is a var so it can be shorter in tests, where all requests uses the
// loopback interface making the expected RTT very small.
//
// TODO: configurable?
var goAwayTimeout = 1 * time.Second

func (sc *serverConn) startGracefulShutdownInternal() {
	sc.goAway(ErrCodeNo)
}

func (sc *serverConn) goAway(code ErrCode) {
	sc.serveG.check()
	if sc.inGoAway {
		return
	}
	sc.inGoAway = true
	sc.needToSendGoAway = true
	sc.goAwayCode = code
	sc.scheduleFrameWrite()
}

func (sc *serverConn) shutDownIn(d time.Duration) {
	sc.serveG.check()
	sc.shutdownTimer = time.AfterFunc(d, sc.onShutdownTimer)
}

func (sc *serverConn) resetStream(se StreamError) {
	sc.serveG.check()
	sc.writeFrame(FrameWriteRequest{write: se})
	if st, ok := sc.streams[se.StreamID]; ok {
		st.resetQueued = true
	}
}

// processFrameFromReader processes the serve loop's read from readFrameCh from the
// frame-reading goroutine.
// processFrameFromReader returns whether the connection should be kept open.
func (sc *serverConn) processFrameFromReader(res readFrameResult) bool {
	sc.serveG.check()
	err := res.err
	if err != nil {
		if err == ErrFrameTooLarge {
			sc.goAway(ErrCodeFrameSize)
			return true // goAway will close the loop
		}
		clientGone := err == io.EOF || err == io.ErrUnexpectedEOF || isClosedConnError(err)
		if clientGone {
			// TODO: could we also get into this state if
			// the peer does a half close
			// (e.g. CloseWrite) because they're done
			// sending frames but they're still wanting
			// our open replies?  Investigate.
			// TODO: add CloseWrite to crypto/tls.Conn first
			// so we have a way to test this? I suppose
			// just for testing we could have a non-TLS mode.
			return false
		}
	} else {
		f := res.f
		if VerboseLogs {
			hlog.Infof("HERTZ: HTTP2 server: read frame %v", summarizeFrame(f))
		}
		err = sc.processFrame(f)
		if err == nil {
			return true
		}
	}

	switch ev := err.(type) {
	case StreamError:
		sc.resetStream(ev)
		return true
	case goAwayFlowError:
		sc.goAway(ErrCodeFlowControl)
		return true
	case ConnectionError:
		hlog.Errorf("HERTZ: HTTP2 server: connection error from %v: %v", sc.conn.RemoteAddr(), ev)
		sc.goAway(ErrCode(ev))
		return true // goAway will handle shutdown
	default:
		if res.err != nil {
			if VerboseLogs {
				hlog.Infof("HERTZ: HTTP2 server: closing client connection; error reading frame from client %s: %v", sc.conn.RemoteAddr(), err)
			}
		} else {
			hlog.Errorf("HERTZ: HTTP2 server: closing client connection: %v", err)
		}
		return false
	}
}

func (sc *serverConn) processFrame(f Frame) error {
	sc.serveG.check()

	// First frame received must be SETTINGS.
	if !sc.sawFirstSettings {
		if _, ok := f.(*SettingsFrame); !ok {
			return ConnectionError(ErrCodeProtocol)
		}
		sc.sawFirstSettings = true
	}

	switch f := f.(type) {
	case *SettingsFrame:
		return sc.processSettings(f)
	case *MetaHeadersFrame:
		return sc.processHeaders(f)
	case *WindowUpdateFrame:
		return sc.processWindowUpdate(f)
	case *PingFrame:
		return sc.processPing(f)
	case *DataFrame:
		return sc.processData(f)
	case *RSTStreamFrame:
		return sc.processResetStream(f)
	case *PriorityFrame:
		return sc.processPriority(f)
	case *GoAwayFrame:
		return sc.processGoAway(f)
	case *PushPromiseFrame:
		// A client cannot push. Thus, servers MUST treat the receipt of a PUSH_PROMISE
		// frame as a connection error (Section 5.4.1) of type PROTOCOL_ERROR.
		return ConnectionError(ErrCodeProtocol)
	default:
		if VerboseLogs {
			hlog.Infof("HERTZ: HTTP2 server: ignoring frame: %v", f.Header())
		}
		return nil
	}
}

func (sc *serverConn) processPing(f *PingFrame) error {
	sc.serveG.check()
	if f.IsAck() {
		// 6.7 PING: " An endpoint MUST NOT respond to PING frames
		// containing this flag."
		return nil
	}
	if f.StreamID != 0 {
		// "PING frames are not associated with any individual
		// stream. If a PING frame is received with a stream
		// identifier field value other than 0x0, the recipient MUST
		// respond with a connection error (Section 5.4.1) of type
		// PROTOCOL_ERROR."
		return ConnectionError(ErrCodeProtocol)
	}
	if sc.inGoAway && sc.goAwayCode != ErrCodeNo {
		return nil
	}
	sc.writeFrame(FrameWriteRequest{write: writePingAck{f}})
	return nil
}

func (sc *serverConn) processWindowUpdate(f *WindowUpdateFrame) error {
	sc.serveG.check()
	switch {
	case f.StreamID != 0: // stream-level flow control
		state, st := sc.state(f.StreamID)
		if state == stateIdle {
			// Section 5.1: "Receiving any frame other than HEADERS
			// or PRIORITY on a stream in this state MUST be
			// treated as a connection error (Section 5.4.1) of
			// type PROTOCOL_ERROR."
			return ConnectionError(ErrCodeProtocol)
		}
		if st == nil {
			// "WINDOW_UPDATE can be sent by a peer that has sent a
			// frame bearing the END_STREAM flag. This means that a
			// receiver could receive a WINDOW_UPDATE frame on a "half
			// closed (remote)" or "closed" stream. A receiver MUST
			// NOT treat this as an error, see Section 5.1."
			return nil
		}
		if !st.flow.add(int32(f.Increment)) {
			return streamError(f.StreamID, ErrCodeFlowControl)
		}
	default: // connection-level flow control
		if !sc.flow.add(int32(f.Increment)) {
			return goAwayFlowError{}
		}
	}
	sc.scheduleFrameWrite()
	return nil
}

func (sc *serverConn) processResetStream(f *RSTStreamFrame) error {
	sc.serveG.check()

	state, st := sc.state(f.StreamID)
	if state == stateIdle {
		// 6.4 "RST_STREAM frames MUST NOT be sent for a
		// stream in the "idle" state. If a RST_STREAM frame
		// identifying an idle stream is received, the
		// recipient MUST treat this as a connection error
		// (Section 5.4.1) of type PROTOCOL_ERROR.
		return ConnectionError(ErrCodeProtocol)
	}
	if st != nil {
		// st.cancelCtx()
		sc.closeStream(st, streamError(f.StreamID, f.ErrCode))
	}
	return nil
}

func (sc *serverConn) closeStream(st *stream, err error) {
	sc.serveG.check()
	if st.state == stateIdle || st.state == stateClosed {
		panic(fmt.Sprintf("invariant; can't close stream in state %v", st.state))
	}
	st.state = stateClosed
	if st.writeDeadline != nil {
		st.writeDeadline.Stop()
	}
	if st.isPushed() {
		sc.curPushedStreams--
	} else {
		sc.curClientStreams--
	}
	delete(sc.streams, st.id)
	if len(sc.streams) == 0 {
		if sc.srv.IdleTimeout != 0 {
			sc.idleTimer.Reset(sc.srv.IdleTimeout)
		}
		if h1ServerKeepAlivesDisabled(sc.engine) {
			sc.startGracefulShutdownInternal()
		}
	}
	if p := st.body; p != nil {
		// Return any buffered unread bytes worth of conn-level flow control.
		// See golang.org/issue/16481
		sc.sendWindowUpdate(nil, p.Len())
		p.CloseWithError(err)
	}
	st.cw.Close() // signals Handler's CloseNotifier, unblocks writes, etc
	sc.writeSched.CloseStream(st.id)
}

func (sc *serverConn) processSettings(f *SettingsFrame) error {
	sc.serveG.check()
	if f.IsAck() {
		sc.unackedSettings--
		if sc.unackedSettings < 0 {
			// Why is the peer ACKing settings we never sent?
			// The spec doesn't mention this case, but
			// hang up on them anyway.
			return ConnectionError(ErrCodeProtocol)
		}
		return nil
	}
	if f.NumSettings() > 100 || f.HasDuplicates() {
		// This isn't actually in the spec, but hang up on
		// suspiciously large settings frames or those with
		// duplicate entries.
		return ConnectionError(ErrCodeProtocol)
	}
	if err := f.ForeachSetting(sc.processSetting); err != nil {
		return err
	}
	// TODO: judging by RFC 7540, Section 6.5.3 each SETTINGS frame should be
	// acknowledged individually, even if multiple are received before the ACK.
	sc.needToSendSettingsAck = true
	sc.scheduleFrameWrite()
	return nil
}

func (sc *serverConn) processSetting(s Setting) error {
	sc.serveG.check()
	if err := s.Valid(); err != nil {
		return err
	}
	if VerboseLogs {
		hlog.Infof("HERTZ: HTTP2 server: processing setting %v", s)
	}
	switch s.ID {
	case SettingHeaderTableSize:
		sc.headerTableSize = s.Val
		sc.hpackEncoder.SetMaxDynamicTableSize(s.Val)
	case SettingEnablePush:
		sc.pushEnabled = s.Val != 0
	case SettingMaxConcurrentStreams:
		sc.clientMaxStreams = s.Val
	case SettingInitialWindowSize:
		return sc.processSettingInitialWindowSize(s.Val)
	case SettingMaxFrameSize:
		sc.maxFrameSize = int32(s.Val) // the maximum valid s.Val is < 2^31
	case SettingMaxHeaderListSize:
		sc.peerMaxHeaderListSize = s.Val
	default:
		// Unknown setting: "An endpoint that receives a SETTINGS
		// frame with any unknown or unsupported identifier MUST
		// ignore that setting."
		if VerboseLogs {
			hlog.Infof("HERTZ: HTTP2 server: ignoring unknown setting %v", s)
		}
	}
	return nil
}

func (sc *serverConn) processSettingInitialWindowSize(val uint32) error {
	sc.serveG.check()
	// Note: val already validated to be within range by
	// processSetting's Valid call.

	// "A SETTINGS frame can alter the initial flow control window
	// size for all current streams. When the value of
	// SETTINGS_INITIAL_WINDOW_SIZE changes, a receiver MUST
	// adjust the size of all stream flow control windows that it
	// maintains by the difference between the new value and the
	// old value."
	old := sc.initialStreamSendWindowSize
	sc.initialStreamSendWindowSize = int32(val)
	growth := int32(val) - old // may be negative
	for _, st := range sc.streams {
		if !st.flow.add(growth) {
			// 6.9.2 Initial Flow Control Window Size
			// "An endpoint MUST treat a change to
			// SETTINGS_INITIAL_WINDOW_SIZE that causes any flow
			// control window to exceed the maximum size as a
			// connection error (Section 5.4.1) of type
			// FLOW_CONTROL_ERROR."
			return ConnectionError(ErrCodeFlowControl)
		}
	}
	return nil
}

func (sc *serverConn) processData(f *DataFrame) error {
	sc.serveG.check()
	if sc.inGoAway && sc.goAwayCode != ErrCodeNo {
		return nil
	}
	data := f.Data()

	// "If a DATA frame is received whose stream is not in "open"
	// or "half closed (local)" state, the recipient MUST respond
	// with a stream error (Section 5.4.2) of type STREAM_CLOSED."
	id := f.Header().StreamID
	state, st := sc.state(id)
	if id == 0 || state == stateIdle {
		// Section 5.1: "Receiving any frame other than HEADERS
		// or PRIORITY on a stream in this state MUST be
		// treated as a connection error (Section 5.4.1) of
		// type PROTOCOL_ERROR."
		return ConnectionError(ErrCodeProtocol)
	}
	if st == nil || state != stateOpen || st.gotTrailerHeader || st.resetQueued {
		// This includes sending a RST_STREAM if the stream is
		// in stateHalfClosedLocal (which currently means that
		// the http.Handler returned, so it's done reading &
		// done writing). Try to stop the client from sending
		// more DATA.

		// But still enforce their connection-level flow control,
		// and return any flow control bytes since we're not going
		// to consume them.
		if sc.inflow.available() < int32(f.Length) {
			return streamError(id, ErrCodeFlowControl)
		}
		// Deduct the flow control from inflow, since we're
		// going to immediately add it back in
		// sendWindowUpdate, which also schedules sending the
		// frames.
		sc.inflow.take(int32(f.Length))
		sc.sendWindowUpdate(nil, int(f.Length)) // conn-level

		if st != nil && st.resetQueued {
			// Already have a stream error in flight. Don't send another.
			return nil
		}
		return streamError(id, ErrCodeStreamClosed)
	}
	if st.body == nil {
		panic("internal error: should have a body in this state")
	}

	// Sender sending more than they'd declared?
	if st.declBodyBytes != -1 && st.bodyBytes+int64(len(data)) > st.declBodyBytes {
		st.body.CloseWithError(fmt.Errorf("sender tried to send more than declared Content-Length of %d bytes", st.declBodyBytes))
		// RFC 7540, sec 8.1.2.6: A request or response is also malformed if the
		// value of a content-length header field does not equal the sum of the
		// DATA frame payload lengths that form the body.
		return streamError(id, ErrCodeProtocol)
	}
	if f.Length > 0 {
		// Check whether the client has flow control quota.
		if st.inflow.available() < int32(f.Length) {
			return streamError(id, ErrCodeFlowControl)
		}
		st.inflow.take(int32(f.Length))

		if len(data) > 0 {
			wrote, err := st.body.Write(data)
			if err != nil {
				return streamError(id, ErrCodeStreamClosed)
			}
			if wrote != len(data) {
				panic("internal error: bad Writer")
			}
			st.bodyBytes += int64(len(data))
		}

		// Return any padded flow control now, since we won't
		// refund it later on body reads.
		if pad := int32(f.Length) - int32(len(data)); pad > 0 {
			sc.sendWindowUpdate32(nil, pad)
			sc.sendWindowUpdate32(st, pad)
		}
	}
	if f.StreamEnded() {
		st.endStream()
	}
	return nil
}

func (sc *serverConn) processGoAway(f *GoAwayFrame) error {
	sc.serveG.check()
	if f.ErrCode != ErrCodeNo {
		hlog.Errorf("HERTZ: HTTP2 received GOAWAY %+v, starting graceful shutdown", f)
	} else {
		if VerboseLogs {
			hlog.Infof("HERTZ: HTTP2 received GOAWAY %+v, starting graceful shutdown", f)
		}
	}
	sc.startGracefulShutdownInternal()
	// http://tools.ietf.org/html/rfc7540#section-6.8
	// We should not create any new streams, which means we should disable push.
	sc.pushEnabled = false
	return nil
}

// isPushed reports whether the stream is server-initiated.
func (st *stream) isPushed() bool {
	return st.id%2 == 0
}

// endStream closes a Request.Body's pipe. It is called when a DATA
// frame says a request body is over (or after trailers).
func (st *stream) endStream() {
	sc := st.sc
	sc.serveG.check()

	if st.declBodyBytes != -1 && st.declBodyBytes != st.bodyBytes {
		st.body.CloseWithError(fmt.Errorf("request declared a Content-Length of %d but only wrote %d bytes",
			st.declBodyBytes, st.bodyBytes))
	} else {
		st.body.closeWithErrorAndCode(io.EOF, st.copyTrailersToHandlerRequest)
		st.body.CloseWithError(io.EOF)
	}
	st.state = stateHalfClosedRemote
}

func (sc *serverConn) processHeaders(f *MetaHeadersFrame) error {
	sc.serveG.check()
	id := f.StreamID
	if sc.inGoAway {
		// Ignore.
		return nil
	}
	// http://tools.ietf.org/html/rfc7540#section-5.1.1
	// Streams initiated by a client MUST use odd-numbered stream
	// identifiers. [...] An endpoint that receives an unexpected
	// stream identifier MUST respond with a connection error
	// (Section 5.4.1) of type PROTOCOL_ERROR.
	if id%2 != 1 {
		return ConnectionError(ErrCodeProtocol)
	}
	// A HEADERS frame can be used to create a new stream or
	// send a trailer for an open one. If we already have a stream
	// open, let it process its own HEADERS frame (trailers at this
	// point, if it's valid).
	if st := sc.streams[f.StreamID]; st != nil {
		if st.resetQueued {
			// We're sending RST_STREAM to close the stream, so don't bother
			// processing this frame.
			return nil
		}
		// RFC 7540, sec 5.1: If an endpoint receives additional frames, other than
		// WINDOW_UPDATE, PRIORITY, or RST_STREAM, for a stream that is in
		// this state, it MUST respond with a stream error (Section 5.4.2) of
		// type STREAM_CLOSED.
		if st.state == stateHalfClosedRemote {
			return streamError(id, ErrCodeStreamClosed)
		}
		return st.processTrailerHeaders(f)
	}

	// [...] The identifier of a newly established stream MUST be
	// numerically greater than all streams that the initiating
	// endpoint has opened or reserved. [...]  An endpoint that
	// receives an unexpected stream identifier MUST respond with
	// a connection error (Section 5.4.1) of type PROTOCOL_ERROR.
	if id <= sc.maxClientStreamID {
		return ConnectionError(ErrCodeProtocol)
	}
	sc.maxClientStreamID = id

	if sc.idleTimer != nil {
		sc.idleTimer.Stop()
	}

	// http://tools.ietf.org/html/rfc7540#section-5.1.2
	// [...] Endpoints MUST NOT exceed the limit set by their peer. An
	// endpoint that receives a HEADERS frame that causes their
	// advertised concurrent stream limit to be exceeded MUST treat
	// this as a stream error (Section 5.4.2) of type PROTOCOL_ERROR
	// or REFUSED_STREAM.
	if sc.curClientStreams+1 > sc.advMaxStreams {
		if sc.unackedSettings == 0 {
			// They should know better.
			return streamError(id, ErrCodeProtocol)
		}
		// Assume it's a network race, where they just haven't
		// received our last SETTINGS update. But actually
		// this can't happen yet, because we don't yet provide
		// a way for users to adjust server parameters at
		// runtime.
		return streamError(id, ErrCodeRefusedStream)
	}

	initialState := stateOpen
	if f.StreamEnded() {
		initialState = stateHalfClosedRemote
	}
	st := sc.newStream(id, 0, initialState)

	if f.HasPriority() {
		if err := checkPriority(f.StreamID, f.Priority); err != nil {
			return err
		}
		sc.writeSched.AdjustStream(st.id, f.Priority)
	}

	req := &st.reqCtx.Request

	rw, err := sc.newWriterAndRequest(st, f)
	st.rw = rw
	if err != nil {
		return err
	}

	st.body = req.BodyStream().(*requestBody).pipe

	st.handler = sc.engine.Core.ServeHTTP
	if f.Truncated {
		// Their header list was too long. Send a 431 error.
		st.handler = handleHeaderListTooLong
	} else if err := checkValidHTTP2RequestHeaders(&req.Header); err != nil {
		st.handler = new400Handler(err)
	}

	// The net/http package sets the read deadline from the
	// http.Server.ReadTimeout during the TLS handshake, but then
	// passes the connection off to us with the deadline already
	// set. Disarm it here after the request headers are read,
	// similar to how the http1 server works. Here it's
	// technically more like the http1 Server's ReadHeaderTimeout
	// (in Go 1.8), though. That's a more sane option anyway.
	if sc.engine.ReadTimeout != 0 {
		sc.conn.SetReadDeadline(time.Time{})
	}

	go sc.runHandler(rw, st.reqCtx, st.handler)
	return nil
}

func checkPriority(streamID uint32, p PriorityParam) error {
	if streamID == p.StreamDep {
		// Section 5.3.1: "A stream cannot depend on itself. An endpoint MUST treat
		// this as a stream error (Section 5.4.2) of type PROTOCOL_ERROR."
		// Section 5.3.3 says that a stream can depend on one of its dependencies,
		// so it's only self-dependencies that are forbidden.
		return streamError(streamID, ErrCodeProtocol)
	}
	return nil
}

func (sc *serverConn) processPriority(f *PriorityFrame) error {
	if sc.inGoAway {
		return nil
	}
	if err := checkPriority(f.StreamID, f.PriorityParam); err != nil {
		return err
	}
	sc.writeSched.AdjustStream(f.StreamID, f.PriorityParam)
	return nil
}

func (sc *serverConn) newStream(id, pusherID uint32, state streamState) *stream {
	sc.serveG.check()
	if id == 0 {
		panic("internal error: cannot create stream with id 0")
	}

	reqCtx := sc.engine.AcquireReqCtx()
	if connection, ok := sc.conn.(network.Conn); ok {
		reqCtx.SetConn(connection)
	}
	reqCtx.Request.Header.SetProtocol(consts.HTTP20)
	reqCtx.Request.Header.InitContentLengthWithValue(-1)
	baseCtx, cancel := context.WithCancel(context.Background())

	st := &stream{
		sc:        sc,
		id:        id,
		state:     state,
		reqCtx:    reqCtx,
		cancelCtx: cancel,
		baseCtx:   baseCtx,
	}
	st.cw.Init()
	st.flow.conn = &sc.flow // link to conn-level counter
	st.flow.add(sc.initialStreamSendWindowSize)
	st.inflow.conn = &sc.inflow // link to conn-level counter
	st.inflow.add(sc.srv.initialStreamRecvWindowSize())

	// FIXME writeDeadline
	// 现在Server端不支持WriteTimeout
	//if sc.engine.WriteTimeout != 0 {
	//	st.writeDeadline = time.AfterFunc(sc.engine.WriteTimeout, st.onWriteTimeout)
	//}

	sc.streams[id] = st
	sc.writeSched.OpenStream(st.id, OpenStreamOptions{PusherID: pusherID})
	if st.isPushed() {
		sc.curPushedStreams++
	} else {
		sc.curClientStreams++
	}

	return st
}

func (sc *serverConn) newWriterAndRequest(st *stream, f *MetaHeadersFrame) (*responseWriter, error) {
	sc.serveG.check()

	rp := requestParam{
		method:    f.PseudoValue("method"),
		scheme:    f.PseudoValue("scheme"),
		authority: f.PseudoValue("authority"),
		path:      f.PseudoValue("path"),
	}

	isConnect := rp.method == "CONNECT"
	if isConnect {
		if rp.path != "" || rp.scheme != "" || rp.authority == "" {
			return nil, streamError(f.StreamID, ErrCodeProtocol)
		}
	} else if rp.method == "" || rp.path == "" || (rp.scheme != "https" && rp.scheme != "http") {
		// See 8.1.2.6 Malformed Requests and Responses:
		//
		// Malformed requests or responses that are detected
		// MUST be treated as a stream error (Section 5.4.2)
		// of type PROTOCOL_ERROR."
		//
		// 8.1.2.3 Request Pseudo-Header Fields
		// "All HTTP/2 requests MUST include exactly one valid
		// value for the :method, :scheme, and :path
		// pseudo-header fields"
		return nil, streamError(f.StreamID, ErrCodeProtocol)
	}

	bodyOpen := !f.StreamEnded()
	if rp.method == "HEAD" && bodyOpen {
		// HEAD requests can't have bodies
		return nil, streamError(f.StreamID, ErrCodeProtocol)
	}

	reqCtx := st.reqCtx
	reqCtx.Request.SetMethod(rp.method)
	var strBuilder strings.Builder

	//  `${rp.scheme}://${rp.authority}${rp.path}`
	strBuilder.Grow(len(rp.scheme) + len(rp.authority) + len(rp.path) + 3)

	strBuilder.WriteString(rp.scheme)
	strBuilder.WriteString("://")
	strBuilder.WriteString(rp.authority)
	strBuilder.WriteString(rp.path)
	reqCtx.Request.SetRequestURI(strBuilder.String())

	for _, hf := range f.RegularFields() {
		key := sc.canonicalHeader(hf.Name)
		reqCtx.Request.Header.SetCanonical(bytesconv.S2b(key), bytesconv.S2b(hf.Value))
	}

	if len(reqCtx.Request.Header.ContentLengthBytes()) > 0 {
		st.declBodyBytes = int64(reqCtx.Request.Header.ContentLength())
	} else {
		st.declBodyBytes = -1
	}

	rw, err := sc.newWriterAndRequestNoBody(st)
	if err != nil {
		return nil, err
	}

	if bodyOpen {
		reqCtx.Request.BodyStream().(*requestBody).pipe = &pipe{
			b: newDataBuffer(bytebufferpool.Get()),
		}
	}

	return rw, nil
}

type requestParam struct {
	method                  string
	scheme, authority, path string
}

func (sc *serverConn) newWriterAndRequestNoBody(st *stream) (*responseWriter, error) {
	reqCtx := st.reqCtx
	sc.serveG.check()

	needsContinue := bytes.Equal(reqCtx.Request.Header.Peek(consts.HeaderExpect), bytestr.Str100Continue)
	if needsContinue {
		reqCtx.Request.Header.DelBytes(bytestr.StrExpect)
	}

	// TODO Trailer
	// Setup Trailers
	//var trailer http.Header
	//for _, v := range rp.header["Trailer"] {
	//	for _, key := range strings.Split(v, ",") {
	//		key = http.CanonicalHeaderKey(strings.TrimSpace(key))
	//		switch key {
	//		case "Transfer-Encoding", "Trailer", "Content-Length":
	//			// Bogus. (copy of http1 rules)
	//			// Ignore.
	//		default:
	//			if trailer == nil {
	//				trailer = make(http.Header)
	//			}
	//			trailer[key] = nil
	//		}
	//	}
	//}
	//delete(rp.header, "Trailer")

	body := &requestBody{
		conn:          sc,
		stream:        st,
		needsContinue: needsContinue,
	}

	reqCtx.Request.ConstructBodyStream(nil, body)

	rws := responseWriterStatePool.Get().(*responseWriterState)
	bwSave := rws.bw
	*rws = responseWriterState{} // zero all the fields
	rws.conn = sc
	rws.bw = bwSave
	rws.bw.Reset(chunkWriter{rws})
	rws.stream = st
	rws.body = body

	rw := &responseWriter{rws: rws}
	return rw, nil
}

// Run on its own goroutine.
func (sc *serverConn) runHandler(rw *responseWriter, reqCtx *app.RequestContext, handler app.HandlerFunc) {
	didPanic := true
	defer func() {
		rw.rws.stream.cancelCtx()
		if didPanic {
			e := recover()
			sc.writeFrameFromHandler(FrameWriteRequest{
				write:  handlerPanicRST{rw.rws.stream.id},
				stream: rw.rws.stream,
			})
			// Same as net/http:
			if e != nil && e != http.ErrAbortHandler {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				hlog.Errorf("HERTZ: HTTP2 panic serving %v: %v\n%s", sc.conn.RemoteAddr(), e, buf)
			}
			return
		} else {
			rw.WriteHeader(reqCtx.Response.StatusCode())
			step := 1 << 14 // max frame size 16384
			body := reqCtx.Response.Body()
			bodyLen := len(body)
			for i := 0; i < bodyLen; i += step {
				end := i + step
				if end > bodyLen {
					end = bodyLen
				}
				rw.Write(body[i:end])
			}
		}
		rw.handlerDone()
	}()
	handler(rw.rws.stream.baseCtx, reqCtx)

	didPanic = false
}

func handleHeaderListTooLong(c context.Context, reqCtx *app.RequestContext) {
	// 10.5.1 Limits on Header Block Size:
	// .. "A server that receives a larger header block than it is
	// willing to handle can send an HTTP 431 (Request Header Fields Too
	// Large) status code"
	const statusRequestHeaderFieldsTooLarge = 431 // only in Go 1.6+
	reqCtx.AbortWithMsg("<h1>HTTP Error 431</h1><p>Request Header Field(s) Too Large</p>", statusRequestHeaderFieldsTooLarge)
}

// called from handler goroutines.
// h may be nil.
func (sc *serverConn) writeHeaders(st *stream, headerData *writeResHeaders) error {
	sc.serveG.checkNotOn() // NOT on
	var errc chan error
	if headerData.h != nil {
		// If there's a header map (which we don't own), so we have to block on
		// waiting for this frame to be written, so an http.Flush mid-handler
		// writes out the correct value of keys, before a handler later potentially
		// mutates it.
		errc = errChanPool.Get().(chan error)
	}
	if err := sc.writeFrameFromHandler(FrameWriteRequest{
		write:  headerData,
		stream: st,
		done:   errc,
	}); err != nil {
		return err
	}
	if errc != nil {
		select {
		case err := <-errc:
			errChanPool.Put(errc)
			return err
		case <-sc.doneServing:
			return errClientDisconnected
		case <-st.cw:
			return errStreamClosed
		}
	}
	return nil
}

// called from handler goroutines.
func (sc *serverConn) write100ContinueHeaders(st *stream) {
	sc.writeFrameFromHandler(FrameWriteRequest{
		write:  write100ContinueHeadersFrame{st.id},
		stream: st,
	})
}

// A bodyReadMsg tells the server loop that the http.Handler read n
// bytes of the DATA from the client on the given stream.
type bodyReadMsg struct {
	st *stream
	n  int
}

// called from handler goroutines.
// Notes that the handler for the given stream ID read n bytes of its body
// and schedules flow control tokens to be sent.
func (sc *serverConn) noteBodyReadFromHandler(st *stream, n int, err error) {
	sc.serveG.checkNotOn() // NOT on
	if n > 0 {
		select {
		case sc.bodyReadCh <- bodyReadMsg{st, n}:
		case <-sc.doneServing:
		}
	}
}

func (sc *serverConn) noteBodyRead(st *stream, n int) {
	sc.serveG.check()
	sc.sendWindowUpdate(nil, n) // conn-level
	if st.state != stateHalfClosedRemote && st.state != stateClosed {
		// Don't send this WINDOW_UPDATE if the stream is closed
		// remotely.
		sc.sendWindowUpdate(st, n)
	}
}

// st may be nil for conn-level
func (sc *serverConn) sendWindowUpdate(st *stream, n int) {
	sc.serveG.check()
	// "The legal range for the increment to the flow control
	// window is 1 to 2^31-1 (2,147,483,647) octets."
	// A Go Read call on 64-bit machines could in theory read
	// a larger Read than this. Very unlikely, but we handle it here
	// rather than elsewhere for now.
	const maxUint31 = 1<<31 - 1
	for n >= maxUint31 {
		sc.sendWindowUpdate32(st, maxUint31)
		n -= maxUint31
	}
	sc.sendWindowUpdate32(st, int32(n))
}

// st may be nil for conn-level
func (sc *serverConn) sendWindowUpdate32(st *stream, n int32) {
	sc.serveG.check()
	if n == 0 {
		return
	}
	if n < 0 {
		panic("negative update")
	}
	var streamID uint32
	if st != nil {
		streamID = st.id
	}
	sc.writeFrame(FrameWriteRequest{
		write:  writeWindowUpdate{streamID: streamID, n: uint32(n)},
		stream: st,
	})
	var ok bool
	if st == nil {
		ok = sc.inflow.add(n)
	} else {
		ok = st.inflow.add(n)
	}
	if !ok {
		panic("internal error; sent too many window updates without decrements?")
	}
}

// requestBody is the Handler's Request.Body type.
// Read and Close may be called concurrently.
type requestBody struct {
	stream        *stream
	conn          *serverConn
	closed        bool  // for use by Close only
	sawEOF        bool  // for use by Read only
	pipe          *pipe // non-nil if we have a HTTP entity message body
	needsContinue bool  // need to send a 100-continue
}

func (b *requestBody) Close() error {
	if b.pipe != nil && !b.closed {
		b.pipe.BreakWithError(errClosedBody)
	}
	b.closed = true
	return nil
}

func (b *requestBody) Read(p []byte) (n int, err error) {
	if b.needsContinue {
		b.needsContinue = false
		b.conn.write100ContinueHeaders(b.stream)
	}
	if b.pipe == nil || b.sawEOF {
		return 0, io.EOF
	}
	n, err = b.pipe.Read(p)
	if err == io.EOF {
		b.sawEOF = true
	}
	if b.conn == nil && inTests {
		return
	}
	b.conn.noteBodyReadFromHandler(b.stream, n, err)
	return
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

// Push errors.
var (
	ErrRecursivePush    = errors.New("http2: recursive push not allowed")
	ErrPushLimitReached = errors.New("http2: push would exceed peer's SETTINGS_MAX_CONCURRENT_STREAMS")
)

var _ http.Pusher = (*responseWriter)(nil)

type startPushRequest struct {
	parent *stream
	method string
	url    *url.URL
	header http.Header
	done   chan error
}

func (sc *serverConn) startPush(msg *startPushRequest) {
	sc.serveG.check()

	// http://tools.ietf.org/html/rfc7540#section-6.6.
	// PUSH_PROMISE frames MUST only be sent on a peer-initiated stream that
	// is in either the "open" or "half-closed (remote)" state.
	if msg.parent.state != stateOpen && msg.parent.state != stateHalfClosedRemote {
		// responseWriter.Push checks that the stream is peer-initiated.
		msg.done <- errStreamClosed
		return
	}

	// http://tools.ietf.org/html/rfc7540#section-6.6.
	if !sc.pushEnabled {
		msg.done <- http.ErrNotSupported
		return
	}

	// PUSH_PROMISE frames must be sent in increasing order by stream ID, so
	// we allocate an ID for the promised stream lazily, when the PUSH_PROMISE
	// is written. Once the ID is allocated, we start the request handler.
	allocatePromisedID := func() (uint32, error) {
		sc.serveG.check()

		// Check this again, just in case. Technically, we might have received
		// an updated SETTINGS by the time we got around to writing this frame.
		if !sc.pushEnabled {
			return 0, http.ErrNotSupported
		}
		// http://tools.ietf.org/html/rfc7540#section-6.5.2.
		if sc.curPushedStreams+1 > sc.clientMaxStreams {
			return 0, ErrPushLimitReached
		}

		// http://tools.ietf.org/html/rfc7540#section-5.1.1.
		// Streams initiated by the server MUST use even-numbered identifiers.
		// A server that is unable to establish a new stream identifier can send a GOAWAY
		// frame so that the client is forced to open a new connection for new streams.
		if sc.maxPushPromiseID+2 >= 1<<31 {
			sc.startGracefulShutdownInternal()
			return 0, ErrPushLimitReached
		}
		sc.maxPushPromiseID += 2
		promisedID := sc.maxPushPromiseID

		// http://tools.ietf.org/html/rfc7540#section-8.2.
		// Strictly speaking, the new stream should start in "reserved (local)", then
		// transition to "half closed (remote)" after sending the initial HEADERS, but
		// we start in "half closed (remote)" for simplicity.
		// See further comments at the definition of stateHalfClosedRemote.
		promised := sc.newStream(promisedID, msg.parent.id, stateHalfClosedRemote)
		rw, err := sc.newWriterAndRequestNoBody(promised)
		if err != nil {
			// Should not happen, since we've already validated msg.url.
			panic(fmt.Sprintf("newWriterAndRequestNoBody(%+v): %v", msg.url, err))
		}

		go sc.runHandler(rw, promised.reqCtx, sc.engine.Core.ServeHTTP)
		return promisedID, nil
	}

	sc.writeFrame(FrameWriteRequest{
		write: &writePushPromise{
			streamID:           msg.parent.id,
			method:             msg.method,
			url:                msg.url,
			h:                  msg.header,
			allocatePromisedID: allocatePromisedID,
		},
		stream: msg.parent,
		done:   msg.done,
	})
}

// From http://httpwg.org/specs/rfc7540.html#rfc.section.8.1.2.2
var connHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Connection",
	"Transfer-Encoding",
	"Upgrade",
}

// checkValidHTTP2RequestHeaders checks whether h is a valid HTTP/2 request,
// per RFC 7540 Section 8.1.2.2.
// The returned error is reported to users.
func checkValidHTTP2RequestHeaders(h *protocol.RequestHeader) error {
	for _, k := range connHeaders {
		if len(h.Peek(k)) > 0 {
			return fmt.Errorf("request header %q is not valid in HTTP/2", k)
		}
	}

	// FIXME: 现在Hertz不支持Trailer，先不管
	//if len(te) > 0 && (len(te) > 1 || (te[0] != "trailers" && te[0] != "")) {
	//	return errors.New(`request header "TE" may only be "trailers" in HTTP/2`)
	//}
	return nil
}

func new400Handler(err error) app.HandlerFunc {
	return func(c context.Context, reqCtx *app.RequestContext) {
		reqCtx.AbortWithMsg(err.Error(), http.StatusBadRequest)
	}
}

// h1ServerKeepAlivesDisabled reports whether engine has its keep-alives
// disabled. See comments on h1ServerShutdownChan above for why
// the code is written this way.
func h1ServerKeepAlivesDisabled(hs *BaseEngine) bool {
	return hs.DisableKeepalive || !hs.Core.IsRunning()
}
