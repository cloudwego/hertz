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

package binding

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestGetQuery(t *testing.T) {
	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?para1=hertz&para2=query1&para2=query2&para3=1&para3=2")

	bindReq := bindRequest{
		req: r,
	}

	values := bindReq.GetQuery()

	assert.DeepEqual(t, []string{"hertz"}, values["para1"])
	assert.DeepEqual(t, []string{"query1", "query2"}, values["para2"])
	assert.DeepEqual(t, []string{"1", "2"}, values["para3"])
}

func TestGetPostForm(t *testing.T) {
	data := "a=aaa&b=b1&b=b2&c=ccc&d=100"
	mr := bytes.NewBufferString(data)

	r := protocol.NewRequest("POST", "/foo", mr)
	r.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationHTMLForm))
	r.Header.SetContentLength(len(data))

	bindReq := bindRequest{
		req: r,
	}

	values, err := bindReq.GetPostForm()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.DeepEqual(t, []string{"aaa"}, values["a"])
	assert.DeepEqual(t, []string{"b1", "b2"}, values["b"])
	assert.DeepEqual(t, []string{"ccc"}, values["c"])
	assert.DeepEqual(t, []string{"100"}, values["d"])
}

func TestGetForm(t *testing.T) {
	data := "a=aaa&b=b1&b=b2&c=ccc&d=100"
	mr := bytes.NewBufferString(data)

	r := protocol.NewRequest("POST", "/foo", mr)
	r.SetRequestURI("/foo/bar?para1=hertz&para2=query1&para2=query2&para3=1&para3=2")
	r.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationHTMLForm))
	r.Header.SetContentLength(len(data))

	bindReq := bindRequest{
		req: r,
	}

	values, err := bindReq.GetForm()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.DeepEqual(t, []string{"aaa"}, values["a"])
	assert.DeepEqual(t, []string{"b1", "b2"}, values["b"])
	assert.DeepEqual(t, []string{"ccc"}, values["c"])
	assert.DeepEqual(t, []string{"100"}, values["d"])
	assert.DeepEqual(t, []string{"hertz"}, values["para1"])
	assert.DeepEqual(t, []string{"query1", "query2"}, values["para2"])
	assert.DeepEqual(t, []string{"1", "2"}, values["para3"])
}

func TestGetCookies(t *testing.T) {
	r := protocol.NewRequest("POST", "/foo", nil)
	r.SetCookie("cookie1", "cookies1")
	r.SetCookie("cookie2", "cookies2")

	bindReq := bindRequest{
		req: r,
	}

	values := bindReq.GetCookies()

	assert.DeepEqual(t, "cookies1", values[0].Value)
	assert.DeepEqual(t, "cookies2", values[1].Value)
}

func TestGetHeader(t *testing.T) {
	headers := map[string]string{
		"Header1": "headers1",
		"Header2": "headers2",
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetHeaders(headers)
	r.SetHeader("Header3", "headers3")

	bindReq := bindRequest{
		req: r,
	}

	values := bindReq.GetHeader()

	assert.DeepEqual(t, []string{"headers1"}, values["Header1"])
	assert.DeepEqual(t, []string{"headers2"}, values["Header2"])
	assert.DeepEqual(t, []string{"headers3"}, values["Header3"])
}

func TestGetMethod(t *testing.T) {
	r := protocol.NewRequest("GET", "/foo", nil)

	bindReq := bindRequest{
		req: r,
	}

	values := bindReq.GetMethod()

	assert.DeepEqual(t, "GET", values)
}

func TestGetContentType(t *testing.T) {
	data := "a=aaa&b=b1&b=b2&c=ccc&d=100"
	mr := bytes.NewBufferString(data)

	r := protocol.NewRequest("POST", "/foo", mr)
	r.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationHTMLForm))
	r.Header.SetContentLength(len(data))

	bindReq := bindRequest{
		req: r,
	}

	values := bindReq.GetContentType()

	assert.DeepEqual(t, consts.MIMEApplicationHTMLForm, values)
}

func TestGetBody(t *testing.T) {
	data := "a=aaa&b=b1&b=b2&c=ccc&d=100"
	mr := bytes.NewBufferString(data)

	r := protocol.NewRequest("POST", "/foo", mr)
	r.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationHTMLForm))
	r.Header.SetContentLength(len(data))

	bindReq := bindRequest{
		req: r,
	}

	values, err := bindReq.GetBody()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.DeepEqual(t, []byte("a=aaa&b=b1&b=b2&c=ccc&d=100"), values)
}

func TestGetFileHeaders(t *testing.T) {
	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f"

fff
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="F1"; filename="TODO1"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="F1"; filename="TODO2"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="F2"; filename="TODO3"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`

	mr := bytes.NewBufferString(s)

	r := protocol.NewRequest("POST", "/foo", mr)
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))
	r.Header.SetContentLength(len(s))

	bindReq := bindRequest{
		req: r,
	}

	values, err := bindReq.GetFileHeaders()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fmt.Printf("%v\n", values)

	assert.DeepEqual(t, "TODO1", values["F1"][0].Filename)
	assert.DeepEqual(t, "TODO2", values["F1"][1].Filename)
	assert.DeepEqual(t, "TODO3", values["F2"][0].Filename)
}
