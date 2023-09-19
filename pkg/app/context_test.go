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

package app

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/app/server/render"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/common/testdata/proto"
	"github.com/cloudwego/hertz/pkg/common/tracer/traceinfo"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/req"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
	con "github.com/cloudwego/hertz/pkg/route/consts"
	"github.com/cloudwego/hertz/pkg/route/param"
)

func TestProtobuf(t *testing.T) {
	ctx := NewContext(0)
	body := proto.TestStruct{Body: []byte("Hello World")}
	ctx.ProtoBuf(consts.StatusOK, &body)

	assert.DeepEqual(t, string(ctx.Response.Body()), "\n\vHello World")
}

func TestPureJson(t *testing.T) {
	ctx := NewContext(0)
	ctx.PureJSON(consts.StatusOK, utils.H{
		"html": "<b>Hello World</b>",
	})
	if string(ctx.Response.Body()) != "{\"html\":\"<b>Hello World</b>\"}\n" {
		t.Fatalf("unexpected purejson: %#v, expected: %#v", string(ctx.Response.Body()), "<b>Hello World</b>")
	}
}

func TestIndentedJSON(t *testing.T) {
	ctx := NewContext(0)
	ctx.IndentedJSON(consts.StatusOK, utils.H{
		"foo":  "bar",
		"html": "h1",
	})
	if string(ctx.Response.Body()) != "{\n    \"foo\": \"bar\",\n    \"html\": \"h1\"\n}" {
		t.Fatalf("unexpected purejson: %#v, expected: %#v", string(ctx.Response.Body()), "{\n    \"foo\": \"bar\",\n    \"html\": \"<b>\"\n}")
	}
}

func TestContext(t *testing.T) {
	reqContext := NewContext(0)
	reqContext.Set("testContextKey", "testValue")
	ctx := reqContext
	if ctx.Value("testContextKey") != "testValue" {
		t.Fatalf("unexpected value: %#v, expected: %#v", ctx.Value("testContextKey"), "testValue")
	}
}

func TestValue(t *testing.T) {
	ctx := NewContext(0)

	v := ctx.Value("testContextKey")
	assert.Nil(t, v)

	ctx.Set("testContextKey", "testValue")
	v = ctx.Value("testContextKey")
	assert.DeepEqual(t, "testValue", v)
}

func TestContextNotModified(t *testing.T) {
	reqContext := NewContext(0)
	reqContext.Response.SetStatusCode(consts.StatusOK)
	if reqContext.Response.StatusCode() != consts.StatusOK {
		t.Fatalf("unexpected status code: %#v, expected: %#v", reqContext.Response.StatusCode(), consts.StatusOK)
	}
	reqContext.NotModified()
	if reqContext.Response.StatusCode() != consts.StatusNotModified {
		t.Fatalf("unexpected status code: %#v, expected: %#v", reqContext.Response.StatusCode(), consts.StatusNotModified)
	}
}

func TestIfModifiedSince(t *testing.T) {
	ctx := NewContext(0)
	var req protocol.Request
	req.Header.Set(string(bytestr.StrIfModifiedSince), "Mon, 02 Jan 2006 15:04:05 MST")
	req.CopyTo(&ctx.Request)
	if !ctx.IfModifiedSince(time.Now()) {
		t.Fatalf("ifModifiedSice error, expected false, but get true")
	}
	tt, _ := time.Parse(time.RFC3339, "2004-11-12T11:45:26.371Z")
	if ctx.IfModifiedSince(tt) {
		t.Fatalf("ifModifiedSice error, expected true, but get false")
	}
}

func TestWrite(t *testing.T) {
	ctx := NewContext(0)
	l, err := ctx.Write([]byte("test body"))
	if err != nil {
		t.Fatalf("unexpected error: %#v", err.Error())
	}
	if l != 9 {
		t.Fatalf("unexpected len: %#v, expected: %#v", l, 9)
	}
	if string(ctx.Response.BodyBytes()) != "test body" {
		t.Fatalf("unexpected body: %#v, expected: %#v", string(ctx.Response.BodyBytes()), "test body")
	}
}

func TestSetConnectionClose(t *testing.T) {
	ctx := NewContext(0)
	ctx.SetConnectionClose()
	if !ctx.Response.Header.ConnectionClose() {
		t.Fatalf("expected close connection, but not")
	}
}

func TestNotFound(t *testing.T) {
	ctx := NewContext(0)
	ctx.NotFound()
	if ctx.Response.StatusCode() != consts.StatusNotFound || string(ctx.Response.BodyBytes()) != "404 Page not found" {
		t.Fatalf("unexpected status code or body")
	}
}

func TestRedirect(t *testing.T) {
	ctx := NewContext(0)
	ctx.Redirect(consts.StatusFound, []byte("/hello"))
	assert.DeepEqual(t, consts.StatusFound, ctx.Response.StatusCode())

	ctx.redirect([]byte("/hello"), consts.StatusMovedPermanently)
	assert.DeepEqual(t, consts.StatusMovedPermanently, ctx.Response.StatusCode())
}

func TestGetRedirectStatusCode(t *testing.T) {
	val := getRedirectStatusCode(consts.StatusMovedPermanently)
	assert.DeepEqual(t, consts.StatusMovedPermanently, val)

	val = getRedirectStatusCode(consts.StatusNotFound)
	assert.DeepEqual(t, consts.StatusFound, val)
}

func TestCookie(t *testing.T) {
	ctx := NewContext(0)
	ctx.Request.Header.SetCookie("cookie", "test cookie")
	if string(ctx.Cookie("cookie")) != "test cookie" {
		t.Fatalf("unexpected cookie: %#v, expected get: %#v", string(ctx.Cookie("cookie")), "test cookie")
	}
}

func TestUserAgent(t *testing.T) {
	ctx := NewContext(0)
	ctx.Request.Header.SetUserAgentBytes([]byte("user agent"))
	if string(ctx.UserAgent()) != "user agent" {
		t.Fatalf("unexpected user agent: %#v, expected get: %#v", string(ctx.UserAgent()), "user agent")
	}
}

func TestStatus(t *testing.T) {
	ctx := NewContext(0)
	ctx.Status(consts.StatusOK)
	if ctx.Response.StatusCode() != consts.StatusOK {
		t.Fatalf("expected get consts.StatusOK, but not")
	}
}

func TestPost(t *testing.T) {
	ctx := NewContext(0)
	ctx.Request.Header.SetMethod(consts.MethodPost)
	if !ctx.IsPost() {
		t.Fatalf("expected post method , but get: %#v", ctx.Method())
	}

	if string(ctx.Method()) != consts.MethodPost {
		t.Fatalf("expected post method , but get: %#v", ctx.Method())
	}
}

func TestGet(t *testing.T) {
	ctx := NewContext(0)
	ctx.Request.Header.SetMethod(consts.MethodPost)
	assert.False(t, ctx.IsGet())

	ctx.Request.Header.SetMethod(consts.MethodGet)
	assert.True(t, ctx.IsGet())
}

func TestCopy(t *testing.T) {
	t.Parallel()
	ctx := NewContext(0)
	ctx.Request.Header.Add("header_a", "header_value_a")
	ctx.Response.Header.Add("header_b", "header_value_b")
	ctx.Params = param.Params{
		{Key: "key_a", Value: "value_a"},
		{Key: "key_b", Value: "value_b"},
		{Key: "key_c", Value: "value_b"},
		{Key: "key_d", Value: "value_b"},
		{Key: "key_e", Value: "value_b"},
		{Key: "key_f", Value: "value_b"},
		{Key: "key_g", Value: "value_b"},
		{Key: "key_h", Value: "value_b"},
		{Key: "key_i", Value: "value_b"},
	}
	ctx.Set("map_key_a", "map_value_a")
	ctx.Set("map_key_b", "map_value_b")
	for i := 0; i <= 10000; i++ {
		c := ctx.Copy()
		go func(context *RequestContext) {
			str, _ := context.Params.Get("key_a")
			if str != "value_a" {
				t.Errorf("unexpected value: %#v, expected: %#v", str, "value_a")
				return
			}

			reqHeaderStr := context.Request.Header.Get("header_a")
			if reqHeaderStr != "header_value_a" {
				t.Errorf("unexpected value: %#v, expected: %#v", reqHeaderStr, "header_value_a")
				return
			}

			respHeaderStr := context.Response.Header.Get("header_b")
			if respHeaderStr != "header_value_b" {
				t.Errorf("unexpected value: %#v, expected: %#v", respHeaderStr, "header_value_b")
				return
			}

			iStr := ctx.Value("map_key_a")
			if iStr.(string) != "map_value_a" {
				t.Errorf("unexpected value: %#v, expected: %#v", iStr.(string), "map_value_a")
				return
			}

			context.Params = context.Params[0:0]
			context.Params = append(context.Params, param.Param{Key: "key_a", Value: "value_a_"})

			context.Request.Header.Reset()
			context.Request.Header.Add("header_a", "header_value_a_")
			context.Response.Header.Reset()
			context.Response.Header.Add("header_b", "header_value_b_")
			context.Keys = nil
			context.Keys = make(map[string]interface{})
			context.Set("header_value_a", "map_value_a_")
		}(c)
	}
}

func TestQuery(t *testing.T) {
	var r protocol.Request
	ctx := NewContext(0)
	s := "POST /foo?name=menu&value= HTTP/1.1\r\nHost: google.com\r\nTransfer-Encoding: chunked\r\nContent-Type: aa/bb\r\n\r\n3  \r\nabc\r\n0\r\n\r\n"
	zr := mock.NewZeroCopyReader(s)
	err := req.Read(&r, zr)
	if err != nil {
		t.Fatalf("Unexpected error when reading chunked request: %s", err)
	}
	r.CopyTo(&ctx.Request)
	if ctx.Query("name") != "menu" {
		t.Fatalf("unexpected query: %#v, expected menu", ctx.Query("name"))
	}

	if ctx.DefaultQuery("name", "default value") != "menu" {
		t.Fatalf("unexpected query: %#v, expected menu", ctx.Query("name"))
	}

	if ctx.DefaultQuery("defaultQuery", "default value") != "default value" {
		t.Fatalf("unexpected query: %#v, expected `default value`", ctx.Query("defaultQuery"))
	}
}

func TestMethod(t *testing.T) {
	ctx := NewContext(0)
	ctx.Status(consts.StatusOK)
	if ctx.Response.StatusCode() != consts.StatusOK {
		t.Fatalf("expected get consts.StatusOK, but not")
	}
}

func makeCtxByReqString(t *testing.T, s string) *RequestContext {
	ctx := NewContext(0)

	mr := mock.NewZeroCopyReader(s)
	if err := req.Read(&ctx.Request, mr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	return ctx
}

func TestPostForm(t *testing.T) {
	t.Parallel()

	ctx := makeCtxByReqString(t, `POST /upload HTTP/1.1
Host: localhost:10000
Content-Length: 521
Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg

------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f1"

value1
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="fileaaa"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
`)

	if ctx.PostForm("f1") != "value1" {
		t.Fatalf("PostForm get Multipart Form data failed")
	}
	if ctx.PostForm("fileaaa") != "" {
		t.Fatalf("PostForm should not get file")
	}

	ctx = makeCtxByReqString(t, `POST /upload HTTP/1.1
Host: localhost:10000
Content-Length: 11
Content-Type: application/x-www-form-urlencoded

hello=world`)

	if ctx.PostForm("hello") != "world" {
		t.Fatalf("PostForm get form failed")
	}
}

func TestDefaultPostForm(t *testing.T) {
	ctx := makeCtxByReqString(t, `POST /upload HTTP/1.1
Host: localhost:10000
Content-Length: 521
Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg

------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f1"

value1
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="fileaaa"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
`)

	val := ctx.DefaultPostForm("f1", "no val")
	assert.DeepEqual(t, "value1", val)

	val = ctx.DefaultPostForm("f99", "no val")
	assert.DeepEqual(t, "no val", val)
}

func TestRequestContext_FormFile(t *testing.T) {
	t.Parallel()

	s := `POST /upload HTTP/1.1
Host: localhost:10000
Content-Length: 521
Content-Type: multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg

------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f1"

value1
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="fileaaa"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`

	mr := mock.NewZeroCopyReader(s)

	ctx := NewContext(0)
	if err := req.Read(&ctx.Request, mr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	tail, err := ioutil.ReadAll(mr)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "tailfoobar" {
		t.Fatalf("unexpected tail %q. Expecting %q", tail, "tailfoobar")
	}

	f, err := ctx.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer ctx.Request.RemoveMultipartFormFiles()

	// verify files
	if len(f.File) != 1 {
		t.Fatalf("unexpected number of file values in multipart form: %d. Expecting 1", len(f.File))
	}
	for k, vv := range f.File {
		if k != "fileaaa" {
			t.Fatalf("unexpected file value name %q. Expecting %q", k, "fileaaa")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of file values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v.Filename != "TODO" {
			t.Fatalf("unexpected filename %q. Expecting %q", v.Filename, "TODO")
		}
		ct := v.Header.Get("Content-Type")
		if ct != consts.MIMEApplicationOctetStream {
			t.Fatalf("unexpected content-type %q. Expecting %q", ct, "application/octet-stream")
		}
	}

	err = ctx.SaveUploadedFile(f.File["fileaaa"][0], "TODO")
	assert.Nil(t, err)
	fileInfo, err := os.Stat("TODO")
	assert.Nil(t, err)
	assert.DeepEqual(t, "TODO", fileInfo.Name())
	assert.DeepEqual(t, f.File["fileaaa"][0].Size, fileInfo.Size())
	err = os.Remove("TODO")
	assert.Nil(t, err)

	ff, err := ctx.FormFile("fileaaa")
	if err != nil || ff == nil {
		t.Fatalf("unexpected error happened when ctx.FormFile()")
	}

	buf := make([]byte, ff.Size)
	fff, _ := ff.Open()
	fff.Read(buf)

	if !strings.Contains(string(buf), "- SessionClient") {
		t.Fatalf("unexpected file content. Expecting %q", "- SessionClient")
	}

	if !strings.Contains(string(buf), "rfc7540 .") {
		t.Fatalf("unexpected file content. Expecting %q", "rfc7540 .")
	}
}

func TestContextRenderFileFromFS(t *testing.T) {
	t.Parallel()

	ctx := NewContext(0)
	var req protocol.Request
	req.Header.SetMethod(consts.MethodGet)
	req.SetRequestURI("/some/path")
	req.CopyTo(&ctx.Request)

	ctx.FileFromFS("./fs.go", &FS{
		Root:               ".",
		IndexNames:         nil,
		GenerateIndexPages: false,
		AcceptByteRange:    true,
	})

	assert.DeepEqual(t, consts.StatusOK, ctx.Response.StatusCode())
	assert.True(t, strings.Contains(resp.GetHTTP1Response(&ctx.Response).String(), "func (fs *FS) initRequestHandler() {"))
	assert.DeepEqual(t, consts.MIMETextPlainUTF8, string(ctx.Response.Header.Peek("Content-Type")))
	assert.DeepEqual(t, "/some/path", string(ctx.Request.URI().Path()))
}

func TestContextRenderFile(t *testing.T) {
	t.Parallel()

	ctx := NewContext(0)
	var req protocol.Request
	req.Header.SetMethod(consts.MethodGet)
	req.SetRequestURI("/")
	req.CopyTo(&ctx.Request)

	ctx.File("./fs.go")

	assert.DeepEqual(t, consts.StatusOK, ctx.Response.StatusCode())
	assert.True(t, strings.Contains(resp.GetHTTP1Response(&ctx.Response).String(), "func (fs *FS) initRequestHandler() {"))
	assert.DeepEqual(t, consts.MIMETextPlainUTF8, string(ctx.Response.Header.Peek("Content-Type")))
}

func TestContextRenderAttachment(t *testing.T) {
	t.Parallel()

	ctx := NewContext(0)
	var req protocol.Request
	req.Header.SetMethod(consts.MethodGet)
	req.SetRequestURI("/")
	req.CopyTo(&ctx.Request)
	newFilename := "new_filename.go"

	ctx.FileAttachment("./context.go", newFilename)

	assert.DeepEqual(t, consts.StatusOK, ctx.Response.StatusCode())
	assert.True(t, strings.Contains(resp.GetHTTP1Response(&ctx.Response).String(),
		"func (ctx *RequestContext) FileAttachment(filepath, filename string) {"))
	assert.DeepEqual(t, fmt.Sprintf("attachment; filename=\"%s\"", newFilename),
		string(ctx.Response.Header.Peek("Content-Disposition")))
}

func TestRequestContext_Header(t *testing.T) {
	c := NewContext(0)

	c.Header("header_key", "header_val")
	val := string(c.Response.Header.Peek("header_key"))
	if val != "header_val" {
		t.Fatalf("unexpected %q. Expecting %q", val, "header_val")
	}

	c.Response.Header.Del("header_key")
	val = string(c.Response.Header.Peek("header_key"))
	if val != "" {
		t.Fatalf("unexpected %q. Expecting %q", val, "")
	}

	c.Header("header_key1", "header_val1")
	c.Header("header_key1", "")
	val = string(c.Response.Header.Peek("header_key1"))
	if val != "" {
		t.Fatalf("unexpected %q. Expecting %q", val, "")
	}
}

func TestRequestContext_Keys(t *testing.T) {
	c := NewContext(0)
	rightVal := "123"
	c.Set("key", rightVal)
	val := c.GetString("key")
	if val != rightVal {
		t.Fatalf("unexpected %v. Expecting %v", val, rightVal)
	}
}

func testFunc(c context.Context, ctx *RequestContext) {
	ctx.Next(c)
}

func testFunc2(c context.Context, ctx *RequestContext) {
	ctx.Set("key", "123")
}

func TestRequestContext_Handler(t *testing.T) {
	c := NewContext(0)
	c.handlers = HandlersChain{testFunc, testFunc2}

	c.Handler()(context.Background(), c)
	val := c.GetString("key")
	if val != "123" {
		t.Fatalf("unexpected %v. Expecting %v", val, "123")
	}

	c.handlers = nil
	handler := c.Handler()
	assert.Nil(t, handler)
}

func TestRequestContext_Handlers(t *testing.T) {
	c := NewContext(0)
	hc := HandlersChain{testFunc, testFunc2}
	c.SetHandlers(hc)
	c.Handlers()[1](context.Background(), c)
	val := c.GetString("key")
	if val != "123" {
		t.Fatalf("unexpected %v. Expecting %v", val, "123")
	}
}

func TestRequestContext_HandlerName(t *testing.T) {
	c := NewContext(0)
	c.handlers = HandlersChain{testFunc, testFunc2}
	val := c.HandlerName()
	if val != "github.com/cloudwego/hertz/pkg/app.testFunc2" {
		t.Fatalf("unexpected %v. Expecting %v", val, "github.com/cloudwego/hertz.testFunc2")
	}
}

func TestNext(t *testing.T) {
	c := NewContext(0)
	a := 0

	testFunc1 := func(c context.Context, ctx *RequestContext) {
		a = 1
	}
	testFunc3 := func(c context.Context, ctx *RequestContext) {
		a = 3
	}
	c.handlers = HandlersChain{testFunc1, testFunc3}

	c.Next(context.Background())

	assert.True(t, c.index == 2)
	assert.DeepEqual(t, 3, a)
}

func TestContextError(t *testing.T) {
	c := NewContext(0)
	assert.Nil(t, c.Errors)

	firstErr := errors.New("first error")
	c.Error(firstErr) // nolint: errcheck
	assert.DeepEqual(t, 1, len(c.Errors))
	assert.DeepEqual(t, "Error #01: first error\n", c.Errors.String())

	secondErr := errors.New("second error")
	c.Error(&errs.Error{ // nolint: errcheck
		Err:  secondErr,
		Meta: "some data 2",
		Type: errs.ErrorTypePublic,
	})
	assert.DeepEqual(t, 2, len(c.Errors))

	assert.DeepEqual(t, firstErr, c.Errors[0].Err)
	assert.Nil(t, c.Errors[0].Meta)
	assert.DeepEqual(t, errs.ErrorTypePrivate, c.Errors[0].Type)

	assert.DeepEqual(t, secondErr, c.Errors[1].Err)
	assert.DeepEqual(t, "some data 2", c.Errors[1].Meta)
	assert.DeepEqual(t, errs.ErrorTypePublic, c.Errors[1].Type)

	assert.DeepEqual(t, c.Errors.Last(), c.Errors[1])

	defer func() {
		if recover() == nil {
			t.Error("didn't panic")
		}
	}()
	c.Error(nil) // nolint: errcheck
}

func TestContextAbortWithError(t *testing.T) {
	c := NewContext(0)

	c.AbortWithError(consts.StatusUnauthorized, errors.New("bad input")).SetMeta("some input") // nolint: errcheck

	assert.DeepEqual(t, consts.StatusUnauthorized, c.Response.StatusCode())
	assert.DeepEqual(t, con.AbortIndex, c.index)
	assert.True(t, c.IsAborted())
}

func TestRender(t *testing.T) {
	c := NewContext(0)

	c.Render(consts.StatusOK, &render.Data{
		ContentType: consts.MIMEApplicationJSONUTF8,
		Data:        []byte("{\"test\":1}"),
	})

	assert.DeepEqual(t, consts.StatusOK, c.Response.StatusCode())
	assert.True(t, strings.Contains(string(c.Response.Body()), "test"))

	c.Reset()
	c.Render(110, &render.Data{
		ContentType: "application/json; charset=utf-8",
		Data:        []byte("{\"test\":1}"),
	})
	assert.DeepEqual(t, "application/json; charset=utf-8", string(c.Response.Header.ContentType()))
	assert.DeepEqual(t, "", string(c.Response.Body()))

	c.Reset()
	c.Render(consts.StatusNoContent, &render.Data{
		ContentType: "application/json; charset=utf-8",
		Data:        []byte("{\"test\":1}"),
	})
	assert.DeepEqual(t, "application/json; charset=utf-8", string(c.Response.Header.ContentType()))
	assert.DeepEqual(t, "", string(c.Response.Body()))

	c.Reset()
	c.Render(consts.StatusNotModified, &render.Data{
		ContentType: "application/json; charset=utf-8",
		Data:        []byte("{\"test\":1}"),
	})
	assert.DeepEqual(t, "application/json; charset=utf-8", string(c.Response.Header.ContentType()))
	assert.DeepEqual(t, "", string(c.Response.Body()))
}

func TestHTML(t *testing.T) {
	c := NewContext(0)

	tmpl := template.Must(template.New("").
		Delims("{[{", "}]}").
		Funcs(template.FuncMap{}).
		ParseFiles("../common/testdata/template/index.tmpl"))

	r := &render.HTMLProduction{Template: tmpl}
	c.HTMLRender = r
	c.HTML(consts.StatusOK, "index.tmpl", utils.H{"title": "Main website"})

	assert.DeepEqual(t, []byte("text/html; charset=utf-8"), c.Response.Header.Peek("Content-Type"))
	assert.DeepEqual(t, []byte("<html><h1>Main website</h1></html>"), c.Response.Body())
}

type xmlmap map[string]interface{}

// Allows type H to be used with xml.Marshal
func (h xmlmap) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{
		Space: "",
		Local: "map",
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for key, value := range h {
		elem := xml.StartElement{
			Name: xml.Name{Space: "", Local: key},
			Attr: []xml.Attr{},
		}
		if err := e.EncodeElement(value, elem); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func TestXML(t *testing.T) {
	c := NewContext(0)
	c.XML(consts.StatusOK, xmlmap{"foo": "bar"})
	assert.DeepEqual(t, []byte("<map><foo>bar</foo></map>"), c.Response.Body())
	assert.DeepEqual(t, []byte("application/xml; charset=utf-8"), c.Response.Header.Peek("Content-Type"))
}

func TestJSON(t *testing.T) {
	c := NewContext(0)
	c.JSON(consts.StatusOK, "test")
	assert.DeepEqual(t, consts.StatusOK, c.Response.StatusCode())
	assert.True(t, strings.Contains(string(c.Response.Body()), "test"))
}

func TestDATA(t *testing.T) {
	c := NewContext(0)
	c.Data(consts.StatusOK, "application/json; charset=utf-8", []byte("{\"test\":1}"))
	assert.DeepEqual(t, consts.StatusOK, c.Response.StatusCode())
	assert.True(t, strings.Contains(string(c.Response.Body()), "test"))
}

func TestContextReset(t *testing.T) {
	c := NewContext(0)

	c.index = 2
	c.Params = param.Params{param.Param{}}
	c.Error(errors.New("test")) // nolint: errcheck
	c.Set("foo", "bar")
	c.Finished()
	c.Request.SetIsTLS(true)
	c.ResetWithoutConn()
	c.Request.URI()
	assert.DeepEqual(t, "https", string(c.Request.Scheme()))
	assert.False(t, c.IsAborted())
	assert.DeepEqual(t, 0, len(c.Errors))
	assert.Nil(t, c.Errors.Errors())
	assert.Nil(t, c.Errors.ByType(errs.ErrorTypeAny))
	assert.DeepEqual(t, 0, len(c.Params))
	assert.DeepEqual(t, int8(-1), c.index)
	assert.Nil(t, c.finished)
}

func TestContextContentType(t *testing.T) {
	c := NewContext(0)
	c.Request.Header.Set("Content-Type", consts.MIMEApplicationJSONUTF8)
	assert.DeepEqual(t, consts.MIMEApplicationJSONUTF8, bytesconv.B2s(c.ContentType()))
}

type MockIpConn struct {
	*mock.Conn
	RemoteIp string
	Port     int
}

func (c *MockIpConn) RemoteAddr() net.Addr {
	return &net.UDPAddr{
		IP:   net.ParseIP(c.RemoteIp),
		Port: c.Port,
	}
}

func newContextClientIPTest() *RequestContext {
	c := NewContext(0)
	c.conn = &MockIpConn{
		Conn:     mock.NewConn(""),
		RemoteIp: "127.0.0.1",
		Port:     8080,
	}
	c.Request.Header.Set("X-Real-IP", " 10.10.10.10  ")
	c.Request.Header.Set("X-Forwarded-For", "  20.20.20.20, 30.30.30.30")
	return c
}

func TestClientIp(t *testing.T) {
	c := newContextClientIPTest()
	// default X-Forwarded-For and X-Real-IP behaviour
	assert.DeepEqual(t, "20.20.20.20", c.ClientIP())

	c.Request.Header.DelBytes([]byte("X-Forwarded-For"))
	assert.DeepEqual(t, "10.10.10.10", c.ClientIP())

	c.Request.Header.Set("X-Forwarded-For", "30.30.30.30  ")
	assert.DeepEqual(t, "30.30.30.30", c.ClientIP())

	// No trusted CIDRS
	c = newContextClientIPTest()
	opts := ClientIPOptions{
		RemoteIPHeaders: []string{"X-Forwarded-For", "X-Real-IP"},
		TrustedCIDRs:    nil,
	}
	c.SetClientIPFunc(ClientIPWithOption(opts))
	assert.DeepEqual(t, "127.0.0.1", c.ClientIP())

	_, cidr, _ := net.ParseCIDR("30.30.30.30/32")
	opts = ClientIPOptions{
		RemoteIPHeaders: []string{"X-Forwarded-For", "X-Real-IP"},
		TrustedCIDRs:    []*net.IPNet{cidr},
	}
	c.SetClientIPFunc(ClientIPWithOption(opts))
	assert.DeepEqual(t, "127.0.0.1", c.ClientIP())

	_, cidr, _ = net.ParseCIDR("127.0.0.1/32")
	opts = ClientIPOptions{
		RemoteIPHeaders: []string{"X-Forwarded-For", "X-Real-IP"},
		TrustedCIDRs:    []*net.IPNet{cidr},
	}
	c.SetClientIPFunc(ClientIPWithOption(opts))
	assert.DeepEqual(t, "30.30.30.30", c.ClientIP())
}

func TestSetClientIPFunc(t *testing.T) {
	fn := func(ctx *RequestContext) string {
		return ""
	}
	SetClientIPFunc(fn)
	assert.DeepEqual(t, reflect.ValueOf(fn).Pointer(), reflect.ValueOf(defaultClientIP).Pointer())
}

func TestGetQuery(t *testing.T) {
	c := NewContext(0)
	c.Request.SetRequestURI("http://aaa.com?a=1&b=")
	v, exists := c.GetQuery("b")
	assert.DeepEqual(t, "", v)
	assert.DeepEqual(t, true, exists)
}

func TestGetPostForm(t *testing.T) {
	c := NewContext(0)
	c.Request.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationHTMLForm))
	c.Request.SetBodyString("a=1&b=")
	v, exists := c.GetPostForm("b")
	assert.DeepEqual(t, "", v)
	assert.DeepEqual(t, true, exists)
}

func TestRemoteAddr(t *testing.T) {
	c := NewContext(0)
	c.Request.SetRequestURI("http://aaa.com?a=1&b=")
	addr := c.RemoteAddr().String()
	assert.DeepEqual(t, "0.0.0.0:0", addr)
}

func TestRequestBodyStream(t *testing.T) {
	c := NewContext(0)
	s := "testRequestBodyStream"
	mr := bytes.NewBufferString(s)
	c.Request.SetBodyStream(mr, -1)
	data, err := ioutil.ReadAll(c.RequestBodyStream())
	assert.Nil(t, err)
	assert.DeepEqual(t, "testRequestBodyStream", string(data))
}

func TestContextIsAborted(t *testing.T) {
	ctx := NewContext(0)
	assert.False(t, ctx.IsAborted())

	ctx.Abort()
	assert.True(t, ctx.IsAborted())

	ctx.Next(context.Background())
	assert.True(t, ctx.IsAborted())

	ctx.index++
	assert.True(t, ctx.IsAborted())
}

func TestContextAbortWithStatus(t *testing.T) {
	c := NewContext(0)

	c.index = 4
	c.AbortWithStatus(consts.StatusUnauthorized)

	assert.DeepEqual(t, con.AbortIndex, c.index)
	assert.DeepEqual(t, consts.StatusUnauthorized, c.Response.Header.StatusCode())
	assert.True(t, c.IsAborted())
}

type testJSONAbortMsg struct {
	Foo string `json:"foo"`
	Bar string `json:"bar"`
}

func TestContextAbortWithStatusJSON(t *testing.T) {
	c := NewContext(0)
	c.index = 4

	in := new(testJSONAbortMsg)
	in.Bar = "barValue"
	in.Foo = "fooValue"

	c.AbortWithStatusJSON(consts.StatusUnsupportedMediaType, in)

	assert.DeepEqual(t, con.AbortIndex, c.index)
	assert.DeepEqual(t, consts.StatusUnsupportedMediaType, c.Response.Header.StatusCode())
	assert.True(t, c.IsAborted())

	contentType := c.Response.Header.Peek("Content-Type")
	assert.DeepEqual(t, consts.MIMEApplicationJSONUTF8, string(contentType))

	jsonStringBody := c.Response.Body()
	assert.DeepEqual(t, "{\"foo\":\"fooValue\",\"bar\":\"barValue\"}", string(jsonStringBody))
}

func TestRequestCtxFormValue(t *testing.T) {
	ctx := NewContext(0)
	ctx.Request.SetRequestURI("/foo/bar?baz=123&aaa=bbb")
	ctx.Request.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationHTMLForm))
	ctx.Request.SetBodyString("qqq=port&mmm=sddd")

	v := ctx.FormValue("baz")
	if string(v) != "123" {
		t.Fatalf("unexpected value %q. Expecting %q", v, "123")
	}
	v = ctx.FormValue("mmm")
	if string(v) != "sddd" {
		t.Fatalf("unexpected value %q. Expecting %q", v, "sddd")
	}
	v = ctx.FormValue("aaaasdfsdf")
	if len(v) > 0 {
		t.Fatalf("unexpected value for unknown key %q", v)
	}
	ctx.Request.Reset()
	ctx.Request.SetFormData(map[string]string{
		"a": "1",
	})
	v = ctx.FormValue("a")
	if string(v) != "1" {
		t.Fatalf("unexpected value %q. Expecting %q", v, "1")
	}

	ctx.Request.Reset()
	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f"

fff
------WebKitFormBoundaryJwfATyF8tmxSJnLg
`
	mr := bytes.NewBufferString(s)
	ctx.Request.SetBodyStream(mr, -1)
	ctx.Request.Header.SetContentLength(len(s))
	ctx.Request.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	v = ctx.FormValue("f")
	if string(v) != "fff" {
		t.Fatalf("unexpected value %q. Expecting %q", v, "fff")
	}
}

func TestSetCustomFormValueFunc(t *testing.T) {
	ctx := NewContext(0)
	ctx.Request.SetRequestURI("/foo/bar?aaa=bbb")
	ctx.Request.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationHTMLForm))
	ctx.Request.SetBodyString("aaa=port")

	ctx.SetFormValueFunc(func(ctx *RequestContext, key string) []byte {
		v := ctx.PostArgs().Peek(key)
		if len(v) > 0 {
			return v
		}
		mf, err := ctx.MultipartForm()
		if err == nil && mf.Value != nil {
			vv := mf.Value[key]
			if len(vv) > 0 {
				return []byte(vv[0])
			}
		}
		v = ctx.QueryArgs().Peek(key)
		if len(v) > 0 {
			return v
		}
		return nil
	})

	v := ctx.FormValue("aaa")
	if string(v) != "port" {
		t.Fatalf("unexpected value %q. Expecting %q", v, "port")
	}
}

func TestContextSetGet(t *testing.T) {
	c := &RequestContext{}
	c.Set("foo", "bar")

	value, err := c.Get("foo")
	assert.DeepEqual(t, "bar", value)
	assert.True(t, err)

	value, err = c.Get("foo2")
	assert.Nil(t, value)
	assert.False(t, err)

	assert.DeepEqual(t, "bar", c.MustGet("foo"))
	assert.Panic(t, func() { c.MustGet("no_exist") })
}

func TestContextSetGetValues(t *testing.T) {
	c := &RequestContext{}
	c.Set("string", "this is a string")
	c.Set("int32", int32(-42))
	c.Set("int64", int64(42424242424242))
	c.Set("uint32", uint32(42))
	c.Set("uint64", uint64(42424242424242))
	c.Set("float32", float32(4.2))
	c.Set("float64", 4.2)
	var a interface{} = 1
	c.Set("intInterface", a)

	assert.DeepEqual(t, c.MustGet("string").(string), "this is a string")
	assert.DeepEqual(t, c.MustGet("int32").(int32), int32(-42))
	assert.DeepEqual(t, c.MustGet("int64").(int64), int64(42424242424242))
	assert.DeepEqual(t, c.MustGet("uint32").(uint32), uint32(42))
	assert.DeepEqual(t, c.MustGet("uint64").(uint64), uint64(42424242424242))
	assert.DeepEqual(t, c.MustGet("float32").(float32), float32(4.2))
	assert.DeepEqual(t, c.MustGet("float64").(float64), 4.2)
	assert.DeepEqual(t, c.MustGet("intInterface").(int), 1)
}

func TestContextGetString(t *testing.T) {
	c := &RequestContext{}
	c.Set("string", "this is a string")
	assert.DeepEqual(t, "this is a string", c.GetString("string"))
	c.Set("bool", false)
	assert.DeepEqual(t, "", c.GetString("bool"))
}

func TestContextSetGetBool(t *testing.T) {
	c := &RequestContext{}
	c.Set("bool", true)
	assert.True(t, c.GetBool("bool"))
	c.Set("string", "this is a string")
	assert.False(t, c.GetBool("string"))
}

func TestContextGetInt(t *testing.T) {
	c := &RequestContext{}
	c.Set("int", 1)
	assert.DeepEqual(t, 1, c.GetInt("int"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, 0, c.GetInt("string"))
}

func TestContextGetInt32(t *testing.T) {
	c := &RequestContext{}
	c.Set("int32", int32(-42))
	assert.DeepEqual(t, int32(-42), c.GetInt32("int32"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, int32(0), c.GetInt32("string"))
}

func TestContextGetInt64(t *testing.T) {
	c := &RequestContext{}
	c.Set("int64", int64(42424242424242))
	assert.DeepEqual(t, int64(42424242424242), c.GetInt64("int64"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, int64(0), c.GetInt64("string"))
}

func TestContextGetUint(t *testing.T) {
	c := &RequestContext{}
	c.Set("uint", uint(1))
	assert.DeepEqual(t, uint(1), c.GetUint("uint"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, uint(0), c.GetUint("string"))
}

func TestContextGetUint32(t *testing.T) {
	c := &RequestContext{}
	c.Set("uint32", uint32(42))
	assert.DeepEqual(t, uint32(42), c.GetUint32("uint32"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, uint32(0), c.GetUint32("string"))
}

func TestContextGetUint64(t *testing.T) {
	c := &RequestContext{}
	c.Set("uint64", uint64(42424242424242))
	assert.DeepEqual(t, uint64(42424242424242), c.GetUint64("uint64"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, uint64(0), c.GetUint64("string"))
}

func TestContextGetFloat32(t *testing.T) {
	c := &RequestContext{}
	c.Set("float32", float32(4.2))
	assert.DeepEqual(t, float32(4.2), c.GetFloat32("float32"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, float32(0.0), c.GetFloat32("string"))
}

func TestContextGetFloat64(t *testing.T) {
	c := &RequestContext{}
	c.Set("float64", 4.2)
	assert.DeepEqual(t, 4.2, c.GetFloat64("float64"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, 0.0, c.GetFloat64("string"))
}

func TestContextGetTime(t *testing.T) {
	c := &RequestContext{}
	t1, _ := time.Parse("1/2/2006 15:04:05", "01/01/2017 12:00:00")
	c.Set("time", t1)
	assert.DeepEqual(t, t1, c.GetTime("time"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, time.Time{}, c.GetTime("string"))
}

func TestContextGetDuration(t *testing.T) {
	c := &RequestContext{}
	c.Set("duration", time.Second)
	assert.DeepEqual(t, time.Second, c.GetDuration("duration"))
	c.Set("string", "this is a string")
	assert.DeepEqual(t, time.Duration(0), c.GetDuration("string"))
}

func TestContextGetStringSlice(t *testing.T) {
	c := &RequestContext{}
	c.Set("slice", []string{"foo"})
	assert.DeepEqual(t, []string{"foo"}, c.GetStringSlice("slice"))
	c.Set("string", "this is a string")
	var expected []string
	assert.DeepEqual(t, expected, c.GetStringSlice("string"))
}

func TestContextGetStringMap(t *testing.T) {
	c := &RequestContext{}
	m := make(map[string]interface{})
	m["foo"] = 1
	c.Set("map", m)

	assert.DeepEqual(t, m, c.GetStringMap("map"))
	assert.DeepEqual(t, 1, c.GetStringMap("map")["foo"])

	c.Set("string", "this is a string")
	var expected map[string]interface{}
	assert.DeepEqual(t, expected, c.GetStringMap("string"))
}

func TestContextGetStringMapString(t *testing.T) {
	c := &RequestContext{}
	m := make(map[string]string)
	m["foo"] = "bar"
	c.Set("map", m)

	assert.DeepEqual(t, m, c.GetStringMapString("map"))
	assert.DeepEqual(t, "bar", c.GetStringMapString("map")["foo"])

	c.Set("string", "this is a string")
	var expected map[string]string
	assert.DeepEqual(t, expected, c.GetStringMapString("string"))
}

func TestContextGetStringMapStringSlice(t *testing.T) {
	c := &RequestContext{}
	m := make(map[string][]string)
	m["foo"] = []string{"foo"}
	c.Set("map", m)

	assert.DeepEqual(t, m, c.GetStringMapStringSlice("map"))
	assert.DeepEqual(t, []string{"foo"}, c.GetStringMapStringSlice("map")["foo"])

	c.Set("string", "this is a string")
	var expected map[string][]string
	assert.DeepEqual(t, expected, c.GetStringMapStringSlice("string"))
}

func TestContextTraceInfo(t *testing.T) {
	ctx := NewContext(0)
	traceIn := traceinfo.NewTraceInfo()
	ctx.SetTraceInfo(traceIn)
	traceOut := ctx.GetTraceInfo()

	assert.DeepEqual(t, traceIn, traceOut)
}

func TestEnableTrace(t *testing.T) {
	ctx := NewContext(0)
	ctx.SetEnableTrace(true)
	trace := ctx.IsEnableTrace()
	assert.True(t, trace)
}

func TestForEachKey(t *testing.T) {
	ctx := NewContext(0)
	ctx.Set("1", "2")
	handle := func(k string, v interface{}) {
		res := k + v.(string)
		assert.DeepEqual(t, res, "12")
	}
	ctx.ForEachKey(handle)
	val, ok := ctx.Get("1")
	assert.DeepEqual(t, val, "2")
	assert.True(t, ok)
}

func TestFlush(t *testing.T) {
	ctx := NewContext(0)
	err := ctx.Flush()
	assert.Nil(t, err)
}

func TestConn(t *testing.T) {
	ctx := NewContext(0)

	conn := mock.NewConn("")

	ctx.SetConn(conn)
	connRes := ctx.GetConn()

	val1 := reflect.ValueOf(conn).Pointer()
	val2 := reflect.ValueOf(connRes).Pointer()
	assert.DeepEqual(t, val1, val2)
}

func TestHijackHandler(t *testing.T) {
	ctx := NewContext(0)
	handle := func(c network.Conn) {
		c.SetReadTimeout(time.Duration(1) * time.Second)
	}
	ctx.SetHijackHandler(handle)
	handleRes := ctx.GetHijackHandler()

	val1 := reflect.ValueOf(handle).Pointer()
	val2 := reflect.ValueOf(handleRes).Pointer()
	assert.DeepEqual(t, val1, val2)
}

func TestGetReader(t *testing.T) {
	ctx := NewContext(0)

	conn := mock.NewConn("")

	ctx.SetConn(conn)
	connRes := ctx.GetReader()

	val1 := reflect.ValueOf(conn).Pointer()
	val2 := reflect.ValueOf(connRes).Pointer()
	assert.DeepEqual(t, val1, val2)
}

func TestGetWriter(t *testing.T) {
	ctx := NewContext(0)

	conn := mock.NewConn("")

	ctx.SetConn(conn)
	connRes := ctx.GetWriter()

	val1 := reflect.ValueOf(conn).Pointer()
	val2 := reflect.ValueOf(connRes).Pointer()
	assert.DeepEqual(t, val1, val2)
}

func TestIndex(t *testing.T) {
	ctx := NewContext(0)
	ctx.ResetWithoutConn()
	res := ctx.GetIndex()
	exc := int8(-1)
	assert.DeepEqual(t, exc, res)
}

func TestHandlerName(t *testing.T) {
	h := func(c context.Context, ctx *RequestContext) {}
	SetHandlerName(h, "test")
	name := GetHandlerName(h)
	assert.DeepEqual(t, "test", name)
}

func TestHijack(t *testing.T) {
	ctx := NewContext(0)
	h := func(c network.Conn) {}
	ctx.Hijack(h)
	assert.True(t, ctx.Hijacked())
}

func TestFinished(t *testing.T) {
	ctx := NewContext(0)
	ctx.Finished()

	ch := make(chan struct{})
	ctx.finished = ch
	chRes := ctx.Finished()

	send := func() {
		time.Sleep(time.Duration(1) * time.Millisecond)
		ch <- struct{}{}
	}
	go send()
	val := <-chRes
	assert.DeepEqual(t, struct{}{}, val)
}

func TestString(t *testing.T) {
	ctx := NewContext(0)
	ctx.String(consts.StatusOK, "ok")
	assert.DeepEqual(t, consts.StatusOK, ctx.Response.StatusCode())
}

func TestFullPath(t *testing.T) {
	ctx := NewContext(0)
	str := "/hello"
	ctx.SetFullPath(str)
	val := ctx.FullPath()
	assert.DeepEqual(t, str, val)
}

func TestReset(t *testing.T) {
	ctx := NewContext(0)
	ctx.Reset()
	assert.DeepEqual(t, nil, ctx.conn)
}

// func TestParam(t *testing.T) {
// 	ctx := NewContext(0)
// 	val := ctx.Param("/user/john")
// 	assert.DeepEqual(t, "john", val)
// }

func TestGetHeader(t *testing.T) {
	ctx := NewContext(0)
	ctx.Request.Header.SetContentTypeBytes([]byte(consts.MIMETextPlainUTF8))
	val := ctx.GetHeader("Content-Type")
	assert.DeepEqual(t, consts.MIMETextPlainUTF8, string(val))
}

func TestGetRawData(t *testing.T) {
	ctx := NewContext(0)
	ctx.Request.SetBody([]byte("hello"))
	val := ctx.GetRawData()
	assert.DeepEqual(t, "hello", string(val))

	val2, err := ctx.Body()
	assert.DeepEqual(t, val, val2)
	assert.Nil(t, err)
}

func TestRequestContext_GetRequest(t *testing.T) {
	c := &RequestContext{}
	c.Request.Header.Set("key1", "value1")
	c.Request.SetBody([]byte("test body"))
	req := c.GetRequest()
	if req.Header.Get("key1") != "value1" {
		t.Fatal("should have header: key1:value1")
	}
	if string(req.Body()) != "test body" {
		t.Fatal("should have body: test body")
	}
}

func TestRequestContext_GetResponse(t *testing.T) {
	c := &RequestContext{}
	c.Response.Header.Set("key1", "value1")
	c.Response.SetBody([]byte("test body"))
	resp := c.GetResponse()
	if resp.Header.Get("key1") != "value1" {
		t.Fatal("should have header: key1:value1")
	}
	if string(resp.Body()) != "test body" {
		t.Fatal("should have body: test body")
	}
}

func TestBindAndValidate(t *testing.T) {
	type Test struct {
		A string `query:"a"`
		B int    `query:"b" vd:"$>10"`
	}

	c := &RequestContext{}
	c.Request.SetRequestURI("/foo/bar?a=123&b=11")

	var req Test
	err := c.BindAndValidate(&req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "123", req.A)
	assert.DeepEqual(t, 11, req.B)

	c.Request.URI().Reset()
	c.Request.SetRequestURI("/foo/bar?a=123&b=9")
	req = Test{}
	err = c.BindAndValidate(&req)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}

	c.Request.URI().Reset()
	c.Request.SetRequestURI("/foo/bar?a=123&b=9")
	req = Test{}
	err = c.Bind(&req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "123", req.A)
	assert.DeepEqual(t, 9, req.B)

	err = c.Validate(&req)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
}

func TestRequestContext_SetCookie(t *testing.T) {
	c := NewContext(0)
	c.SetCookie("user", "hertz", 1, "/", "localhost", protocol.CookieSameSiteLaxMode, true, true)
	assert.DeepEqual(t, "user=hertz; max-age=1; domain=localhost; path=/; HttpOnly; secure; SameSite=Lax", c.Response.Header.Get("Set-Cookie"))
}

func TestRequestContext_SetCookiePathEmpty(t *testing.T) {
	c := NewContext(0)
	c.SetCookie("user", "hertz", 1, "", "localhost", protocol.CookieSameSiteDisabled, true, true)
	assert.DeepEqual(t, "user=hertz; max-age=1; domain=localhost; path=/; HttpOnly; secure", c.Response.Header.Get("Set-Cookie"))
}

func TestRequestContext_VisitAll(t *testing.T) {
	t.Run("VisitAllQueryArgs", func(t *testing.T) {
		c := NewContext(0)
		var s []string
		c.QueryArgs().Add("cloudwego", "hertz")
		c.QueryArgs().Add("hello", "world")
		c.VisitAllQueryArgs(func(key, value []byte) {
			s = append(s, string(key), string(value))
		})
		assert.DeepEqual(t, []string{"cloudwego", "hertz", "hello", "world"}, s)
	})

	t.Run("VisitAllPostArgs", func(t *testing.T) {
		c := NewContext(0)
		var s []string
		c.PostArgs().Add("cloudwego", "hertz")
		c.PostArgs().Add("hello", "world")
		c.VisitAllPostArgs(func(key, value []byte) {
			s = append(s, string(key), string(value))
		})
		assert.DeepEqual(t, []string{"cloudwego", "hertz", "hello", "world"}, s)
	})

	t.Run("VisitAllCookie", func(t *testing.T) {
		c := NewContext(0)
		var s []string
		c.Request.Header.Set("Cookie", "aaa=bbb;ccc=ddd")
		c.VisitAllCookie(func(key, value []byte) {
			s = append(s, string(key), string(value))
		})
		assert.DeepEqual(t, []string{"aaa", "bbb", "ccc", "ddd"}, s)
	})

	t.Run("VisitAllHeaders", func(t *testing.T) {
		c := NewContext(0)
		c.Request.Header.Set("xxx", "yyy")
		c.Request.Header.Set("xxx2", "yyy2")
		c.VisitAllHeaders(
			func(k, v []byte) {
				key := string(k)
				value := string(v)
				if key != "Xxx" && key != "Xxx2" {
					t.Fatalf("Unexpected %v. Expected %v", key, "xxx or yyy")
				}
				if key == "Xxx" && value != "yyy" {
					t.Fatalf("Unexpected %v. Expected %v", value, "yyy")
				}
				if key == "Xxx2" && value != "yyy2" {
					t.Fatalf("Unexpected %v. Expected %v", value, "yyy2")
				}
			})
	})
}
