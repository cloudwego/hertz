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
 * Modifications are Copyright 2023 CloudWeGo Authors
 */

package binding

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server/binding/testdata"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	req2 "github.com/cloudwego/hertz/pkg/protocol/http1/req"
	"github.com/cloudwego/hertz/pkg/route/param"
	"google.golang.org/protobuf/proto"
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
	m.Req.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationJSON))
	return m
}

func (m *mockRequest) SetProtobufContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte(consts.MIMEPROTOBUF))
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

	err := DefaultBinder().Bind(req.Req, &result, params)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 1, result.Version)
	assert.DeepEqual(t, 12, result.ID)
	assert.DeepEqual(t, "header", result.Header)
	assert.DeepEqual(t, "form", result.Form)
}

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

	err := DefaultBinder().Bind(req.Req, &result, nil)
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

	err := DefaultBinder().Bind(req.Req, &result, nil)
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

	err := DefaultBinder().Bind(req.Req, &result, nil)
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
	err := DefaultBinder().Bind(req.Req, &result, nil)
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
	err := DefaultBinder().Bind(req.Req, &result, nil)
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
	err := DefaultBinder().Bind(req.Req, &result, nil)
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
	err := DefaultBinder().Bind(req.Req, &result, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 1, len(***result.F1))
	assert.DeepEqual(t, "f1", (***result.F1)["f1"])

	type Foo2 struct {
		F1 map[string]string `query:"f1" json:"f1"`
	}
	result2 := Foo2{}
	err = DefaultBinder().Bind(req.Req, &result2, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, 1, len(result2.F1))
	assert.DeepEqual(t, "f1", result2.F1["f1"])
	req = newMockRequest().
		SetRequestURI("http://foobar.com?f1={\"f1\":\"f1\"")
	result2 = Foo2{}
	err = DefaultBinder().Bind(req.Req, &result2, nil)
	if err == nil {
		t.Error(err)
	}
}

func TestBind_UnexportedField(t *testing.T) {
	var s struct {
		A int `query:"a"`
		b int `query:"b"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=1&b=2")
	err := DefaultBinder().Bind(req.Req, &s, nil)
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

	err := DefaultBinder().Bind(req.Req, &s, params)
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

	bindConfig := &BindConfig{}
	bindConfig.LooseZeroMode = true
	binder := NewDefaultBinder(bindConfig)
	err := binder.Bind(req.Req, &s, nil)
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

	err := DefaultBinder().Bind(req.Req, &s, nil)
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

	err = DefaultBinder().Bind(req.Req, &d, nil)
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

	err := DefaultBinder().Bind(req.Req, &s, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var d struct {
		A int `query:"a,required" header:"A"`
	}
	err = DefaultBinder().Bind(req.Req, &d, nil)
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
	err := DefaultBinder().Bind(req.Req, &s, nil)
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
	err := DefaultBinder().Bind(req.Req, &s, nil)
	if err != nil {
		t.Fatal(err)
	}
}

type CustomizedDecode struct {
	A string
}

func TestBind_CustomizedTypeDecode(t *testing.T) {
	type Foo struct {
		F ***CustomizedDecode `query:"a"`
	}

	bindConfig := &BindConfig{}
	err := bindConfig.RegTypeUnmarshal(reflect.TypeOf(CustomizedDecode{}), func(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
		q1 := req.URI().QueryArgs().Peek("a")
		if len(q1) == 0 {
			return reflect.Value{}, fmt.Errorf("can be nil")
		}
		val := CustomizedDecode{
			A: string(q1),
		}
		return reflect.ValueOf(val), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	binder := NewDefaultBinder(bindConfig)

	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=1&b=2")
	result := Foo{}
	err = binder.Bind(req.Req, &result, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.DeepEqual(t, "1", (***result.F).A)

	type Bar struct {
		B *Foo
	}

	result2 := Bar{}
	err = binder.Bind(req.Req, &result2, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "1", (***(*result2.B).F).A)
}

func TestBind_CustomizedTypeDecodeForPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expect a panic, but get nil")
		}
	}()

	bindConfig := &BindConfig{}
	bindConfig.MustRegTypeUnmarshal(reflect.TypeOf(string("")), func(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
		return reflect.Value{}, nil
	})
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
	err := DefaultBinder().Bind(req.Req, &result, nil)
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
	bindConfig := &BindConfig{}
	bindConfig.UseStdJSONUnmarshaler()
	binder := NewDefaultBinder(bindConfig)
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
	err := binder.Bind(req.Req, &result, nil)
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
	err := DefaultBinder().Bind(req.Req, &s, nil)
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
	err := DefaultBinder().Bind(req.Req, &s, nil)
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
	err := DefaultBinder().Bind(req.Req, &s, nil)
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
		SetRequestURI("http://foobar.com?ID=12").
		SetHeaders("Header", "header").
		SetPostArg("Form", "form").
		SetUrlEncodeContentType()
	var params param.Params
	params = append(params, param.Param{
		Key:   "Version",
		Value: "1",
	})

	var result Req

	err := DefaultBinder().Bind(req.Req, &result, params)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 0, result.Version)
	assert.DeepEqual(t, 0, result.ID)
	assert.DeepEqual(t, "", result.Header)
	assert.DeepEqual(t, "", result.Form)
}

func TestBind_DefaultTag(t *testing.T) {
	type Req struct {
		Version int
		ID      int
		Header  string
		Form    string
	}
	type Req2 struct {
		Version int
		ID      int
		Header  string
		Form    string
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?ID=12").
		SetHeaders("Header", "header").
		SetPostArg("Form", "form").
		SetUrlEncodeContentType()
	var params param.Params
	params = append(params, param.Param{
		Key:   "Version",
		Value: "1",
	})
	var result Req
	err := DefaultBinder().Bind(req.Req, &result, params)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 1, result.Version)
	assert.DeepEqual(t, 12, result.ID)
	assert.DeepEqual(t, "header", result.Header)
	assert.DeepEqual(t, "form", result.Form)

	bindConfig := &BindConfig{}
	bindConfig.DisableDefaultTag = true
	binder := NewDefaultBinder(bindConfig)
	result2 := Req2{}
	err = binder.Bind(req.Req, &result2, params)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 0, result2.Version)
	assert.DeepEqual(t, 0, result2.ID)
	assert.DeepEqual(t, "", result2.Header)
	assert.DeepEqual(t, "", result2.Form)
}

func TestBind_StructFieldResolve(t *testing.T) {
	type Nested struct {
		A int `query:"a" json:"a"`
		B int `query:"b" json:"b"`
	}
	type Req struct {
		N Nested `query:"n"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?n={\"a\":1,\"b\":2}").
		SetHeaders("Header", "header").
		SetPostArg("Form", "form").
		SetUrlEncodeContentType()
	var result Req
	bindConfig := &BindConfig{}
	bindConfig.DisableStructFieldResolve = false
	binder := NewDefaultBinder(bindConfig)
	err := binder.Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 1, result.N.A)
	assert.DeepEqual(t, 2, result.N.B)

	req = newMockRequest().
		SetRequestURI("http://foobar.com?n={\"a\":1,\"b\":2}&a=11&b=22").
		SetHeaders("Header", "header").
		SetPostArg("Form", "form").
		SetUrlEncodeContentType()
	err = DefaultBinder().Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 11, result.N.A)
	assert.DeepEqual(t, 22, result.N.B)
}

func TestBind_JSONRequiredField(t *testing.T) {
	type Nested2 struct {
		C int `json:"c,required"`
		D int `json:"dd,required"`
	}
	type Nested struct {
		A  int     `json:"a,required"`
		B  int     `json:"b,required"`
		N2 Nested2 `json:"n2"`
	}
	type Req struct {
		N Nested `json:"n,required"`
	}
	bodyBytes := []byte(`{
    "n": {
        "a": 1,
        "b": 2,
        "n2": {
             "dd": 4
        }
    }
}`)
	req := newMockRequest().
		SetRequestURI("http://foobar.com?j2=13").
		SetJSONContentType().
		SetBody(bodyBytes)
	var result Req
	err := DefaultBinder().Bind(req.Req, &result, nil)
	if err == nil {
		t.Errorf("expected an error, but get nil")
	}
	assert.DeepEqual(t, 1, result.N.A)
	assert.DeepEqual(t, 2, result.N.B)
	assert.DeepEqual(t, 0, result.N.N2.C)
	assert.DeepEqual(t, 4, result.N.N2.D)

	bodyBytes = []byte(`{
    "n": {
        "a": 1,
        "b": 2
    }
}`)
	req = newMockRequest().
		SetRequestURI("http://foobar.com?j2=13").
		SetJSONContentType().
		SetBody(bodyBytes)
	var result2 Req
	err = DefaultBinder().Bind(req.Req, &result2, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 1, result2.N.A)
	assert.DeepEqual(t, 2, result2.N.B)
	assert.DeepEqual(t, 0, result2.N.N2.C)
	assert.DeepEqual(t, 0, result2.N.N2.D)
}

func TestValidate_MultipleValidate(t *testing.T) {
	type Test1 struct {
		A int `query:"a" vd:"$>10"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?a=9")
	var result Test1
	err := DefaultBinder().BindAndValidate(req.Req, &result, nil)
	if err == nil {
		t.Fatalf("expected an error, but get nil")
	}
}

func TestBind_BindQuery(t *testing.T) {
	type Req struct {
		Q1 int `query:"q1"`
		Q2 int
		Q3 string
		Q4 string
		Q5 []int
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?q1=1&Q2=2&Q3=3&Q4=4&Q5=51&Q5=52")

	var result Req

	err := DefaultBinder().BindQuery(req.Req, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 1, result.Q1)
	assert.DeepEqual(t, 2, result.Q2)
	assert.DeepEqual(t, "3", result.Q3)
	assert.DeepEqual(t, "4", result.Q4)
	assert.DeepEqual(t, 51, result.Q5[0])
	assert.DeepEqual(t, 52, result.Q5[1])
}

func TestBind_LooseMode(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.LooseZeroMode = false
	binder := NewDefaultBinder(bindConfig)
	type Req struct {
		ID int `query:"id"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=")

	var result Req

	err := binder.Bind(req.Req, &result, nil)
	if err == nil {
		t.Fatal("expected err")
	}
	assert.DeepEqual(t, 0, result.ID)

	bindConfig.LooseZeroMode = true
	binder = NewDefaultBinder(bindConfig)
	var result2 Req

	err = binder.Bind(req.Req, &result2, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 0, result.ID)
}

func TestBind_NonStruct(t *testing.T) {
	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=1&id=2")
	var id interface{}
	err := DefaultBinder().Bind(req.Req, &id, nil)
	if err != nil {
		t.Error(err)
	}

	err = DefaultBinder().BindAndValidate(req.Req, &id, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestBind_BindTag(t *testing.T) {
	type Req struct {
		Query  string
		Header string
		Path   string
		Form   string
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?Query=query").
		SetHeader("Header", "header").
		SetPostArg("Form", "form")
	var params param.Params
	params = append(params, param.Param{
		Key:   "Path",
		Value: "path",
	})
	result := Req{}

	// test query tag
	err := DefaultBinder().BindQuery(req.Req, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "query", result.Query)

	// test header tag
	result = Req{}
	err = DefaultBinder().BindHeader(req.Req, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "header", result.Header)

	// test form tag
	result = Req{}
	err = DefaultBinder().BindForm(req.Req, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "form", result.Form)

	// test path tag
	result = Req{}
	err = DefaultBinder().BindPath(req.Req, &result, params)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "path", result.Path)

	// test json tag
	req = newMockRequest().
		SetRequestURI("http://foobar.com").
		SetJSONContentType().
		SetBody([]byte("{\n    \"Query\": \"query\",\n    \"Path\": \"path\",\n    \"Header\": \"header\",\n    \"Form\": \"form\"\n}"))
	result = Req{}
	err = DefaultBinder().BindJSON(req.Req, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "form", result.Form)
	assert.DeepEqual(t, "query", result.Query)
	assert.DeepEqual(t, "header", result.Header)
	assert.DeepEqual(t, "path", result.Path)
}

func TestBind_BindAndValidate(t *testing.T) {
	type Req struct {
		ID int `query:"id" vd:"$>10"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12")

	// test bindAndValidate
	var result Req
	err := BindAndValidate(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 12, result.ID)

	// test bind
	result = Req{}
	err = Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 12, result.ID)

	// test validate
	req = newMockRequest().
		SetRequestURI("http://foobar.com?id=9")
	result = Req{}
	err = Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	err = Validate(result)
	if err == nil {
		t.Errorf("expect an error, but get nil")
	}
	assert.DeepEqual(t, 9, result.ID)
}

func TestBind_FastPath(t *testing.T) {
	type Req struct {
		ID int `query:"id" vd:"$>10"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12")

	// test bindAndValidate
	var result Req
	err := BindAndValidate(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 12, result.ID)
	// execute multiple times, test cache
	for i := 0; i < 10; i++ {
		result = Req{}
		err := BindAndValidate(req.Req, &result, nil)
		if err != nil {
			t.Error(err)
		}
		assert.DeepEqual(t, 12, result.ID)
	}
}

func TestBind_NonPointer(t *testing.T) {
	type Req struct {
		ID int `query:"id" vd:"$>10"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12")

	// test bindAndValidate
	var result Req
	err := BindAndValidate(req.Req, result, nil)
	if err == nil {
		t.Error("expect an error, but get nil")
	}

	err = Bind(req.Req, result, nil)
	if err == nil {
		t.Error("expect an error, but get nil")
	}
}

func TestBind_PreBind(t *testing.T) {
	type Req struct {
		Query  string
		Header string
		Path   string
		Form   string
	}
	// test json tag
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetJSONContentType().
		SetBody([]byte("\n    \"Query\": \"query\",\n    \"Path\": \"path\",\n    \"Header\": \"header\",\n    \"Form\": \"form\"\n}"))
	result := Req{}
	err := DefaultBinder().Bind(req.Req, &result, nil)
	if err == nil {
		t.Error("expect an error, but get nil")
	}
	err = DefaultBinder().BindAndValidate(req.Req, &result, nil)
	if err == nil {
		t.Error("expect an error, but get nil")
	}
}

func TestBind_BindProtobuf(t *testing.T) {
	data := testdata.HertzReq{Name: "hertz"}
	body, err := proto.Marshal(&data)
	if err != nil {
		t.Fatal(err)
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetProtobufContentType().
		SetBody(body)

	result := testdata.HertzReq{}
	err = DefaultBinder().BindAndValidate(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "hertz", result.Name)

	result = testdata.HertzReq{}
	err = DefaultBinder().BindProtobuf(req.Req, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "hertz", result.Name)
}

func TestBind_PointerStruct(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.DisableStructFieldResolve = false
	binder := NewDefaultBinder(bindConfig)
	type Foo struct {
		F1 string `query:"F1"`
	}
	type Bar struct {
		B1 **Foo `query:"B1,required"`
	}
	query := make(url.Values)
	query.Add("B1", "{\n    \"F1\": \"111\"\n}")

	var result Bar
	req := newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?%s", query.Encode()))

	err := binder.Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "111", (**result.B1).F1)

	result = Bar{}
	req = newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?%s&F1=222", query.Encode()))
	err = binder.Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "222", (**result.B1).F1)
}

func TestBind_StructRequired(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.DisableStructFieldResolve = false
	binder := NewDefaultBinder(bindConfig)
	type Foo struct {
		F1 string `query:"F1"`
	}
	type Bar struct {
		B1 **Foo `query:"B1,required"`
	}

	var result Bar
	req := newMockRequest().
		SetRequestURI("http://foobar.com")

	err := binder.Bind(req.Req, &result, nil)
	if err == nil {
		t.Error("expect an error, but get nil")
	}

	type Bar2 struct {
		B1 **Foo `query:"B1"`
	}
	var result2 Bar2
	req = newMockRequest().
		SetRequestURI("http://foobar.com")

	err = binder.Bind(req.Req, &result2, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestBind_StructErrorToWarn(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.DisableStructFieldResolve = false
	binder := NewDefaultBinder(bindConfig)
	type Foo struct {
		F1 string `query:"F1"`
	}
	type Bar struct {
		B1 **Foo `query:"B1,required"`
	}

	var result Bar
	req := newMockRequest().
		SetRequestURI("http://foobar.com?B1=111&F1=222")

	err := binder.Bind(req.Req, &result, nil)
	// transfer 'unmarsahl err' to 'warn'
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "222", (**result.B1).F1)

	type Bar2 struct {
		B1 Foo `query:"B1,required"`
	}
	var result2 Bar2
	err = binder.Bind(req.Req, &result2, nil)
	// transfer 'unmarsahl err' to 'warn'
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "222", result2.B1.F1)
}

func TestBind_DisallowUnknownFieldsConfig(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.EnableDecoderDisallowUnknownFields = true
	binder := NewDefaultBinder(bindConfig)
	type FooStructUseNumber struct {
		Foo interface{} `json:"foo"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetJSONContentType().
		SetBody([]byte(`{"foo": 123,"bar": "456"}`))
	var result FooStructUseNumber

	err := binder.BindJSON(req.Req, &result)
	if err == nil {
		t.Errorf("expected an error, but get nil")
	}
}

func TestBind_UseNumberConfig(t *testing.T) {
	bindConfig := &BindConfig{}
	bindConfig.EnableDecoderUseNumber = true
	binder := NewDefaultBinder(bindConfig)
	type FooStructUseNumber struct {
		Foo interface{} `json:"foo"`
	}
	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetJSONContentType().
		SetBody([]byte(`{"foo": 123}`))
	var result FooStructUseNumber

	err := binder.BindJSON(req.Req, &result)
	if err != nil {
		t.Error(err)
	}
	v, err := result.Foo.(json.Number).Int64()
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, int64(123), v)
}

func TestBind_InterfaceType(t *testing.T) {
	type Bar struct {
		B1 interface{} `query:"B1"`
	}

	var result Bar
	query := make(url.Values)
	query.Add("B1", `{"B1":"111"}`)
	req := newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?%s", query.Encode()))
	err := DefaultBinder().Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}

	type Bar2 struct {
		B2 *interface{} `query:"B1"`
	}

	var result2 Bar2
	err = DefaultBinder().Bind(req.Req, &result2, nil)
	if err != nil {
		t.Error(err)
	}
}

func Test_BindHeaderNormalize(t *testing.T) {
	type Req struct {
		Header string `header:"h"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeaders("h", "header")
	var result Req

	err := DefaultBinder().Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "header", result.Header)
	req = newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeaders("H", "header")
	err = DefaultBinder().Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "header", result.Header)

	type Req2 struct {
		Header string `header:"H"`
	}

	req2 := newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeaders("h", "header")
	var result2 Req2

	err2 := DefaultBinder().Bind(req2.Req, &result2, nil)
	if err != nil {
		t.Error(err2)
	}
	assert.DeepEqual(t, "header", result2.Header)
	req2 = newMockRequest().
		SetRequestURI("http://foobar.com").
		SetHeaders("H", "header")
	err2 = DefaultBinder().Bind(req2.Req, &result2, nil)
	if err2 != nil {
		t.Error(err2)
	}
	assert.DeepEqual(t, "header", result2.Header)

	type Req3 struct {
		Header string `header:"h"`
	}

	// without normalize, the header key & tag key need to be consistent
	req3 := newMockRequest().
		SetRequestURI("http://foobar.com")
	req3.Req.Header.DisableNormalizing()
	req3.SetHeaders("h", "header")
	var result3 Req3
	err3 := DefaultBinder().Bind(req3.Req, &result3, nil)
	if err3 != nil {
		t.Error(err3)
	}
	assert.DeepEqual(t, "header", result3.Header)
	req3 = newMockRequest().
		SetRequestURI("http://foobar.com")
	req3.Req.Header.DisableNormalizing()
	req3.SetHeaders("H", "header")
	result3 = Req3{}
	err3 = DefaultBinder().Bind(req3.Req, &result3, nil)
	if err3 != nil {
		t.Error(err3)
	}
	assert.DeepEqual(t, "", result3.Header)
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

func Test_ValidatorErrorFactory(t *testing.T) {
	type TestBind struct {
		A string `query:"a,required"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?b=20")
	CustomValidateErrFunc := func(failField, msg string) error {
		err := ValidateError{
			ErrType:   "validateErr",
			FailField: "[validateFailField]: " + failField,
			Msg:       "[validateErrMsg]: " + msg,
		}

		return &err
	}

	validateConfig := NewValidateConfig()
	validateConfig.SetValidatorErrorFactory(CustomValidateErrFunc)
	validator := NewValidator(validateConfig)

	var req TestBind
	err := Bind(r, &req, nil)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}

	type TestValidate struct {
		B int `query:"b" vd:"$>100"`
	}

	var reqValidate TestValidate
	err = Bind(r, &reqValidate, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = validator.ValidateStruct(&reqValidate)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
	assert.DeepEqual(t, "validateErr: expr_path=[validateFailField]: B, cause=[validateErrMsg]: ", err.Error())
}

// Test_Issue964 used to the cover issue for time.Time
func Test_Issue964(t *testing.T) {
	type CreateReq struct {
		StartAt *time.Time `json:"startAt"`
	}
	r := newMockRequest().SetBody([]byte("{\n  \"startAt\": \"2006-01-02T15:04:05+07:00\"\n}")).SetJSONContentType()
	var req CreateReq
	err := DefaultBinder().BindAndValidate(r.Req, &req, nil)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "2006-01-02 15:04:05 +0700 +0700", req.StartAt.String())
	r = newMockRequest()
	req = CreateReq{}
	err = DefaultBinder().BindAndValidate(r.Req, &req, nil)
	if err != nil {
		t.Error(err)
	}
	if req.StartAt != nil {
		t.Error("expected nil")
	}
}

type reqSameType struct {
	Parent   *reqSameType  `json:"parent"`
	Children []reqSameType `json:"children"`
	Foo1     reqSameType2  `json:"foo1"`
	A        string        `json:"a"`
}

type reqSameType2 struct {
	Foo1 *reqSameType `json:"foo1"`
}

func TestBind_Issue1015(t *testing.T) {
	req := newMockRequest().
		SetJSONContentType().
		SetBody([]byte(`{"parent":{"parent":{}, "children":[{},{}], "foo1":{"foo1":{}}}, "children":[{},{}], "a":"asd"}`))

	var result reqSameType

	err := DefaultBinder().Bind(req.Req, &result, nil)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, result.Parent)
	assert.NotNil(t, result.Parent.Parent)
	assert.Nil(t, result.Parent.Parent.Parent)
	assert.NotNil(t, result.Parent.Children)
	assert.DeepEqual(t, 2, len(result.Parent.Children))
	assert.NotNil(t, result.Parent.Foo1.Foo1)
	assert.DeepEqual(t, "", result.Parent.A)
	assert.DeepEqual(t, 2, len(result.Children))
	assert.Nil(t, result.Foo1.Foo1)
	assert.DeepEqual(t, "asd", result.A)
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
		err := DefaultBinder().Bind(req.Req, &result, params)
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
