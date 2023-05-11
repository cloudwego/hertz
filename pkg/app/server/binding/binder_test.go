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
 * MIT License
 *
 * Copyright (c) 2019-present Fenny and Contributors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package binding

import (
	"fmt"
	"mime/multipart"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server/binding/path"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	req2 "github.com/cloudwego/hertz/pkg/protocol/http1/req"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type mockRequest struct {
	Req *protocol.Request
}

func newMockRequest() *mockRequest {
	return &mockRequest{
		Req: &protocol.Request{},
	}
}

func (m *mockRequest) SetRequestURI(uri string) *mockRequest {
	m.Req.SetRequestURI(uri)
	return m
}

func (m *mockRequest) SetFile(param, fileName string) *mockRequest {
	m.Req.SetFile(param, fileName)
	return m
}

func (m *mockRequest) SetHeader(key, value string) *mockRequest {
	m.Req.Header.Set(key, value)
	return m
}

func (m *mockRequest) SetHeaders(key, value string) *mockRequest {
	m.Req.Header.Set(key, value)
	return m
}

func (m *mockRequest) SetPostArg(key, value string) *mockRequest {
	m.Req.PostArgs().Add(key, value)
	return m
}

func (m *mockRequest) SetUrlEncodeContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte("application/x-www-form-urlencoded"))
	return m
}

func (m *mockRequest) SetJSONContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte(jsonContentType))
	return m
}

func (m *mockRequest) SetBody(data []byte) *mockRequest {
	m.Req.SetBody(data)
	m.Req.Header.SetContentLength(len(data))
	return m
}

func TestBind_BaseType(t *testing.T) {
	type Req struct {
		Version int    `path:"v"`
		ID      int    `query:"id"`
		Header  string `header:"H"`
		Form    string `form:"f"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12").
		SetHeaders("H", "header").
		SetPostArg("f", "form").
		SetUrlEncodeContentType()
	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	var result Req

	err := DefaultBinder.Bind(req.Req, params, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 1, result.Version)
	assert.DeepEqual(t, 12, result.ID)
	assert.DeepEqual(t, "header", result.Header)
	assert.DeepEqual(t, "form", result.Form)
}

// fixme: []byte 绑定
func TestBind_SliceType(t *testing.T) {
	type Req struct {
		ID   *[]int    `query:"id"`
		Str  [3]string `query:"str"`
		Byte []byte    `query:"b"`
	}
	IDs := []int{11, 12, 13}
	Strs := [3]string{"qwe", "asd", "zxc"}
	Bytes := []byte("123")

	req := newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?id=%d&id=%d&id=%d&str=%s&str=%s&str=%s&b=%d&b=%d&b=%d", IDs[0], IDs[1], IDs[2], Strs[0], Strs[1], Strs[2], Bytes[0], Bytes[1], Bytes[2]))

	var result Req

	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 3, len(*result.ID))
	for idx, val := range IDs {
		assert.DeepEqual(t, val, (*result.ID)[idx])
	}
	assert.DeepEqual(t, 3, len(result.Str))
	for idx, val := range Strs {
		assert.DeepEqual(t, val, result.Str[idx])
	}
	assert.DeepEqual(t, 3, len(result.Byte))
	for idx, val := range Bytes {
		assert.DeepEqual(t, val, result.Byte[idx])
	}
}

func TestBind_StructType(t *testing.T) {
	type FFF struct {
		F1 string `query:"F1"`
	}

	type TTT struct {
		T1 string `query:"F1"`
		T2 FFF
	}

	type Foo struct {
		F1 string `query:"F1"`
		F2 string `header:"f2"`
		F3 TTT
	}

	type Bar struct {
		B1 string `query:"B1"`
		B2 Foo    `query:"B2"`
	}

	var result Bar

	req := newMockRequest().SetRequestURI("http://foobar.com?F1=f1&B1=b1").SetHeader("f2", "f2")

	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Error(err)
	}

	assert.DeepEqual(t, "b1", result.B1)
	assert.DeepEqual(t, "f1", result.B2.F1)
	assert.DeepEqual(t, "f2", result.B2.F2)
	assert.DeepEqual(t, "f1", result.B2.F3.T1)
	assert.DeepEqual(t, "f1", result.B2.F3.T2.F1)
}

func TestBind_PointerType(t *testing.T) {
	type TT struct {
		T1 string `query:"F1"`
	}

	type Foo struct {
		F1 *TT                       `query:"F1"`
		F2 *******************string `query:"F1"`
	}

	type Bar struct {
		B1 ***string `query:"B1"`
		B2 ****Foo   `query:"B2"`
		B3 []*string `query:"B3"`
		B4 [2]*int   `query:"B4"`
	}

	result := Bar{}

	F1 := "f1"
	B1 := "b1"
	B2 := "b2"
	B3s := []string{"b31", "b32"}
	B4s := [2]int{0, 1}

	req := newMockRequest().SetRequestURI(fmt.Sprintf("http://foobar.com?F1=%s&B1=%s&B2=%s&B3=%s&B3=%s&B4=%d&B4=%d", F1, B1, B2, B3s[0], B3s[1], B4s[0], B4s[1])).
		SetHeader("f2", "f2")

	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, B1, ***result.B1)
	assert.DeepEqual(t, F1, (*(****result.B2).F1).T1)
	assert.DeepEqual(t, F1, *******************(****result.B2).F2)
	assert.DeepEqual(t, len(B3s), len(result.B3))
	for idx, val := range B3s {
		assert.DeepEqual(t, val, *result.B3[idx])
	}
	assert.DeepEqual(t, len(B4s), len(result.B4))
	for idx, val := range B4s {
		assert.DeepEqual(t, val, *result.B4[idx])
	}
}

func TestBind_NestedStruct(t *testing.T) {
	type Foo struct {
		F1 string `query:"F1"`
	}

	type Bar struct {
		Foo
		Nested struct {
			N1 string `query:"F1"`
		}
	}

	result := Bar{}

	req := newMockRequest().SetRequestURI("http://foobar.com?F1=qwe")
	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "qwe", result.Foo.F1)
	assert.DeepEqual(t, "qwe", result.Nested.N1)
}

func TestBind_SliceStruct(t *testing.T) {
	type Foo struct {
		F1 string `json:"f1"`
	}

	type Bar struct {
		B1 []Foo `query:"F1"`
	}

	result := Bar{}
	B1s := []string{"1", "2", "3"}

	req := newMockRequest().SetRequestURI(fmt.Sprintf("http://foobar.com?F1={\"f1\":\"%s\"}&F1={\"f1\":\"%s\"}&F1={\"f1\":\"%s\"}", B1s[0], B1s[1], B1s[2]))
	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, len(result.B1), len(B1s))
	for idx, val := range B1s {
		assert.DeepEqual(t, B1s[idx], val)
	}
}

func TestBind_MapType(t *testing.T) {
	var result map[string]string
	req := newMockRequest().
		SetJSONContentType().
		SetBody([]byte(`{"j1":"j1", "j2":"j2"}`))
	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 2, len(result))
	assert.DeepEqual(t, "j1", result["j1"])
	assert.DeepEqual(t, "j2", result["j2"])
}

func TestBind_MapFieldType(t *testing.T) {
	type Foo struct {
		F1 ***map[string]string `query:"f1" json:"f1"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?f1={\"f1\":\"f1\"}").
		SetJSONContentType().
		SetBody([]byte(`{"j1":"j1", "j2":"j2"}`))
	result := Foo{}
	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 1, len(***result.F1))
	assert.DeepEqual(t, "f1", (***result.F1)["f1"])
}

func TestBind_UnexportedField(t *testing.T) {
	var s struct {
		A int `query:"a"`
		b int `query:"b"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=1&b=2")
	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 1, s.A)
	assert.DeepEqual(t, 0, s.b)
}

func TestBind_NoTagField(t *testing.T) {
	var s struct {
		A string
		B string
		C string
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?B=b1&C=c1").
		SetHeader("A", "a2")

	var params param.Params
	params = append(params, param.Param{
		Key:   "B",
		Value: "b2",
	})

	err := DefaultBinder.Bind(req.Req, params, &s)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, "a2", s.A)
	assert.DeepEqual(t, "b2", s.B)
	assert.DeepEqual(t, "c1", s.C)
}

func TestBind_ZeroValueBind(t *testing.T) {
	var s struct {
		A int     `query:"a"`
		B float64 `query:"b"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=&b")

	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 0, s.A)
	assert.DeepEqual(t, float64(0), s.B)
}

func TestBind_DefaultValueBind(t *testing.T) {
	var s struct {
		A int      `default:"15"`
		B float64  `query:"b" default:"17"`
		C []int    `default:"15"`
		D []string `default:"qwe"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com")

	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 15, s.A)
	assert.DeepEqual(t, float64(17), s.B)
	assert.DeepEqual(t, 15, s.C[0])
	assert.DeepEqual(t, "qwe", s.D[0])

	var d struct {
		D [2]string `default:"qwe"`
	}

	err = DefaultBinder.Bind(req.Req, nil, &d)
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestBind_RequiredBind(t *testing.T) {
	var s struct {
		A int `query:"a,required"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeader("A", "1")

	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err == nil {
		t.Fatal("expected error")
	}

	var d struct {
		A int `query:"a,required" header:"A"`
	}
	err = DefaultBinder.Bind(req.Req, nil, &d)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 1, d.A)
}

func TestBind_TypedefType(t *testing.T) {
	type Foo string
	type Bar *int
	type T struct {
		T1 string `query:"a"`
	}
	type TT T

	var s struct {
		A  Foo `query:"a"`
		B  Bar `query:"b"`
		T1 TT
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=1&b=2")
	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, Foo("1"), s.A)
	assert.DeepEqual(t, 2, *s.B)
	assert.DeepEqual(t, "1", s.T1.T1)
}

type EnumType int64

const (
	EnumType_TWEET   EnumType = 0
	EnumType_RETWEET EnumType = 2
)

func (p EnumType) String() string {
	switch p {
	case EnumType_TWEET:
		return "TWEET"
	case EnumType_RETWEET:
		return "RETWEET"
	}
	return "<UNSET>"
}

func TestBind_EnumBind(t *testing.T) {
	var s struct {
		A EnumType `query:"a"`
		B EnumType `query:"b"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=0&b=2")
	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err != nil {
		t.Fatal(err)
	}
}

type CustomizedDecode struct {
	A string
}

func (c *CustomizedDecode) CustomizedFieldDecode(req *protocol.Request, params path.PathParam) error {
	q1 := req.URI().QueryArgs().Peek("a")
	if len(q1) == 0 {
		return fmt.Errorf("can be nil")
	}

	c.A = string(q1)
	return nil
}

func TestBind_CustomizedTypeDecode(t *testing.T) {
	type Foo struct {
		F ***CustomizedDecode
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=1&b=2")
	result := Foo{}
	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, "1", (***result.F).A)

	type Bar struct {
		B *Foo
	}

	result2 := Bar{}
	err = DefaultBinder.Bind(req.Req, nil, &result2)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "1", (***(*result2.B).F).A)
}

func TestBind_JSON(t *testing.T) {
	type Req struct {
		J1 string    `json:"j1"`
		J2 int       `json:"j2" query:"j2"` // 1. json unmarshal 2. query binding cover
		J3 []byte    `json:"j3"`
		J4 [2]string `json:"j4"`
	}
	J3s := []byte("12")
	J4s := [2]string{"qwe", "asd"}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?j2=13").
		SetJSONContentType().
		SetBody([]byte(fmt.Sprintf(`{"j1":"j1", "j2":12, "j3":[%d, %d], "j4":["%s", "%s"]}`, J3s[0], J3s[1], J4s[0], J4s[1])))
	var result Req
	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "j1", result.J1)
	assert.DeepEqual(t, 13, result.J2)
	for idx, val := range J3s {
		assert.DeepEqual(t, val, result.J3[idx])
	}
	for idx, val := range J4s {
		assert.DeepEqual(t, val, result.J4[idx])
	}
}

func TestBind_ResetJSONUnmarshal(t *testing.T) {
	ResetStdJSONUnmarshaler()
	type Req struct {
		J1 string    `json:"j1"`
		J2 int       `json:"j2"`
		J3 []byte    `json:"j3"`
		J4 [2]string `json:"j4"`
	}
	J3s := []byte("12")
	J4s := [2]string{"qwe", "asd"}

	req := newMockRequest().
		SetJSONContentType().
		SetBody([]byte(fmt.Sprintf(`{"j1":"j1", "j2":12, "j3":[%d, %d], "j4":["%s", "%s"]}`, J3s[0], J3s[1], J4s[0], J4s[1])))
	var result Req
	err := DefaultBinder.Bind(req.Req, nil, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "j1", result.J1)
	assert.DeepEqual(t, 12, result.J2)
	for idx, val := range J3s {
		assert.DeepEqual(t, val, result.J3[idx])
	}
	for idx, val := range J4s {
		assert.DeepEqual(t, val, result.J4[idx])
	}
}

func TestBind_FileBind(t *testing.T) {
	type Nest struct {
		N multipart.FileHeader `file_name:"d"`
	}

	var s struct {
		A *multipart.FileHeader `file_name:"a"`
		B *multipart.FileHeader `form:"b"`
		C multipart.FileHeader
		D **Nest `file_name:"d"`
	}
	fileName := "binder_test.go"
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetFile("a", fileName).
		SetFile("b", fileName).
		SetFile("C", fileName).
		SetFile("d", fileName)
	// to parse multipart files
	req2 := req2.GetHTTP1Request(req.Req)
	_ = req2.String()
	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, fileName, s.A.Filename)
	assert.DeepEqual(t, fileName, s.B.Filename)
	assert.DeepEqual(t, fileName, s.C.Filename)
	assert.DeepEqual(t, fileName, (**s.D).N.Filename)
}

func TestBind_FileSliceBind(t *testing.T) {
	type Nest struct {
		N *[]*multipart.FileHeader `form:"b"`
	}
	var s struct {
		A []multipart.FileHeader  `form:"a"`
		B [3]multipart.FileHeader `form:"b"`
		C []*multipart.FileHeader `form:"b"`
		D Nest
	}
	fileName := "binder_test.go"
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetFile("a", fileName).
		SetFile("a", fileName).
		SetFile("a", fileName).
		SetFile("b", fileName).
		SetFile("b", fileName).
		SetFile("b", fileName)
	// to parse multipart files
	req2 := req2.GetHTTP1Request(req.Req)
	_ = req2.String()
	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 3, len(s.A))
	for _, file := range s.A {
		assert.DeepEqual(t, fileName, file.Filename)
	}
	assert.DeepEqual(t, 3, len(s.B))
	for _, file := range s.B {
		assert.DeepEqual(t, fileName, file.Filename)
	}
	assert.DeepEqual(t, 3, len(s.C))
	for _, file := range s.C {
		assert.DeepEqual(t, fileName, file.Filename)
	}
	assert.DeepEqual(t, 3, len(*s.D.N))
	for _, file := range *s.D.N {
		assert.DeepEqual(t, fileName, file.Filename)
	}
}

func TestBind_AnonymousField(t *testing.T) {
	type nest struct {
		n1     string       `query:"n1"` // bind default value
		N2     ***string    `query:"n2"` // bind n2 value
		string `query:"n3"` // bind default value
	}

	var s struct {
		s1  int          `query:"s1"` // bind default value
		int `query:"s2"` // bind default value
		nest
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?s1=1&s2=2&n1=1&n2=2&n3=3")
	err := DefaultBinder.Bind(req.Req, nil, &s)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 0, s.s1)
	assert.DeepEqual(t, 0, s.int)
	assert.DeepEqual(t, "", s.nest.n1)
	assert.DeepEqual(t, "2", ***s.nest.N2)
	assert.DeepEqual(t, "", s.nest.string)
}

func TestBind_IgnoreField(t *testing.T) {
	type Req struct {
		Version int    `path:"-"`
		ID      int    `query:"-"`
		Header  string `header:"-"`
		Form    string `form:"-"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12").
		SetHeaders("H", "header").
		SetPostArg("f", "form").
		SetUrlEncodeContentType()
	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	var result Req

	err := DefaultBinder.Bind(req.Req, params, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 0, result.Version)
	assert.DeepEqual(t, 0, result.ID)
	assert.DeepEqual(t, "", result.Header)
	assert.DeepEqual(t, "", result.Form)
}

func Benchmark_Binding(b *testing.B) {
	type Req struct {
		Version string `path:"v"`
		ID      int    `query:"id"`
		Header  string `header:"h"`
		Form    string `form:"f"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12").
		SetHeaders("H", "header").
		SetPostArg("f", "form").
		SetUrlEncodeContentType()

	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result Req
		err := DefaultBinder.Bind(req.Req, params, &result)
		if err != nil {
			b.Error(err)
		}
		if result.ID != 12 {
			b.Error("Id failed")
		}
		if result.Form != "form" {
			b.Error("form failed")
		}
		if result.Header != "header" {
			b.Error("header failed")
		}
		if result.Version != "1" {
			b.Error("path failed")
		}
	}
}
