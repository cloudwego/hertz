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

package adaptor

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestCompatResponse_WriteHeader(t *testing.T) {
	var testHeader http.Header
	var testBody string
	testUrl1 := "http://127.0.0.1:9000/test1"
	testUrl2 := "http://127.0.0.1:9000/test2"
	testStatusCode := 299
	testCookieValue := "cookie"

	testHeader = make(map[string][]string)
	testHeader["Key1"] = []string{"value1"}
	testHeader["Key2"] = []string{"value2", "value22"}
	testHeader["Key3"] = []string{"value3", "value33", "value333"}
	testHeader[consts.HeaderSetCookie] = []string{testCookieValue}

	testBody = "test body"

	h := server.New(server.WithHostPorts("127.0.0.1:9000"))
	h.POST("/test1", func(c context.Context, ctx *app.RequestContext) {
		req, _ := GetCompatRequest(&ctx.Request)
		resp := GetCompatResponseWriter(&ctx.Response)
		handlerAndCheck(t, resp, req, testHeader, testBody, testStatusCode)
	})

	h.POST("/test2", func(c context.Context, ctx *app.RequestContext) {
		req, _ := GetCompatRequest(&ctx.Request)
		resp := GetCompatResponseWriter(&ctx.Response)
		handlerAndCheck(t, resp, req, testHeader, testBody)
	})

	go h.Spin()
	time.Sleep(200 * time.Millisecond)

	makeACall(t, http.MethodPost, testUrl1, testHeader, testBody, testStatusCode, []byte(testCookieValue))
	makeACall(t, http.MethodPost, testUrl2, testHeader, testBody, consts.StatusOK, []byte(testCookieValue))
}

func makeACall(t *testing.T, method, url string, header http.Header, body string, expectStatusCode int, expectCookieValue []byte) {
	client := http.Client{}
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header = header
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("make a call error: %s", err)
	}

	respHeader := resp.Header

	for k, v := range header {
		for i := 0; i < len(v); i++ {
			if respHeader[k][i] != v[i] {
				t.Fatalf("Header error: want %s=%s, got %s=%s", respHeader[k], respHeader[k][i], respHeader[k], v[i])
			}
		}
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Read body error: %s", err)
	}
	assert.DeepEqual(t, body, string(b))
	assert.DeepEqual(t, expectStatusCode, resp.StatusCode)

	// Parse out the cookie to verify it is correct
	cookie := protocol.Cookie{}
	_ = cookie.Parse(header[consts.HeaderSetCookie][0])
	assert.DeepEqual(t, expectCookieValue, cookie.Value())
}

// handlerAndCheck is designed to handle the program and check the header
//
// "..." is used in the type of statusCode, which is a syntactic sugar in Go.
// In this way, the statusCode can be made an optional parameter,
// and there is no need to pass in some meaningless numbers to judge some special cases.
func handlerAndCheck(t *testing.T, writer http.ResponseWriter, request *http.Request, wantHeader http.Header, wantBody string, statusCode ...int) {
	reqHeader := request.Header
	for k, v := range wantHeader {
		if reqHeader[k] == nil {
			t.Fatalf("Header error: want %s=%s, got %s=nil", reqHeader[k], reqHeader[k][0], reqHeader[k])
		}
		if reqHeader[k][0] != v[0] {
			t.Fatalf("Header error: want %s=%s, got %s=%s", reqHeader[k], reqHeader[k][0], reqHeader[k], v[0])
		}
	}

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		t.Fatalf("Read body error: %s", err)
	}
	assert.DeepEqual(t, wantBody, string(body))

	respHeader := writer.Header()
	for k, v := range reqHeader {
		respHeader[k] = v
	}

	// When the incoming status code is nil, the execution of this code is skipped
	// and the status code is set to 200
	if statusCode != nil {
		writer.WriteHeader(statusCode[0])
	}

	_, err = writer.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write body error: %s", err)
	}
	_, err = writer.Write([]byte(" body"))
	if err != nil {
		t.Fatalf("Write body error: %s", err)
	}
}

func TestCopyToHertzRequest(t *testing.T) {
	req := http.Request{
		Method:     "GET",
		RequestURI: "/test",
		URL: &url.URL{
			Scheme: "http",
			Host:   "test.com",
		},
		Proto:  "HTTP/1.1",
		Header: http.Header{},
	}
	req.Header.Set("key1", "value1")
	req.Header.Add("key2", "value2")
	req.Header.Add("key2", "value22")
	hertzReq := protocol.Request{}
	err := CopyToHertzRequest(&req, &hertzReq)
	assert.Nil(t, err)
	assert.DeepEqual(t, req.Method, string(hertzReq.Method()))
	assert.DeepEqual(t, req.RequestURI, string(hertzReq.Path()))
	assert.DeepEqual(t, req.Proto, hertzReq.Header.GetProtocol())
	assert.DeepEqual(t, req.Header.Get("key1"), hertzReq.Header.Get("key1"))
	valueSlice := make([]string, 0, 2)
	hertzReq.Header.VisitAllCustomHeader(func(key, value []byte) {
		if strings.ToLower(string(key)) == "key2" {
			valueSlice = append(valueSlice, string(value))
		}
	})

	assert.DeepEqual(t, req.Header.Values("key2"), valueSlice)

	assert.DeepEqual(t, 3, hertzReq.Header.Len())
}
