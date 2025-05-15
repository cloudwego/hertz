/*
 * Copyright 2025 CloudWeGo Authors
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

package adaptor

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/hertz/internal/testutils"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/route"
)

//go:embed *
var adaptorFiles embed.FS

func runEngine(onCreate func(*route.Engine)) (string, *route.Engine) {
	opt := config.NewOptions(nil)
	opt.Addr = "127.0.0.1:0"
	engine := route.NewEngine(opt)
	onCreate(engine)
	go engine.Run()
	testutils.WaitEngineRunning(engine)
	return testutils.GetListenerAddr(engine), engine
}

func TestHertzHandler_BodyStream(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	h := HertzHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		b := make([]byte, 100)
		for i := 0; i < 3; i++ { // reading chunked data
			n, err := r.Body.Read(b)
			assert.Nil(t, err)
			assert.Assert(t, n == 5, n)
			assert.Assert(t, string(b[:n]) == "hello")
		}
		n, err := r.Body.Read(b)
		assert.Assert(t, err == io.EOF)
		assert.Assert(t, n == 0)
	}))
	addr, e := runEngine(func(e *route.Engine) {
		e.GetOptions().StreamRequestBody = true
		e.POST("/test", h)
	})
	defer e.Close()

	r, w := io.Pipe() // for sending chunked data
	req, err := http.NewRequest("POST", "http://"+addr+"/test", r)
	assert.Nil(t, err)
	cli := &http.Client{}
	go cli.Do(req)
	for i := 0; i < 3; i++ {
		w.Write([]byte("hello"))
		time.Sleep(50 * time.Millisecond)
	}
	w.Close()
	wg.Wait()
}

func TestHertzHandler_Chunked(t *testing.T) {
	h := HertzHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := w.(http.Flusher)
		w.Header().Set("Transfer-Encoding", "chunked")
		for i := 0; i < 5; i++ {
			chunk := fmt.Sprintf("data:%d", i)
			_, err := w.Write([]byte(chunk))
			assert.Nil(t, err)
			f.Flush()
			time.Sleep(20 * time.Millisecond)
		}
	}))
	addr, e := runEngine(func(e *route.Engine) {
		e.GET("/test", h)
	})
	defer e.Close()

	resp, err := http.Get("http://" + addr + "/test")
	assert.Nil(t, err)
	defer resp.Body.Close()
	assert.Assert(t, len(resp.TransferEncoding) == 1 && resp.TransferEncoding[0] == "chunked")
	for i := 0; i < 5; i++ {
		b := make([]byte, 10)
		n, err := resp.Body.Read(b)
		assert.Nil(t, err)
		assert.Assert(t, string(b[:n]) == fmt.Sprintf("data:%d", i))
	}
}

func TestHertzHandler_Hijack(t *testing.T) {
	h := HertzHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, rw, err := w.(http.Hijacker).Hijack()
		assert.Nil(t, err)
		_, _, err = w.(http.Hijacker).Hijack() // hijacked
		assert.NotNil(t, err)

		w.WriteHeader(500) // hijacked, noop
		_, err = w.Write([]byte("hello"))
		assert.Assert(t, err == errConnHijacked)

		rw.Write([]byte("hello"))
		rw.Flush()
		b := make([]byte, 10)
		n, err := rw.Read(b)
		assert.Nil(t, err)
		assert.Assert(t, string(b[:n]) == "world")
	}))
	addr, e := runEngine(func(e *route.Engine) {
		e.GET("/test", h)
	})
	defer e.Close()

	conn, err := net.Dial("tcp", addr)
	assert.Nil(t, err)
	defer conn.Close()
	conn.Write([]byte("GET /test HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	b := make([]byte, 100)
	n, err := conn.Read(b)
	assert.Nil(t, err)
	assert.Assert(t, string(b[:n]) == "hello", string(b[:n]))
	_, err = conn.Write([]byte("world"))
	assert.Nil(t, err)

	n, err = conn.Read(b) // Keep-Alive will not work if hijacked
	assert.Assert(t, err == io.EOF)
	assert.Assert(t, n == 0)
}

func TestHertzHandler_FSEmbed(t *testing.T) {
	addr, e := runEngine(func(e *route.Engine) {
		h := HertzHandler(http.FileServer(http.FS(adaptorFiles)))
		e.GET("/*filepath", h)
		e.HEAD("/*filepath", h)
	})
	defer e.Close()

	resp, err := http.Get("http://" + addr + "/handler_test.go")
	assert.Nil(t, err)

	expect := "hello, I'm handler_test.go"

	b, err := io.ReadAll(resp.Body)
	s := string(b)
	assert.Nil(t, err)
	assert.Assert(t, strings.Contains(s, expect), s)
}

func TestHertzHandler_Multipart(t *testing.T) {
	kvs := map[string]string{
		"name":  "Alice",
		"email": "alice@example.com",
	}
	h := HertzHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, expectv := range kvs {
			v := r.FormValue(k)
			assert.Assert(t, v == expectv, v)
		}
		w.WriteHeader(204)
		w.Write([]byte("hello"))
	}))
	addr, e := runEngine(func(e *route.Engine) {
		opts := e.GetOptions()
		opts.StreamRequestBody = true
		opts.DisablePreParseMultipartForm = true
		e.POST("/test", func(ctx context.Context, rc *app.RequestContext) {
			_, err := rc.MultipartForm() // call rc.MultipartForm before HertzHandler
			assert.Nil(t, err)
			h(ctx, rc)
		})
	})
	defer e.Close()

	body, ct := createMultipartBody(kvs)
	req, err := http.NewRequest("POST", "http://"+addr+"/test", bytes.NewReader(body.Bytes()))
	assert.Nil(t, err)
	req.Header.Set("Content-Type", ct)

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.Nil(t, err)
	assert.Assert(t, resp.StatusCode == 204, resp.StatusCode)
	resp.Body.Close()
}

func createMultipartBody(kvs map[string]string) (*bytes.Buffer, string) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for k, v := range kvs {
		_ = w.WriteField(k, v)
	}
	_ = w.Close()
	return buf, w.FormDataContentType()
}

func TestNoopHijackWriter(t *testing.T) {
	writer := noopHijackWriter{}

	// Test Write method
	n, err := writer.Write([]byte("test"))
	assert.Assert(t, n == 0, n)
	assert.Assert(t, err == errConnHijacked, err)

	// Test Flush method
	err = writer.Flush()
	assert.Assert(t, err == errConnHijacked, err)

	// Test Finalize method
	err = writer.Finalize()
	assert.Nil(t, err)
}

func TestNoopWriter(t *testing.T) {
	writer := noopWriter{}

	// Test Write method
	testData := []byte("test data")
	n, err := writer.Write(testData)
	assert.Assert(t, n == len(testData), n)
	assert.Nil(t, err)

	// Test Flush method
	err = writer.Flush()
	assert.Nil(t, err)

	// Test Finalize method
	err = writer.Finalize()
	assert.Nil(t, err)
}
