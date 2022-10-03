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
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

type Trailer = []struct {
	k []byte
	v []byte
}

// stream represents a stream. This is the minimal metadata needed by
// the serve goroutine. Most of the actual stream state is owned by
// the http.Handler's goroutine in the responseWriter. Because the
// responseWriter's responseWriterState is recycled at the end of a
// handler, this struct intentionally has no pointer to the
// *responseWriter{,State} itself, as the Handler ending nils out the
// responseWriter's state field.
type stream struct {
	// immutable:
	sc        *serverConn
	id        uint32
	body      *pipe       // non-nil if expecting DATA frames
	cw        closeWaiter // closed wait stream transitions to closed state
	reqCtx    *app.RequestContext
	baseCtx   context.Context
	cancelCtx func()

	// owned by serverConn's serve loop:
	bodyBytes        int64 // body bytes seen so far
	declBodyBytes    int64 // or -1 if undeclared
	flow             flow  // limits writing from Handler to client
	inflow           flow  // what the client is allowed to POST/etc to us
	state            streamState
	resetQueued      bool        // RST_STREAM queued for write; set by sc.resetStream
	gotTrailerHeader bool        // HEADER frame for trailers was seen
	wroteHeaders     bool        // whether we wrote headers (not status 100)
	writeDeadline    *time.Timer // nil if unused
	rw               *responseWriter
	handler          app.HandlerFunc

	// FIXME IMPL Trailer
	// trailer    Trailer // accumulated trailers
	// reqTrailer Trailer // handler's Request.Trailer
}

// copyTrailersToHandlerRequest is run in the Handler's goroutine in
// its Request.Body.Read just before it gets io.EOF.
func (st *stream) copyTrailersToHandlerRequest() {
	//for k, vv := range st.trailer {
	//	if _, ok := st.reqTrailer[k]; ok {
	//		// Only copy it over it was pre-declared.
	//		st.reqTrailer[k] = vv
	//	}
	//}
}

func (st *stream) processTrailerHeaders(f *MetaHeadersFrame) error {
	sc := st.sc
	sc.serveG.check()
	if st.gotTrailerHeader {
		return ConnectionError(ErrCodeProtocol)
	}
	st.gotTrailerHeader = true
	if !f.StreamEnded() {
		return streamError(st.id, ErrCodeProtocol)
	}

	if len(f.PseudoFields()) > 0 {
		return streamError(st.id, ErrCodeProtocol)
	}

	st.endStream()
	return nil
}
