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

package http2

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// responseWriter is the http.ResponseWriter implementation. It's
// intentionally small (1 pointer wide) to minimize garbage. The
// responseWriterState pointer inside is zeroed at the end of a
// request (in handlerDone) and calls on the responseWriter thereafter
// simply crash (caller's mistake), but the much larger responseWriterState
// and buffers are reused between multiple requests.
type responseWriter struct {
	rws *responseWriterState
}

// Optional http.ResponseWriter interfaces implemented.
var (
	_ http.Flusher = (*responseWriter)(nil)
	_ stringWriter = (*responseWriter)(nil)
)

type responseWriterState struct {
	// immutable within a request:
	stream *stream
	body   *requestBody // to close at end of request, if DATA frames didn't
	conn   *serverConn

	// TODO: adjust buffer writing sizes based on server config, frame size updates from peer, etc
	bw *bufio.Writer // writing to a chunkWriter{this *responseWriterState}

	trailers    []string // set in writeChunk
	status      int      // status code passed to WriteHeaders
	wroteHeader bool     // WriteHeaders called (explicitly or implicitly). Not necessarily sent to user yet.
	sentHeader  bool     // have we sent the header frame?
	handlerDone bool     // handler has finished
	dirty       bool     // a Write failed; don't reuse this responseWriterState

	sentContentLen int64 // non-zero if handler set a Content-Length header
	wroteBytes     int64

	closeNotifierMu sync.Mutex // guards closeNotifierCh
	closeNotifierCh chan bool  // nil until first used
}

type chunkWriter struct{ rws *responseWriterState }

func (cw chunkWriter) Write(p []byte) (n int, err error) { return cw.rws.writeChunk(p) }

func (rws *responseWriterState) hasTrailers() bool { return len(rws.trailers) > 0 }

func (rws *responseWriterState) hasNonemptyTrailers() bool {
	// TODO Not support trailers yet

	// for _, trailer := range rws.trailers {
	// 	if _, ok := rws.handlerHeader[trailer]; ok {
	// 		return true
	// 	}
	// }
	return false
}

// writeChunk writes chunks from the bufio.Writer. But because
// bufio.Writer may bypass its chunking, sometimes p may be
// arbitrarily large.
//
// writeChunk is also responsible (on the first chunk) for sending the
// HEADER response.
func (rws *responseWriterState) writeChunk(p []byte) (n int, err error) {
	if !rws.wroteHeader {
		rws.writeHeader(200)
	}

	reqCtx := rws.stream.reqCtx
	isHeadResp := bytes.Equal(reqCtx.Method(), bytestr.StrHead)
	if !rws.sentHeader {
		header := &rws.stream.reqCtx.Response.Header
		rws.sentHeader = true

		var clen string
		if clen = string(header.ContentLengthBytes()); clen != "" {
			if cl, err := strconv.ParseUint(clen, 10, 63); err == nil {
				rws.sentContentLen = int64(cl)
			} else {
				clen = ""
			}
		}

		if clen == "" && rws.handlerDone && bodyAllowedForStatus(rws.status) && (len(p) > 0 || !isHeadResp) {
			clen = strconv.Itoa(len(p))
		}
		ctype := header.ContentType()
		// If the Content-Encoding is non-blank, we shouldn't
		// sniff the body. See Issue golang.org/issue/31753.

		ce := header.Peek(consts.HeaderContentEncoding)

		if len(ce) == 0 && len(ctype) == 0 && bodyAllowedForStatus(rws.status) && len(p) > 0 {
			ctype = bytesconv.S2b(http.DetectContentType(p))
		}

		date := string(header.Peek(consts.HeaderDate))
		if len(date) == 0 {
			protocol.ServerDateOnce.Do(protocol.UpdateServerDate)
			date = string(protocol.ServerDate.Load().([]byte))
		} else {
			header.Del(consts.HeaderDate)
		}

		// "Connection" headers aren't allowed in HTTP/2 (RFC 7540, 8.1.2.2),
		// but respect "Connection" == "close" to mean sending a GOAWAY and tearing
		// down the TCP connection when idle, like we do for HTTP/1.
		// TODO: remove more Connection-specific header fields here, in addition
		// to "Connection".
		connection := header.Peek("Connection")
		if len(connection) > 0 {
			if bytes.Equal(connection, bytestr.StrClose) {
				rws.conn.startGracefulShutdown()
			}
			header.Del("Connection")
		}
		endStream := (rws.handlerDone && !rws.hasTrailers() && len(p) == 0) || isHeadResp
		err = rws.conn.writeHeaders(rws.stream, &writeResHeaders{
			streamID:      rws.stream.id,
			httpResCode:   rws.status,
			h:             header,
			endStream:     endStream,
			contentType:   string(ctype),
			contentLength: clen,
			date:          date,
		})
		if err != nil {
			rws.dirty = true
			return 0, err
		}
		if endStream {
			return 0, nil
		}
	}
	if isHeadResp {
		return len(p), nil
	}
	if len(p) == 0 && !rws.handlerDone {
		return 0, nil
	}

	// only send trailers if they have actually been defined by the
	// server handler.
	hasNonemptyTrailers := rws.hasNonemptyTrailers()
	endStream := rws.handlerDone && !hasNonemptyTrailers
	if len(p) > 0 || endStream {
		// only send a 0 byte DATA frame if we're ending the stream.
		if err := rws.conn.writeDataFromHandler(rws.stream, p, endStream); err != nil {
			rws.dirty = true
			return 0, err
		}
	}

	// if rws.handlerDone && hasNonemptyTrailers {
	// TODO send trailers
	// }
	return len(p), nil
}

func (w *responseWriter) Flush() {
	rws := w.rws
	if rws == nil {
		panic("Header called after Handler finished")
	}
	if rws.bw.Buffered() > 0 {
		if err := rws.bw.Flush(); err != nil {
			// Ignore the error. The frame writer already knows.
			return
		}
	} else {
		// The bufio.Writer won't call chunkWriter.Write
		// (writeChunk with zero bytes, so we have to do it
		// ourselves to force the HTTP response header and/or
		// final DATA frame (with END_STREAM) to be sent.
		rws.writeChunk(nil)
	}
}

func (w *responseWriter) CloseNotify() <-chan bool {
	rws := w.rws
	if rws == nil {
		panic("CloseNotify called after Handler finished")
	}
	rws.closeNotifierMu.Lock()
	ch := rws.closeNotifierCh
	if ch == nil {
		ch = make(chan bool, 1)
		rws.closeNotifierCh = ch
		cw := rws.stream.cw
		go func() {
			cw.Wait() // wait for close
			ch <- true
		}()
	}
	rws.closeNotifierMu.Unlock()
	return ch
}

func (w *responseWriter) Header() *protocol.ResponseHeader {
	rws := w.rws
	if rws == nil {
		panic("Header called after Handler finished")
	}
	return &rws.body.stream.reqCtx.Response.Header
}

// checkWriteHeaderCode is a copy of net/http's checkWriteHeaderCode.
func checkWriteHeaderCode(code int) {
	// Issue 22880: require valid WriteHeaders status codes.
	// For now we only enforce that it's three digits.
	// In the future we might block things over 599 (600 and above aren't defined
	// at http://httpwg.org/specs/rfc7231.html#status.codes)
	// and we might block under 200 (once we have more mature 1xx support).
	// But for now any three digits.
	//
	// We used to send "HTTP/1.1 000 0" on the wire in responses but there's
	// no equivalent bogus thing we can realistically send in HTTP/2,
	// so we'll consistently panic instead and help people find their bugs
	// early. (We can't return an error from WriteHeaders even if we wanted to.)
	if code < 100 || code > 999 {
		panic(fmt.Sprintf("invalid WriteHeaders code %v", code))
	}
}

func (w *responseWriter) WriteHeader(code int) {
	rws := w.rws
	if rws == nil {
		panic("WriteHeaders called after Handler finished")
	}
	rws.writeHeader(code)
}

func (rws *responseWriterState) writeHeader(code int) {
	if !rws.wroteHeader {
		checkWriteHeaderCode(code)
		rws.wroteHeader = true
		rws.status = code
	}
}

// The Life Of A Write is like this:
//
// * Handler calls w.Write or w.WriteString ->
// * -> rws.bw (*bufio.Writer) ->
// * (Handler might call Flush)
// * -> chunkWriter{rws}
// * -> responseWriterState.writeChunk(p []byte)
// * -> responseWriterState.writeChunk (most of the magic; see comment there)
func (w *responseWriter) Write(p []byte) (n int, err error) {
	return w.write(len(p), p, "")
}

func (w *responseWriter) WriteString(s string) (n int, err error) {
	return w.write(len(s), nil, s)
}

// either dataB or dataS is non-zero.
func (w *responseWriter) write(lenData int, dataB []byte, dataS string) (n int, err error) {
	rws := w.rws
	if rws == nil {
		panic("Write called after Handler finished")
	}
	if !rws.wroteHeader {
		w.WriteHeader(200)
	}
	if !bodyAllowedForStatus(rws.status) {
		return 0, http.ErrBodyNotAllowed
	}
	rws.wroteBytes += int64(len(dataB)) + int64(len(dataS)) // only one can be set
	if rws.sentContentLen != 0 && rws.wroteBytes > rws.sentContentLen {
		// TODO: send a RST_STREAM
		return 0, errors.New("http2: handler wrote more than declared Content-Length")
	}

	if dataB != nil {
		return rws.bw.Write(dataB)
	} else {
		return rws.bw.WriteString(dataS)
	}
}

func (w *responseWriter) handlerDone() {
	rws := w.rws
	dirty := rws.dirty
	rws.handlerDone = true
	w.Flush()
	w.rws.stream.reqCtx.Reset()
	w.rws.stream.sc.engine.ReleaseReqCtx(w.rws.stream.reqCtx)
	w.rws = nil
	if !dirty {
		// Only recycle the pool if all prior Write calls to
		// the serverConn goroutine completed successfully. If
		// they returned earlier due to resets from the peer
		// there might still be write goroutines outstanding
		// from the serverConn referencing the rws memory. See
		// issue 20704.
		responseWriterStatePool.Put(rws)
	}
}

func (w *responseWriter) Push(target string, opts *http.PushOptions) error {
	st := w.rws.stream
	sc := st.sc
	sc.serveG.checkNotOn()

	// No recursive pushes: "PUSH_PROMISE frames MUST only be sent on a peer-initiated stream."
	// http://tools.ietf.org/html/rfc7540#section-6.6
	if st.isPushed() {
		return ErrRecursivePush
	}

	if opts == nil {
		opts = new(http.PushOptions)
	}

	// Default options.
	if opts.Method == "" {
		opts.Method = "GET"
	}
	if opts.Header == nil {
		opts.Header = http.Header{}
	}
	wantScheme := bytesconv.B2s(w.rws.stream.reqCtx.URI().Scheme())

	// Validate the request.
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	reqCtx := w.rws.stream.reqCtx
	if u.Scheme == "" {
		if !strings.HasPrefix(target, "/") {
			return fmt.Errorf("target must be an absolute URL or an absolute path: %q", target)
		}
		u.Scheme = wantScheme
		u.Host = bytesconv.B2s(reqCtx.Host())
	} else {
		if u.Scheme != wantScheme {
			return fmt.Errorf("cannot push URL with scheme %q from request with scheme %q", u.Scheme, wantScheme)
		}
		if u.Host == "" {
			return errors.New("URL must have a host")
		}
	}
	for k := range opts.Header {
		if strings.HasPrefix(k, ":") {
			return fmt.Errorf("promised request headers cannot include pseudo header %q", k)
		}
		// These headers are meaningful only if the request has a body,
		// but PUSH_PROMISE requests cannot have a body.
		// http://tools.ietf.org/html/rfc7540#section-8.2
		// Also disallow Host, since the promised URL must be absolute.
		switch strings.ToLower(k) {
		case "content-length", "content-encoding", "trailer", "te", "expect", "host":
			return fmt.Errorf("promised request headers cannot include %q", k)
		}
	}

	// The RFC effectively limits promised requests to GET and HEAD:
	// "Promised requests MUST be cacheable [GET, HEAD, or POST], and MUST be safe [GET or HEAD]"
	// http://tools.ietf.org/html/rfc7540#section-8.2
	if opts.Method != "GET" && opts.Method != "HEAD" {
		return fmt.Errorf("method %q must be GET or HEAD", opts.Method)
	}

	msg := &startPushRequest{
		parent: st,
		method: opts.Method,
		url:    u,
		header: cloneHeader(opts.Header),
		done:   errChanPool.Get().(chan error),
	}

	select {
	case <-sc.doneServing:
		return errClientDisconnected
	case <-st.cw:
		return errStreamClosed
	case sc.serveMsgCh <- msg:
	}

	select {
	case <-sc.doneServing:
		return errClientDisconnected
	case <-st.cw:
		return errStreamClosed
	case err := <-msg.done:
		errChanPool.Put(msg.done)
		return err
	}
}
