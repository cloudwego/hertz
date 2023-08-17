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

package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	c "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/app/server/registry"
	"github.com/cloudwego/hertz/pkg/common/config"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/req"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

func TestHertz_Run(t *testing.T) {
	hertz := New(WithHostPorts("127.0.0.1:6666"))
	hertz.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(time.Second)
		path := ctx.Request.URI().PathOriginal()
		ctx.SetBodyString(string(path))
	})

	testint := uint32(0)
	hertz.Engine.OnShutdown = append(hertz.OnShutdown, func(ctx context.Context) {
		atomic.StoreUint32(&testint, 1)
	})

	go hertz.Spin()
	time.Sleep(100 * time.Millisecond)

	hertz.Close()
	resp, err := http.Get("http://127.0.0.1:6666/test")
	assert.NotNil(t, err)
	assert.Nil(t, resp)
	assert.DeepEqual(t, uint32(0), atomic.LoadUint32(&testint))
}

func TestHertz_GracefulShutdown(t *testing.T) {
	engine := New(WithHostPorts("127.0.0.1:6667"))
	engine.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(time.Second * 2)
		path := ctx.Request.URI().PathOriginal()
		ctx.SetBodyString(string(path))
	})
	engine.GET("/test2", func(c context.Context, ctx *app.RequestContext) {})

	testint := uint32(0)
	testint2 := uint32(0)
	testint3 := uint32(0)
	engine.Engine.OnShutdown = append(engine.OnShutdown, func(ctx context.Context) {
		atomic.StoreUint32(&testint, 1)
	})
	engine.Engine.OnShutdown = append(engine.OnShutdown, func(ctx context.Context) {
		atomic.StoreUint32(&testint2, 2)
	})
	engine.Engine.OnShutdown = append(engine.OnShutdown, func(ctx context.Context) {
		time.Sleep(2 * time.Second)
		atomic.StoreUint32(&testint3, 3)
	})

	go engine.Spin()
	time.Sleep(time.Millisecond)

	hc := http.Client{Timeout: time.Second}
	var err error
	var resp *http.Response
	ch := make(chan struct{})
	ch2 := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()
		for range ticker.C {
			t.Logf("[%v]begin listening\n", time.Now())
			_, err2 := hc.Get("http://127.0.0.1:6667/test2")
			if err2 != nil {
				t.Logf("[%v]listening closed: %v", time.Now(), err2)
				ch2 <- struct{}{}
				break
			}
		}
	}()
	go func() {
		t.Logf("[%v]begin request\n", time.Now())
		resp, err = http.Get("http://127.0.0.1:6667/test")
		t.Logf("[%v]end request\n", time.Now())
		ch <- struct{}{}
	}()

	time.Sleep(time.Second * 1)
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	t.Logf("[%v]begin shutdown\n", start)
	engine.Shutdown(ctx)
	end := time.Now()
	t.Logf("[%v]end shutdown\n", end)

	<-ch
	assert.Nil(t, err)
	assert.NotNil(t, resp)
	assert.DeepEqual(t, true, resp.Close)
	assert.DeepEqual(t, uint32(1), atomic.LoadUint32(&testint))
	assert.DeepEqual(t, uint32(2), atomic.LoadUint32(&testint2))
	assert.DeepEqual(t, uint32(3), atomic.LoadUint32(&testint3))

	<-ch2

	cancel()
}

func TestLoadHTMLGlob(t *testing.T) {
	engine := New(WithMaxRequestBodySize(15), WithHostPorts("127.0.0.1:8890"))
	engine.Delims("{[{", "}]}")
	engine.LoadHTMLGlob("../../common/testdata/template/index.tmpl")
	engine.GET("/index", func(c context.Context, ctx *app.RequestContext) {
		ctx.HTML(consts.StatusOK, "index.tmpl", utils.H{
			"title": "Main website",
		})
	})
	go engine.Run()
	time.Sleep(200 * time.Millisecond)
	resp, _ := http.Get("http://127.0.0.1:8890/index")
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
	b := make([]byte, 100)
	n, _ := resp.Body.Read(b)
	const expected = `<html><h1>Main website</h1></html>`

	assert.DeepEqual(t, expected, string(b[0:n]))
}

func TestLoadHTMLFiles(t *testing.T) {
	engine := New(WithMaxRequestBodySize(15), WithHostPorts("127.0.0.1:8891"))
	engine.Delims("{[{", "}]}")
	engine.SetFuncMap(template.FuncMap{
		"formatAsDate": formatAsDate,
	})
	engine.LoadHTMLFiles("../../common/testdata/template/htmltemplate.html", "../../common/testdata/template/index.tmpl")

	engine.GET("/raw", func(c context.Context, ctx *app.RequestContext) {
		ctx.HTML(consts.StatusOK, "htmltemplate.html", map[string]interface{}{
			"now": time.Date(2017, 0o7, 0o1, 0, 0, 0, 0, time.UTC),
		})
	})
	go engine.Run()
	time.Sleep(200 * time.Millisecond)
	resp, _ := http.Get("http://127.0.0.1:8891/raw")
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
	b := make([]byte, 100)
	n, _ := resp.Body.Read(b)
	assert.DeepEqual(t, "<h1>Date: 2017/07/01</h1>", string(b[0:n]))
}

func formatAsDate(t time.Time) string {
	year, month, day := t.Date()
	return fmt.Sprintf("%d/%02d/%02d", year, month, day)
}

// copied from router
var default400Body = []byte("400 bad request")

func TestServer_Use(t *testing.T) {
	router := New()
	router.Use(func(c context.Context, ctx *app.RequestContext) {})
	assert.DeepEqual(t, 1, len(router.Handlers))
	router.Use(func(c context.Context, ctx *app.RequestContext) {})
	assert.DeepEqual(t, 2, len(router.Handlers))
}

func Test_getServerName(t *testing.T) {
	engine := New()
	assert.DeepEqual(t, []byte("hertz"), engine.GetServerName())
	ss := New()
	ss.Name = "test_name"
	assert.DeepEqual(t, []byte("test_name"), ss.GetServerName())
}

func TestServer_Run(t *testing.T) {
	hertz := New(WithHostPorts("127.0.0.1:8888"))
	hertz.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		path := ctx.Request.URI().PathOriginal()
		ctx.SetBodyString(string(path))
	})
	hertz.POST("/redirect", func(c context.Context, ctx *app.RequestContext) {
		ctx.Redirect(consts.StatusMovedPermanently, []byte("http://127.0.0.1:8888/test"))
	})
	go hertz.Run()
	time.Sleep(100 * time.Microsecond)
	resp, err := http.Get("http://127.0.0.1:8888/test")
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
	b := make([]byte, 5)
	resp.Body.Read(b)
	assert.DeepEqual(t, "/test", string(b))

	resp, err = http.Get("http://127.0.0.1:8888/foo")
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusNotFound, resp.StatusCode)

	resp, err = http.Post("http://127.0.0.1:8888/redirect", "", nil)
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
	b = make([]byte, 5)
	resp.Body.Read(b)
	assert.DeepEqual(t, "/test", string(b))

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = hertz.Shutdown(ctx)
}

func TestNotAbsolutePath(t *testing.T) {
	engine := New(WithHostPorts("127.0.0.1:9990"))
	engine.POST("/", func(c context.Context, ctx *app.RequestContext) {
		ctx.Write(ctx.Request.Body())
	})
	engine.POST("/a", func(c context.Context, ctx *app.RequestContext) {
		ctx.Write(ctx.Request.Body())
	})
	go engine.Run()
	time.Sleep(200 * time.Microsecond)

	s := "POST ?a=b HTTP/1.1\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)

	ctx := app.NewContext(0)
	if err := req.Read(&ctx.Request, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	engine.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, consts.StatusOK, ctx.Response.StatusCode())
	assert.DeepEqual(t, ctx.Request.Body(), ctx.Response.Body())

	s = "POST a?a=b HTTP/1.1\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr = mock.NewZeroCopyReader(s)

	ctx = app.NewContext(0)
	if err := req.Read(&ctx.Request, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	engine.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, consts.StatusOK, ctx.Response.StatusCode())
	assert.DeepEqual(t, ctx.Request.Body(), ctx.Response.Body())
}

func TestNotAbsolutePathWithRawPath(t *testing.T) {
	engine := New(WithHostPorts("127.0.0.1:9991"), WithUseRawPath(true))
	engine.POST("/", func(c context.Context, ctx *app.RequestContext) {
	})
	engine.POST("/a", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	time.Sleep(200 * time.Microsecond)

	s := "POST ?a=b HTTP/1.1\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)

	ctx := app.NewContext(0)
	if err := req.Read(&ctx.Request, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	engine.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, consts.StatusBadRequest, ctx.Response.StatusCode())
	assert.DeepEqual(t, default400Body, ctx.Response.Body())

	s = "POST a?a=b HTTP/1.1\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr = mock.NewZeroCopyReader(s)

	ctx = app.NewContext(0)
	if err := req.Read(&ctx.Request, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	engine.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, consts.StatusBadRequest, ctx.Response.StatusCode())
	assert.DeepEqual(t, default400Body, ctx.Response.Body())
}

func TestWithBasePath(t *testing.T) {
	engine := New(WithBasePath("/hertz"), WithHostPorts("127.0.0.1:19898"))
	engine.POST("/test", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	time.Sleep(500 * time.Microsecond)
	var r http.Request
	r.ParseForm()
	r.Form.Add("xxxxxx", "xxx")
	body := strings.NewReader(r.Form.Encode())
	resp, err := http.Post("http://127.0.0.1:19898/hertz/test", "application/x-www-form-urlencoded", body)
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
}

func TestNotEnoughBodySize(t *testing.T) {
	engine := New(WithMaxRequestBodySize(5), WithHostPorts("127.0.0.1:8889"))
	engine.POST("/test", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	time.Sleep(200 * time.Microsecond)
	var r http.Request
	r.ParseForm()
	r.Form.Add("xxxxxx", "xxx")
	body := strings.NewReader(r.Form.Encode())
	resp, err := http.Post("http://127.0.0.1:8889/test", "application/x-www-form-urlencoded", body)
	assert.Nil(t, err)
	assert.DeepEqual(t, 413, resp.StatusCode)
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	assert.DeepEqual(t, "Request Entity Too Large", string(bodyBytes))
}

func TestEnoughBodySize(t *testing.T) {
	engine := New(WithMaxRequestBodySize(15), WithHostPorts("127.0.0.1:8892"))
	engine.POST("/test", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	time.Sleep(200 * time.Microsecond)
	var r http.Request
	r.ParseForm()
	r.Form.Add("xxxxxx", "xxx")
	body := strings.NewReader(r.Form.Encode())
	resp, _ := http.Post("http://127.0.0.1:8892/test", "application/x-www-form-urlencoded", body)
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
}

func TestRequestCtxHijack(t *testing.T) {
	hijackStartCh := make(chan struct{})
	hijackStopCh := make(chan struct{})
	engine := New()
	engine.Init()

	engine.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		if ctx.Hijacked() {
			t.Error("connection mustn't be hijacked")
		}
		ctx.Hijack(func(c network.Conn) {
			<-hijackStartCh

			b := make([]byte, 1)
			// ping-pong echo via hijacked conn
			for {
				n, err := c.Read(b)
				if n != 1 {
					if err == io.EOF {
						close(hijackStopCh)
						return
					}
					if err != nil {
						t.Errorf("unexpected error: %s", err)
					}
					t.Errorf("unexpected number of bytes read: %d. Expecting 1", n)
				}
				if _, err = c.Write(b); err != nil {
					t.Errorf("unexpected error when writing data: %s", err)
				}
			}
		})
		if !ctx.Hijacked() {
			t.Error("connection must be hijacked")
		}
		ctx.Data(consts.StatusOK, "foo/bar", []byte("hijack it!"))
	})

	hijackedString := "foobar baz hijacked!!!"

	c := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n" + hijackedString)

	ch := make(chan error)
	go func() {
		ch <- engine.Serve(context.Background(), c)
	}()

	time.Sleep(100 * time.Millisecond)

	close(hijackStartCh)

	if err := <-ch; err != nil {
		if !errors.Is(err, errs.ErrHijacked) {
			t.Fatalf("Unexpected error from serveConn: %s", err)
		}
	}
	verifyResponse(t, c.WriterRecorder(), consts.StatusOK, "foo/bar", "hijack it!")

	select {
	case <-hijackStopCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}

	zw := c.WriterRecorder()
	data, err := zw.ReadBinary(zw.Len())
	if err != nil {
		t.Fatalf("Unexpected error when reading remaining data: %s", err)
	}
	if string(data) != hijackedString {
		t.Fatalf("Unexpected data read after the first response %q. Expecting %q", data, hijackedString)
	}
}

func verifyResponse(t *testing.T, zr network.Reader, expectedStatusCode int, expectedContentType, expectedBody string) {
	var r protocol.Response
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("Unexpected error when parsing response: %s", err)
	}

	if !bytes.Equal(r.Body(), []byte(expectedBody)) {
		t.Fatalf("Unexpected body %q. Expected %q", r.Body(), []byte(expectedBody))
	}
	verifyResponseHeader(t, &r.Header, expectedStatusCode, len(r.Body()), expectedContentType, "")
}

func verifyResponseHeader(t *testing.T, h *protocol.ResponseHeader, expectedStatusCode, expectedContentLength int, expectedContentType, expectedContentEncoding string) {
	if h.StatusCode() != expectedStatusCode {
		t.Fatalf("Unexpected status code %d. Expected %d", h.StatusCode(), expectedStatusCode)
	}
	if h.ContentLength() != expectedContentLength {
		t.Fatalf("Unexpected content length %d. Expected %d", h.ContentLength(), expectedContentLength)
	}
	if string(h.ContentType()) != expectedContentType {
		t.Fatalf("Unexpected content type %q. Expected %q", h.ContentType(), expectedContentType)
	}
	if string(h.ContentEncoding()) != expectedContentEncoding {
		t.Fatalf("Unexpected content encoding %q. Expected %q", h.ContentEncoding(), expectedContentEncoding)
	}
}

func TestParamInconsist(t *testing.T) {
	mapS := sync.Map{}
	h := New(WithHostPorts("localhost:10091"))
	h.GET("/:label", func(c context.Context, ctx *app.RequestContext) {
		label := ctx.Param("label")
		x, _ := mapS.LoadOrStore(label, label)
		labelString := x.(string)
		if label != labelString {
			t.Errorf("unexpected label: %s, expected return label: %s", label, labelString)
		}
	})
	go h.Run()
	time.Sleep(time.Millisecond * 50)
	client, _ := c.NewClient()
	wg := sync.WaitGroup{}
	tr := func() {
		defer wg.Done()
		for i := 0; i < 5000; i++ {
			client.Get(context.Background(), nil, "http://localhost:10091/test1")
		}
	}
	ti := func() {
		defer wg.Done()
		for i := 0; i < 5000; i++ {
			client.Get(context.Background(), nil, "http://localhost:10091/test2")
		}
	}

	for i := 0; i < 30; i++ {
		go tr()
		go ti()
		wg.Add(2)
	}
	wg.Wait()
}

func TestDuplicateReleaseBodyStream(t *testing.T) {
	h := New(WithStreamBody(true), WithHostPorts("localhost:10092"))
	h.POST("/test", func(ctx context.Context, c *app.RequestContext) {
		stream := c.RequestBodyStream()
		c.Response.SetBodyStream(stream, -1)
	})
	go h.Spin()
	time.Sleep(time.Second)
	client, _ := c.NewClient(c.WithMaxConnsPerHost(1000000), c.WithDialTimeout(time.Minute))
	bodyBytes := make([]byte, 102388)
	index := 0
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for i := 0; i < 102388; i++ {
		bodyBytes[i] = letterBytes[index]
		if i%1969 == 0 && i != 0 {
			index = index + 1
		}
	}
	body := string(bodyBytes)

	wg := sync.WaitGroup{}
	testFunc := func() {
		defer wg.Done()
		r := protocol.NewRequest("POST", "http://localhost:10092/test", nil)
		r.SetBodyString(body)
		resp := protocol.AcquireResponse()
		err := client.Do(context.Background(), r, resp)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
		}
		if body != string(resp.Body()) {
			t.Errorf("unequal body")
		}
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go testFunc()
	}
	wg.Wait()
}

func TestServiceRegisterFailed(t *testing.T) {
	mockRegErr := errors.New("mock register error")
	var rCount int32
	var drCount int32
	mockRegistry := MockRegistry{
		RegisterFunc: func(info *registry.Info) error {
			atomic.AddInt32(&rCount, 1)
			return mockRegErr
		},
		DeregisterFunc: func(info *registry.Info) error {
			atomic.AddInt32(&drCount, 1)
			return nil
		},
	}
	var opts []config.Option
	opts = append(opts, WithRegistry(mockRegistry, nil))
	opts = append(opts, WithHostPorts("127.0.0.1:9222"))
	srv := New(opts...)
	srv.Spin()
	time.Sleep(2 * time.Second)
	assert.Assert(t, atomic.LoadInt32(&rCount) == 1)
}

func TestServiceDeregisterFailed(t *testing.T) {
	mockDeregErr := errors.New("mock deregister error")
	var rCount int32
	var drCount int32
	mockRegistry := MockRegistry{
		RegisterFunc: func(info *registry.Info) error {
			atomic.AddInt32(&rCount, 1)
			return nil
		},
		DeregisterFunc: func(info *registry.Info) error {
			atomic.AddInt32(&drCount, 1)
			return mockDeregErr
		},
	}
	var opts []config.Option
	opts = append(opts, WithRegistry(mockRegistry, nil))
	opts = append(opts, WithHostPorts("127.0.0.1:9223"))
	srv := New(opts...)
	go srv.Spin()
	time.Sleep(1 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	time.Sleep(1 * time.Second)
	assert.Assert(t, atomic.LoadInt32(&rCount) == 1)
	assert.Assert(t, atomic.LoadInt32(&drCount) == 1)
}

func TestServiceRegistryInfo(t *testing.T) {
	registryInfo := &registry.Info{
		Weight:      100,
		Tags:        map[string]string{"aa": "bb"},
		ServiceName: "hertz.api.test",
	}
	checkInfo := func(info *registry.Info) {
		assert.Assert(t, info.Weight == registryInfo.Weight)
		assert.Assert(t, info.ServiceName == "hertz.api.test")
		assert.Assert(t, len(info.Tags) == len(registryInfo.Tags), info.Tags)
		assert.Assert(t, info.Tags["aa"] == registryInfo.Tags["aa"], info.Tags)
	}
	var rCount int32
	var drCount int32
	mockRegistry := MockRegistry{
		RegisterFunc: func(info *registry.Info) error {
			checkInfo(info)
			atomic.AddInt32(&rCount, 1)
			return nil
		},
		DeregisterFunc: func(info *registry.Info) error {
			checkInfo(info)
			atomic.AddInt32(&drCount, 1)
			return nil
		},
	}
	var opts []config.Option
	opts = append(opts, WithRegistry(mockRegistry, registryInfo))
	opts = append(opts, WithHostPorts("127.0.0.1:9225"))
	srv := New(opts...)
	go srv.Spin()
	time.Sleep(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = srv.Shutdown(ctx)
	time.Sleep(2 * time.Second)
	assert.Assert(t, atomic.LoadInt32(&rCount) == 1)
	assert.Assert(t, atomic.LoadInt32(&drCount) == 1)
}

func TestServiceRegistryNoInitInfo(t *testing.T) {
	checkInfo := func(info *registry.Info) {
		assert.Assert(t, info == nil)
	}
	var rCount int32
	var drCount int32
	mockRegistry := MockRegistry{
		RegisterFunc: func(info *registry.Info) error {
			checkInfo(info)
			atomic.AddInt32(&rCount, 1)
			return nil
		},
		DeregisterFunc: func(info *registry.Info) error {
			checkInfo(info)
			atomic.AddInt32(&drCount, 1)
			return nil
		},
	}
	var opts []config.Option
	opts = append(opts, WithRegistry(mockRegistry, nil))
	opts = append(opts, WithHostPorts("127.0.0.1:9227"))
	srv := New(opts...)
	go srv.Spin()
	time.Sleep(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = srv.Shutdown(ctx)
	time.Sleep(2 * time.Second)
	assert.Assert(t, atomic.LoadInt32(&rCount) == 1)
	assert.Assert(t, atomic.LoadInt32(&drCount) == 1)
}

type testTracer struct{}

func (t testTracer) Start(ctx context.Context, c *app.RequestContext) context.Context {
	value := 0
	if v := ctx.Value("testKey"); v != nil {
		value = v.(int)
		value++
	}
	return context.WithValue(ctx, "testKey", value)
}

func (t testTracer) Finish(ctx context.Context, c *app.RequestContext) {}

func TestReuseCtx(t *testing.T) {
	h := New(WithTracer(testTracer{}), WithHostPorts("localhost:9228"))
	h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		assert.DeepEqual(t, 0, ctx.Value("testKey").(int))
	})

	go h.Spin()
	time.Sleep(time.Second)
	for i := 0; i < 1000; i++ {
		_, _, err := c.Get(context.Background(), nil, "http://127.0.0.1:9228/ping")
		assert.Nil(t, err)
	}
}

type CloseWithoutResetBuffer interface {
	CloseNoResetBuffer() error
}

func TestOnprepare(t *testing.T) {
	h := New(
		WithHostPorts("localhost:9229"),
		WithOnConnect(func(ctx context.Context, conn network.Conn) context.Context {
			b, err := conn.Peek(3)
			assert.Nil(t, err)
			assert.DeepEqual(t, string(b), "GET")
			if c, ok := conn.(CloseWithoutResetBuffer); ok {
				c.CloseNoResetBuffer()
			} else {
				conn.Close()
			}
			return ctx
		}))
	h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, utils.H{"ping": "pong"})
	})

	go h.Spin()
	time.Sleep(time.Second)
	_, _, err := c.Get(context.Background(), nil, "http://127.0.0.1:9229/ping")
	assert.DeepEqual(t, "the server closed connection before returning the first response byte. Make sure the server returns 'Connection: close' response header before closing the connection", err.Error())

	h = New(
		WithOnAccept(func(conn net.Conn) context.Context {
			conn.Close()
			return context.Background()
		}),
		WithHostPorts("localhost:9230"))
	h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, utils.H{"ping": "pong"})
	})
	go h.Spin()
	time.Sleep(time.Second)
	_, _, err = c.Get(context.Background(), nil, "http://127.0.0.1:9230/ping")
	if err == nil {
		t.Fatalf("err should not be nil")
	}

	h = New(
		WithOnAccept(func(conn net.Conn) context.Context {
			assert.DeepEqual(t, conn.LocalAddr().String(), "127.0.0.1:9231")
			return context.Background()
		}),
		WithHostPorts("localhost:9231"),
		WithTransport(standard.NewTransporter))
	h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, utils.H{"ping": "pong"})
	})
	go h.Spin()
	time.Sleep(time.Second)
	c.Get(context.Background(), nil, "http://127.0.0.1:9231/ping")
}

type lockBuffer struct {
	sync.Mutex
	b bytes.Buffer
}

func (l *lockBuffer) Write(p []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.b.Write(p)
}

func (l *lockBuffer) String() string {
	l.Lock()
	defer l.Unlock()
	return l.b.String()
}

func TestSilentMode(t *testing.T) {
	hlog.SetSilentMode(true)
	b := &lockBuffer{b: bytes.Buffer{}}

	hlog.SetOutput(b)

	h := New(WithHostPorts("localhost:9232"), WithTransport(standard.NewTransporter))
	h.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.Write([]byte("hello, world"))
	})
	go h.Spin()
	time.Sleep(time.Second)

	d := standard.NewDialer()
	conn, _ := d.DialConnection("tcp", "127.0.0.1:9232", 0, nil)
	conn.Write([]byte("aaa"))
	conn.Close()

	if strings.Contains(b.String(), "Error") {
		t.Fatalf("unexpected error in log: %s", b.String())
	}
}
