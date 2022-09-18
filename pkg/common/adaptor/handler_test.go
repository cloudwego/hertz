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
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestNewHertzHandler(t *testing.T) {
	t.Parallel()

	expectedMethod := consts.MethodPost
	expectedProto := "HTTP/1.1"
	expectedProtoMajor := 1
	expectedProtoMinor := 1
	expectedRequestURI := "http://foobar.com/foo/bar?baz=123"
	expectedBody := "<!doctype html><html>"
	expectedContentLength := len(expectedBody)
	expectedHost := "foobar.com"
	expectedHeader := map[string]string{
		"Foo-Bar":         "baz",
		"Abc":             "defg",
		"XXX-Remote-Addr": "123.43.4543.345",
	}
	expectedURL, err := url.ParseRequestURI(expectedRequestURI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedContextKey := "contextKey"
	expectedContextValue := "contextValue"
	expectedContentType := "text/html; charset=utf-8"

	callsCount := 0
	nethttpH := func(w http.ResponseWriter, r *http.Request) {
		callsCount++
		assert.Assertf(t, r.Method == expectedMethod, "unexpected method %q. Expecting %q", r.Method, expectedMethod)
		assert.Assertf(t, r.Proto == expectedProto, "unexpected proto %q. Expecting %q", r.Proto, expectedProto)
		assert.Assertf(t, r.ProtoMajor == expectedProtoMajor, "unexpected protoMajor %d. Expecting %d", r.ProtoMajor, expectedProtoMajor)
		assert.Assertf(t, r.ProtoMinor == expectedProtoMinor, "unexpected protoMinor %d. Expecting %d", r.ProtoMinor, expectedProtoMinor)
		assert.Assertf(t, r.RequestURI == expectedRequestURI, "unexpected requestURI %q. Expecting %q", r.RequestURI, expectedRequestURI)
		assert.Assertf(t, r.ContentLength == int64(expectedContentLength), "unexpected contentLength %d. Expecting %d", r.ContentLength, expectedContentLength)
		assert.Assertf(t, len(r.TransferEncoding) == 0, "unexpected transferEncoding %q. Expecting []", r.TransferEncoding)
		assert.Assertf(t, r.Host == expectedHost, "unexpected host %q. Expecting %q", r.Host, expectedHost)
		body, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			t.Fatalf("unexpected error when reading request body: %v", err)
		}
		assert.Assertf(t, string(body) == expectedBody, "unexpected body %q. Expecting %q", body, expectedBody)
		assert.Assertf(t, reflect.DeepEqual(r.URL, expectedURL), "unexpected URL: %#v. Expecting %#v", r.URL, expectedURL)
		assert.Assertf(t, r.Context().Value(expectedContextKey) == expectedContextValue,
			"unexpected context value for key %q. Expecting %q, in fact: %v", expectedContextKey,
			expectedContextValue, r.Context().Value(expectedContextKey))
		for k, expectedV := range expectedHeader {
			v := r.Header.Get(k)
			if v != expectedV {
				t.Fatalf("unexpected header value %q for key %q. Expecting %q", v, k, expectedV)
			}
		}
		w.Header().Set("Header1", "value1")
		w.Header().Set("Header2", "value2")
		w.WriteHeader(http.StatusBadRequest) // nolint:errcheck
		w.Write(body)
	}
	hertzH := NewHertzHTTPHandler(http.HandlerFunc(nethttpH))
	hertzH = setContextValueMiddleware(hertzH, expectedContextKey, expectedContextValue)
	var ctx app.RequestContext
	var req protocol.Request
	req.Header.SetMethod(expectedMethod)
	req.SetRequestURI(expectedRequestURI)
	req.Header.SetHost(expectedHost)
	req.BodyWriter().Write([]byte(expectedBody)) // nolint:errcheck
	for k, v := range expectedHeader {
		req.Header.Set(k, v)
	}
	req.CopyTo(&ctx.Request)
	hertzH(context.Background(), &ctx)
	assert.Assertf(t, callsCount == 1, "unexpected callsCount: %d. Expecting 1", callsCount)
	resp := &ctx.Response
	assert.Assertf(t, resp.StatusCode() == http.StatusBadRequest, "unexpected statusCode: %d. Expecting %d", resp.StatusCode(), http.StatusBadRequest)
	assert.Assertf(t, string(resp.Header.Peek("Header1")) == "value1", "unexpected header value: %q. Expecting %q", resp.Header.Peek("Header1"), "value1")
	assert.Assertf(t, string(resp.Header.Peek("Header2")) == "value2", "unexpected header value: %q. Expecting %q", resp.Header.Peek("Header2"), "value2")
	assert.Assertf(t, string(resp.Body()) == expectedBody, "unexpected response body %q. Expecting %q", resp.Body(), expectedBody)
	assert.Assertf(t, string(resp.Header.Peek("Content-Type")) == expectedContentType, "unexpected content-type %q. Expecting %q", string(resp.Header.Peek("Content-Type")), expectedContentType)
}

func setContextValueMiddleware(next app.HandlerFunc, key string, value interface{}) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Set(key, value)
		next(ctx, c)
	}
}
