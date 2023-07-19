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
	"mime/multipart"
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

func TestBindAndValidate(t *testing.T) {
	type TestBind struct {
		A string               `query:"a"`
		B []string             `query:"b"`
		C string               `query:"c"`
		D string               `header:"d"`
		E string               `path:"e"`
		F string               `form:"f"`
		G multipart.FileHeader `form:"g"`
		H string               `cookie:"h"`
	}

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f"

fff
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="g"; filename="TODO"
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
	r.SetRequestURI("/foo/bar?a=aaa&b=b1&b=b2&c&i=19")
	r.SetHeader("d", "ddd")
	r.Header.SetContentLength(len(s))
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	r.SetCookie("h", "hhh")

	para := param.Params{
		{Key: "e", Value: "eee"},
	}

	// test BindAndValidate()
	SetLooseZeroMode(true)
	var req TestBind
	err := BindAndValidate(r, &req, para)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "aaa", req.A)
	assert.DeepEqual(t, 2, len(req.B))
	assert.DeepEqual(t, "", req.C)
	assert.DeepEqual(t, "ddd", req.D)
	assert.DeepEqual(t, "eee", req.E)
	assert.DeepEqual(t, "fff", req.F)
	assert.DeepEqual(t, "TODO", req.G.Filename)
	assert.DeepEqual(t, "hhh", req.H)

	// test Bind()
	req = TestBind{}
	err = Bind(r, &req, para)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "aaa", req.A)
	assert.DeepEqual(t, 2, len(req.B))
	assert.DeepEqual(t, "", req.C)
	assert.DeepEqual(t, "ddd", req.D)
	assert.DeepEqual(t, "eee", req.E)
	assert.DeepEqual(t, "fff", req.F)
	assert.DeepEqual(t, "TODO", req.G.Filename)
	assert.DeepEqual(t, "hhh", req.H)

	type TestValidate struct {
		I int `query:"i" vd:"$>20"`
	}

	// test BindAndValidate()
	var bindReq TestValidate
	err = BindAndValidate(r, &bindReq, para)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}

	// test Validate()
	bindReq = TestValidate{}
	err = Bind(r, &bindReq, para)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, 19, bindReq.I)
	err = Validate(&bindReq)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
}

func TestJsonBind(t *testing.T) {
	type Test struct {
		A string   `json:"a"`
		B []string `json:"b"`
		C string   `json:"c"`
		D int      `json:"d,string"`
	}

	data := `{"a":"aaa", "b":["b1","b2"], "c":"ccc", "d":"100"}`
	mr := bytes.NewBufferString(data)
	r := protocol.NewRequest("POST", "/foo", mr)
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.SetHeader("d", "ddd")
	r.Header.SetContentLength(len(data))

	var req Test
	err := BindAndValidate(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "aaa", req.A)
	assert.DeepEqual(t, 2, len(req.B))
	assert.DeepEqual(t, "ccc", req.C)
	// NOTE: The default does not support string to go int conversion in json.
	// You can add "string" tags or use other json unmarshal libraries that support this feature
	assert.DeepEqual(t, 100, req.D)

	req = Test{}
	UseGJSONUnmarshaler()
	err = BindAndValidate(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "aaa", req.A)
	assert.DeepEqual(t, 2, len(req.B))
	assert.DeepEqual(t, "ccc", req.C)
	// NOTE: The default does not support string to go int conversion in json.
	// You can add "string" tags or use other json unmarshal libraries that support this feature
	assert.DeepEqual(t, 100, req.D)

	req = Test{}
	UseStdJSONUnmarshaler()
	err = BindAndValidate(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "aaa", req.A)
	assert.DeepEqual(t, 2, len(req.B))
	assert.DeepEqual(t, "ccc", req.C)
	// NOTE: The default does not support string to go int conversion in json.
	// You can add "string" tags or use other json unmarshal libraries that support this feature
	assert.DeepEqual(t, 100, req.D)
}

// TestQueryParamInconsistency tests the Inconsistency for GetQuery(), the other unit test for GetFunc() in request.go  are similar to it
func TestQueryParamInconsistency(t *testing.T) {
	type QueryPara struct {
		Para1 string  `query:"para1"`
		Para2 *string `query:"para2"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?para1=hertz&para2=binding")

	var req QueryPara
	err := BindAndValidate(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	beforePara1 := deepCopyString(req.Para1)
	beforePara2 := deepCopyString(*req.Para2)
	r.URI().QueryArgs().Set("para1", "test")
	r.URI().QueryArgs().Set("para2", "test")
	afterPara1 := req.Para1
	afterPara2 := *req.Para2
	assert.DeepEqual(t, beforePara1, afterPara1)
	assert.DeepEqual(t, beforePara2, afterPara2)
}

func deepCopyString(str string) string {
	tmp := make([]byte, len(str))
	copy(tmp, str)
	c := string(tmp)

	return c
}

func TestBindingFile(t *testing.T) {
	type FileParas struct {
		F   *multipart.FileHeader `form:"F1"`
		F1  multipart.FileHeader
		Fs  []multipart.FileHeader  `form:"F1"`
		Fs1 []*multipart.FileHeader `form:"F1"`
		F2  *multipart.FileHeader   `form:"F2"`
	}

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
	r.SetRequestURI("/foo/bar?a=aaa&b=b1&b=b2&c&i=19")
	r.SetHeader("d", "ddd")
	r.Header.SetContentLength(len(s))
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	var req FileParas
	err := BindAndValidate(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "TODO1", req.F.Filename)
	assert.DeepEqual(t, "TODO1", req.F1.Filename)
	assert.DeepEqual(t, 2, len(req.Fs))
	assert.DeepEqual(t, 2, len(req.Fs1))
	assert.DeepEqual(t, "TODO3", req.F2.Filename)
}

type BindError struct {
	ErrType, FailField, Msg string
}

// Error implements error interface.
func (e *BindError) Error() string {
	if e.Msg != "" {
		return e.ErrType + ": expr_path=" + e.FailField + ", cause=" + e.Msg
	}
	return e.ErrType + ": expr_path=" + e.FailField + ", cause=invalid"
}

type ValidateError struct {
	ErrType, FailField, Msg string
}

// Error implements error interface.
func (e *ValidateError) Error() string {
	if e.Msg != "" {
		return e.ErrType + ": expr_path=" + e.FailField + ", cause=" + e.Msg
	}
	return e.ErrType + ": expr_path=" + e.FailField + ", cause=invalid"
}

func TestSetErrorFactory(t *testing.T) {
	type TestBind struct {
		A string `query:"a,required"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?b=20")

	CustomBindErrFunc := func(failField, msg string) error {
		err := BindError{
			ErrType:   "bindErr",
			FailField: "[bindFailField]: " + failField,
			Msg:       "[bindErrMsg]: " + msg,
		}

		return &err
	}

	CustomValidateErrFunc := func(failField, msg string) error {
		err := ValidateError{
			ErrType:   "validateErr",
			FailField: "[validateFailField]: " + failField,
			Msg:       "[validateErrMsg]: " + msg,
		}

		return &err
	}

	SetErrorFactory(CustomBindErrFunc, CustomValidateErrFunc)

	var req TestBind
	err := Bind(r, &req, nil)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
	assert.DeepEqual(t, "bindErr: expr_path=[bindFailField]: A, cause=[bindErrMsg]: missing required parameter", err.Error())

	type TestValidate struct {
		B int `query:"b" vd:"$>100"`
	}

	var reqValidate TestValidate
	err = Bind(r, &reqValidate, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = Validate(&reqValidate)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
	assert.DeepEqual(t, "validateErr: expr_path=[validateFailField]: B, cause=[validateErrMsg]: ", err.Error())
}

func TestMustRegTypeUnmarshal(t *testing.T) {
	type Nested struct {
		B string
		C string
	}

	type TestBind struct {
		A Nested `query:"a,required"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?a=hertzbinding")

	MustRegTypeUnmarshal(reflect.TypeOf(Nested{}), func(v string, emptyAsZero bool) (reflect.Value, error) {
		if v == "" && emptyAsZero {
			return reflect.ValueOf(Nested{}), nil
		}
		val := Nested{
			B: v[:5],
			C: v[5:],
		}
		return reflect.ValueOf(val), nil
	})

	var req TestBind
	err := Bind(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.DeepEqual(t, "hertz", req.A.B)
	assert.DeepEqual(t, "binding", req.A.C)
}

func TestMustRegValidateFunc(t *testing.T) {
	type TestValidate struct {
		A string `query:"a" vd:"test($)"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?a=123")

	MustRegValidateFunc("test", func(args ...interface{}) error {
		if len(args) != 1 {
			return fmt.Errorf("the args must be one")
		}
		s, _ := args[0].(string)
		if s == "123" {
			return fmt.Errorf("the args can not be 123")
		}
		return nil
	})

	var req TestValidate
	err := Bind(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = Validate(&req)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
}

func TestQueryAlias(t *testing.T) {
	type MyInt int
	type MyString string
	type MyIntSlice []int
	type MyStringSlice []string
	type Test struct {
		A []MyInt       `query:"a"`
		B MyIntSlice    `query:"b"`
		C MyString      `query:"c"`
		D MyStringSlice `query:"d"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?a=1&a=2&b=2&b=3&c=string1&d=string2&d=string3")

	var req Test
	err := Bind(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	assert.DeepEqual(t, 2, len(req.A))
	assert.DeepEqual(t, 1, int(req.A[0]))
	assert.DeepEqual(t, 2, int(req.A[1]))
	assert.DeepEqual(t, 2, len(req.B))
	assert.DeepEqual(t, 2, req.B[0])
	assert.DeepEqual(t, 3, req.B[1])
	assert.DeepEqual(t, "string1", string(req.C))
	assert.DeepEqual(t, 2, len(req.D))
	assert.DeepEqual(t, "string2", req.D[0])
	assert.DeepEqual(t, "string3", req.D[1])
}
