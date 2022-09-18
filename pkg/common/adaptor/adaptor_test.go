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
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

func TestCompatResponse_WriteHeader(t *testing.T) {
	var testHeader http.Header
	var testBody string
	testUrl := "http://127.0.0.1:9000/test"
	testStatusCode := 299

	testHeader = make(map[string][]string)
	testHeader["Key1"] = []string{"value1"}
	testHeader["Key2"] = []string{"value2", "value22"}
	testHeader["Key3"] = []string{"value3", "value33", "value333"}

	testBody = "test body"

	h := server.New(server.WithHostPorts("127.0.0.1:9000"))
	h.POST("/test", func(c context.Context, ctx *app.RequestContext) {
		req, _ := GetCompatRequest(&ctx.Request)
		resp := GetCompatResponseWriter(&ctx.Response)
		handlerAndCheck(t, resp, req, testHeader, testBody, testStatusCode)
	})

	go h.Spin()
	time.Sleep(200 * time.Millisecond)

	makeACall(t, http.MethodPost, testUrl, testHeader, testBody, testStatusCode)
}

func makeACall(t *testing.T, method, url string, header http.Header, body string, expectStatusCode int) {
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
	if string(b) != body {
		t.Fatalf("Body not equal: want: %s, got: %s", body, string(b))
	}

	if resp.StatusCode != expectStatusCode {
		t.Fatalf("Status code not equal: want: %d, got: %d", expectStatusCode, resp.StatusCode)
	}
}

func handlerAndCheck(t *testing.T, writer http.ResponseWriter, request *http.Request, wantHeader http.Header, wantBody string, statusCode int) {
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
	if string(body) != wantBody {
		t.Fatalf("Body not equal: want: %s, got: %s", wantBody, string(body))
	}

	respHeader := writer.Header()
	for k, v := range reqHeader {
		respHeader[k] = v
	}
	writer.WriteHeader(statusCode)
	_, err = writer.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write body error: %s", err)
	}
	_, err = writer.Write([]byte(" body"))
	if err != nil {
		t.Fatalf("Write body error: %s", err)
	}
}

func TestNewHertzHTTPHandler(t *testing.T) {
	var headers []ut.Header
	opt := config.NewOptions([]config.Option{})
	engine := route.NewEngine(opt)

	expectedValue := "success"
	expectedKey := "Authorization"
	expectedJson := []byte("{\"hi\":\"version1\"}")
	expectedContentLength := len(expectedJson)
	expectedCode := 200

	nethttpH := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		assert.DeepEqual(t, r.Header.Get("Test1"), "test")
		w.Header().Set(expectedKey, expectedValue)
		w.Write(expectedJson)
	}
	nethttpH2 := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		w.Write(body)
	}
	engine.GET("/get", NewHertzHTTPHandlerFunc(nethttpH))
	engine.POST("/post", NewHertzHTTPHandlerFunc(nethttpH2))
	headers = []ut.Header{
		{Key: "Content-Type", Value: "application/json"},
		{Key: "Test1", Value: "test"},
	}
	w := ut.PerformRequest(engine, "GET", "/get", nil, headers...)
	res := w.Result()
	assert.DeepEqual(t, expectedCode, res.StatusCode())
	assert.DeepEqual(t, expectedJson, res.Body())
	assert.DeepEqual(t, expectedValue, res.Header.Get(expectedKey))
	assert.DeepEqual(t, expectedContentLength, res.Header.ContentLength())

	w2 := ut.PerformRequest(engine, "POST", "/post", &ut.Body{
		Body: bytes.NewBuffer(expectedJson),
		Len:  len(expectedJson),
	}, headers...)
	res2 := w2.Result()

	assert.DeepEqual(t, expectedCode, res2.StatusCode())
	assert.DeepEqual(t, expectedJson, res2.Body())
	assert.DeepEqual(t, expectedContentLength, res2.Header.ContentLength())
}
