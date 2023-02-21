/*
 * Copyright 2023 CloudWeGo Authors
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

package http1

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	inStats "github.com/cloudwego/hertz/internal/stats"
	"github.com/cloudwego/hertz/pkg/app"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/common/tracer"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	"github.com/cloudwego/hertz/pkg/common/tracer/traceinfo"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

var pool = &sync.Pool{New: func() interface{} {
	return &eventStack{}
}}

func TestTraceEventCompleted(t *testing.T) {
	server := &Server{}
	server.eventStackPool = pool
	server.EnableTrace = true
	reqCtx := &app.RequestContext{}
	server.Core = &mockCore{
		ctxPool: &sync.Pool{New: func() interface{} {
			ti := traceinfo.NewTraceInfo()
			ti.Stats().SetLevel(2)
			reqCtx.SetTraceInfo(&mockTraceInfo{ti})
			return reqCtx
		}},
		controller: &inStats.Controller{},
	}
	err := server.Serve(context.TODO(), mock.NewConn("GET /aaa HTTP/1.1\nHost: foobar.com\n\n"))
	assert.True(t, errors.Is(err, errs.ErrShortConnection))
	traceInfo := reqCtx.GetTraceInfo()
	assert.False(t, traceInfo.Stats().GetEvent(stats.HTTPStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadHeaderStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadHeaderFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadBodyStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadBodyFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ServerHandleStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ServerHandleFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.WriteStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.WriteFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.HTTPFinish).IsNil())
}

func TestTraceEventReadHeaderError(t *testing.T) {
	server := &Server{}
	server.eventStackPool = pool
	server.EnableTrace = true
	reqCtx := &app.RequestContext{}
	server.Core = &mockCore{
		ctxPool: &sync.Pool{New: func() interface{} {
			ti := traceinfo.NewTraceInfo()
			ti.Stats().SetLevel(2)
			reqCtx.SetTraceInfo(&mockTraceInfo{ti})
			return reqCtx
		}},
		controller: &inStats.Controller{},
	}
	err := server.Serve(context.TODO(), mock.NewConn("ErrorFirstLine\r\n\r\n"))
	assert.NotNil(t, err)
	traceInfo := reqCtx.GetTraceInfo()
	assert.False(t, traceInfo.Stats().GetEvent(stats.HTTPStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadHeaderStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadHeaderFinish).IsNil())
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.ReadBodyStart))
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.ReadBodyFinish))
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.ServerHandleStart))
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.ServerHandleFinish))
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.WriteStart))
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.WriteFinish))
	assert.False(t, traceInfo.Stats().GetEvent(stats.HTTPFinish).IsNil())
}

func TestTraceEventReadBodyError(t *testing.T) {
	server := &Server{}
	server.eventStackPool = pool
	server.EnableTrace = true
	server.GetOnly = true
	reqCtx := &app.RequestContext{}
	server.Core = &mockCore{
		ctxPool: &sync.Pool{New: func() interface{} {
			ti := traceinfo.NewTraceInfo()
			ti.Stats().SetLevel(2)
			reqCtx.SetTraceInfo(&mockTraceInfo{ti})
			return reqCtx
		}},
		controller: &inStats.Controller{},
	}
	err := server.Serve(context.TODO(), mock.NewConn("POST /aaa HTTP/1.1\nHost: foobar.com\nContent-Length: 5\nContent-Type: foo/bar\n\n12346\n\n"))
	assert.NotNil(t, err)

	traceInfo := reqCtx.GetTraceInfo()
	assert.False(t, traceInfo.Stats().GetEvent(stats.HTTPStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadHeaderStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadHeaderFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadBodyStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadBodyFinish).IsNil())
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.ServerHandleStart))
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.ServerHandleFinish))
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.WriteStart))
	assert.Nil(t, traceInfo.Stats().GetEvent(stats.WriteFinish))
	assert.False(t, traceInfo.Stats().GetEvent(stats.HTTPFinish).IsNil())
}

func TestTraceEventWriteError(t *testing.T) {
	server := &Server{}
	server.eventStackPool = pool
	server.EnableTrace = true
	reqCtx := &app.RequestContext{}
	server.Core = &mockCore{
		ctxPool: &sync.Pool{New: func() interface{} {
			ti := traceinfo.NewTraceInfo()
			ti.Stats().SetLevel(2)
			reqCtx.SetTraceInfo(&mockTraceInfo{ti})
			return reqCtx
		}},
		controller: &inStats.Controller{},
	}
	err := server.Serve(
		context.TODO(),
		&mockErrorWriter{
			mock.NewConn("POST /aaa HTTP/1.1\nHost: foobar.com\nContent-Length: 5\nContent-Type: foo/bar\n\n12346\n\n"),
		},
	)
	assert.NotNil(t, err)
	traceInfo := reqCtx.GetTraceInfo()
	assert.False(t, traceInfo.Stats().GetEvent(stats.HTTPStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadHeaderStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadHeaderFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadBodyStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ReadBodyFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ServerHandleStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.ServerHandleFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.WriteStart).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.WriteFinish).IsNil())
	assert.False(t, traceInfo.Stats().GetEvent(stats.HTTPFinish).IsNil())
}

func TestEventStack(t *testing.T) {
	// Create a stack.
	s := &eventStack{}
	assert.True(t, s.isEmpty())

	count := 0

	// Push 10 events.
	for i := 0; i < 10; i++ {
		s.push(func(ti traceinfo.TraceInfo, err error) {
			count += 1
		})
	}

	assert.False(t, s.isEmpty())
	// Pop 10 events and process them.
	for last := s.pop(); last != nil; last = s.pop() {
		last(nil, nil)
	}

	assert.DeepEqual(t, 10, count)

	// Pop an empty stack.
	e := s.pop()
	if e != nil {
		t.Fatalf("should be nil")
	}
}

func TestDefaultWriter(t *testing.T) {
	server := &Server{}
	reqCtx := &app.RequestContext{}
	server.Core = &mockCore{
		ctxPool: &sync.Pool{New: func() interface{} {
			return reqCtx
		}},
		mockHandler: func(c context.Context, ctx *app.RequestContext) {
			ctx.Write([]byte("hello, hertz"))
			ctx.Flush()
		},
	}
	defaultConn := mock.NewConn("GET / HTTP/1.1\nHost: foobar.com\n\n")
	err := server.Serve(context.TODO(), defaultConn)
	assert.True(t, errors.Is(err, errs.ErrShortConnection))
	defaultResponseResult := defaultConn.WriterRecorder()
	assert.DeepEqual(t, 0, defaultResponseResult.Len()) // all data is flushed so the buffer length is 0
	response := protocol.AcquireResponse()
	resp.Read(response, defaultResponseResult)
	assert.DeepEqual(t, "hello, hertz", string(response.Body()))
}

func TestHijackResponseWriter(t *testing.T) {
	server := &Server{}
	reqCtx := &app.RequestContext{}
	buf := new(bytes.Buffer)
	isFinal := false
	server.Core = &mockCore{
		ctxPool: &sync.Pool{New: func() interface{} {
			return reqCtx
		}},
		mockHandler: func(c context.Context, ctx *app.RequestContext) {
			// response before write will be dropped
			ctx.Write([]byte("invalid data"))

			ctx.Response.HijackWriter(&mock.ExtWriter{
				Buf:     buf,
				IsFinal: &isFinal,
			})

			ctx.Write([]byte("hello, hertz"))
			ctx.Flush()
		},
	}
	defaultConn := mock.NewConn("GET / HTTP/1.1\nHost: foobar.com\n\n")
	err := server.Serve(context.TODO(), defaultConn)
	assert.True(t, errors.Is(err, errs.ErrShortConnection))
	defaultResponseResult := defaultConn.WriterRecorder()
	response := protocol.AcquireResponse()
	resp.Read(response, defaultResponseResult)
	assert.DeepEqual(t, 0, len(response.Body()))
	assert.DeepEqual(t, "hello, hertz", buf.String())
	assert.True(t, isFinal)
}

type mockCore struct {
	ctxPool     *sync.Pool
	controller  tracer.Controller
	mockHandler func(c context.Context, ctx *app.RequestContext)
}

func (m *mockCore) IsRunning() bool {
	return false
}

func (m *mockCore) GetCtxPool() *sync.Pool {
	return m.ctxPool
}

func (m *mockCore) ServeHTTP(c context.Context, ctx *app.RequestContext) {
	if m.mockHandler != nil {
		m.mockHandler(c, ctx)
	}
}

func (m *mockCore) GetTracer() tracer.Controller {
	return m.controller
}

type mockTraceInfo struct {
	traceinfo.TraceInfo
}

func (m *mockTraceInfo) Reset() {}

type mockErrorWriter struct {
	network.Conn
}

func (errorWriter *mockErrorWriter) Flush() error {
	return errors.New("error")
}
