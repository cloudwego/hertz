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
 */

package decoder

import (
	"fmt"
	"mime/multipart"
	"reflect"
	"testing"
	"unsafe"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

func makeparams(kvs ...string) param.Params {
	ret := make(param.Params, 0, len(kvs)/2)
	for i := 0; i < len(kvs); i += 2 {
		ret = append(ret, param.Param{Key: kvs[i], Value: kvs[i+1]})
	}
	return ret
}

func requestFieldOffset(p *protocol.Request, fieldname string) uintptr {
	f, ok := reflect.TypeOf(p).Elem().FieldByName(fieldname)
	if !ok {
		panic(fmt.Sprintf("fieldname %q not found in %T", fieldname, p))
	}
	return f.Offset
}

func TestDecoder_Base(t *testing.T) {
	type TestStruct struct {
		A bool    `path:"a"`
		B uint    `path:"b"`
		C uint8   `path:"c"`
		D uint16  `path:"d"`
		E uint32  `path:"e"`
		F uint64  `path:"f"`
		G int     `path:"g"`
		H int8    `path:"h"`
		I int16   `path:"i"`
		J int32   `path:"j"`
		K int64   `path:"k"`
		L string  `path:"l"`
		M float32 `path:"m"`
		N float64 `path:"n"`
	}

	args := makeparams(
		"a", "1",
		"b", "2",
		"c", "3",
		"d", "4",
		"e", "5",
		"f", "6",
		"g", "7",
		"h", "8",
		"i", "9",
		"j", "10",
		"k", "11",
		"l", "12",
		"m", "13",
		"n", "14",
	)
	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.Assert(t, err == nil, err)

	p := &TestStruct{}
	changed, err := dec.Decode(nil, args, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, p.A == true, p.A)
	assert.Assert(t, p.B == 2, p.B)
	assert.Assert(t, p.C == 3, p.C)
	assert.Assert(t, p.D == 4, p.D)
	assert.Assert(t, p.E == 5, p.E)
	assert.Assert(t, p.F == 6, p.F)
	assert.Assert(t, p.G == 7, p.G)
	assert.Assert(t, p.H == 8, p.H)
	assert.Assert(t, p.I == 9, p.I)
	assert.Assert(t, p.J == 10, p.J)
	assert.Assert(t, p.K == 11, p.K)
	assert.Assert(t, p.L == "12", p.L)
	assert.Assert(t, p.M == 13, p.M)
	assert.Assert(t, p.N == 14, p.N)
}

func TestDecoder_Base_LooseZeroMode(t *testing.T) {
	type TestStruct struct {
		A bool `path:"a"`
	}

	args := makeparams(
		"a", "",
	)

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{LooseZeroMode: true})
	assert.Assert(t, err == nil, err)

	p := &TestStruct{}
	changed, err := dec.Decode(nil, args, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, p.A == false, p.A)
}

func TestDecoder_CustomFunc(t *testing.T) {
	type TestStruct struct {
		A int `path:"a"`
	}

	args := makeparams(
		"a", "8",
	)

	customFunc := func(_ *protocol.Request, _ param.Params, s string) (reflect.Value, error) {
		assert.Assert(t, s == "8", s)
		return reflect.ValueOf(7), nil
	}

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)),
		&DecodeConfig{TypeUnmarshalFuncs: map[reflect.Type]CustomDecodeFunc{
			reflect.TypeOf(int(0)): func(req *protocol.Request, p param.Params, s string) (reflect.Value, error) {
				return customFunc(req, p, s)
			},
		}})
	assert.Assert(t, err == nil, err)

	p := &TestStruct{}
	changed, err := dec.Decode(nil, args, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, p.A == 7, p.A)

	customFunc = func(_ *protocol.Request, _ param.Params, _ string) (reflect.Value, error) {
		return reflect.ValueOf(int8(0)), nil
	}

	p.A = 9
	changed, err = dec.Decode(nil, args, reflect.ValueOf(p))
	assert.Assert(t, changed == false && err != nil, changed, err)
	assert.DeepEqual(t, `decode field "A" err: call TypeUnmarshalFunc for type int err: returned int8`, err.Error())
	assert.Assert(t, p.A == 9, p.A)

}

func TestDecoder_Map(t *testing.T) {
	type TestStruct struct {
		M map[string]string `path:"m"`
	}

	args := makeparams(
		"m", `{"x":"y"}`,
	)

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.Assert(t, err == nil, err)

	p := &TestStruct{}
	changed, err := dec.Decode(nil, args, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, len(p.M) == 1 && p.M["x"] == "y", p.M)
}

func TestDecoder_Map_LooseZeroMode(t *testing.T) {
	type TestStruct struct {
		M map[string]string `path:"m"`
	}

	args := makeparams(
		"m", "",
	)

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{LooseZeroMode: true})
	assert.Assert(t, err == nil, err)

	p := &TestStruct{}
	changed, err := dec.Decode(nil, args, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, len(p.M) == 0 && p.M != nil, p.M)
}

func update_multipartForm(p *protocol.Request, form *multipart.Form) {
	off := requestFieldOffset(p, "multipartForm")
	*(**multipart.Form)(unsafe.Add(unsafe.Pointer(p), off)) = form
}

func TestDecoder_File(t *testing.T) {
	type TestStruct struct {
		F1 *multipart.FileHeader   `form:"test1.go" file_name:"test2.go"`
		F2 multipart.FileHeader    `form:"test1.go"`
		F3 []*multipart.FileHeader `form:"test1.go" file_name:"test3.go"`
	}

	f1 := &multipart.FileHeader{Filename: "test1.go"}
	f2 := &multipart.FileHeader{Filename: "test2.go"}
	f3 := &multipart.FileHeader{Filename: "test3-1.go"}
	f4 := &multipart.FileHeader{Filename: "test3-2.go"}

	req := &protocol.Request{}
	update_multipartForm(req, &multipart.Form{
		File: map[string][]*multipart.FileHeader{
			"test1.go": {f1},
			"test2.go": {f2},
			"test3.go": {f3, f4},
		},
	})

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.Assert(t, err == nil, err)

	p := &TestStruct{}
	changed, err := dec.Decode(req, nil, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, p.F1 != nil && p.F1.Filename == f2.Filename)
	assert.Assert(t, p.F2.Filename == f1.Filename)
	assert.Assert(t, len(p.F3) == 2, len(p.F3))
	assert.Assert(t, p.F3[0].Filename == f3.Filename)
	assert.Assert(t, p.F3[1].Filename == f4.Filename)

}

func TestDecoder_Slice(t *testing.T) {
	type TestStruct struct {
		SS    []string `path:"ss,required" form:"ss"`
		Slice []int    `form:"i" default:"[7]"`
		Array [1]int   `form:"j"`
		Zero  [1]int   `form:"k"`

		Custom []float32 `form:"custom"`

		NestedSlice [][]int `form:"nested"`
	}

	customFunc := func(_ *protocol.Request, _ param.Params, s string) (reflect.Value, error) {
		assert.Assert(t, s == "hello", s)
		return reflect.ValueOf(float32(100)), nil
	}

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)),
		&DecodeConfig{LooseZeroMode: true,
			TypeUnmarshalFuncs: map[reflect.Type]CustomDecodeFunc{
				reflect.TypeOf(float32(0)): func(req *protocol.Request, p param.Params, s string) (reflect.Value, error) {
					return customFunc(req, p, s)
				},
			}})
	assert.Assert(t, err == nil, err)

	req := &protocol.Request{}
	req.SetRequestURI("http://example.com?" +
		"ss=1&ss=2&" +
		"j=8&" +
		"k=&" + // for LooseZeroMode
		"custom=hello&" + // for customFunc
		"nested=1&",
	)

	p := &TestStruct{}
	changed, err := dec.Decode(req, nil, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, len(p.SS) == 2 && p.SS[0] == "1" && p.SS[1] == "2", p.SS)
	assert.Assert(t, len(p.Slice) == 1 && p.Slice[0] == 7, p.Slice)
	assert.Assert(t, p.Array[0] == 8, p.Array)
	assert.Assert(t, p.Zero[0] == 0, p.Zero)
	assert.Assert(t, len(p.Custom) == 1 && p.Custom[0] == 100, p.Custom)
}

func update_rawbody(p *protocol.Request, body []byte) {
	off := requestFieldOffset(p, "bodyRaw")
	*(*[]byte)(unsafe.Add(unsafe.Pointer(p), off)) = body
}

func TestDecoder_RawBody(t *testing.T) {
	type TestStruct struct {
		Body *[]byte `raw_body:""`
	}

	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), &DecodeConfig{})
	assert.Assert(t, err == nil, err)

	req := &protocol.Request{}
	update_rawbody(req, []byte("test"))

	p := &TestStruct{}
	changed, err := dec.Decode(req, nil, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, p.Body != nil && string(*p.Body) == "test")

}

func TestDecoder_Struct(t *testing.T) {
	type TestStruct0 struct {
		A string `json:"a" path:"a"`
		B string `json:"b" path:"b"`
		C string
	}

	type TestStruct struct {
		X TestStruct0 `path:"x"`
	}
	args := makeparams(
		"x", `{"a":"hello", "b":"world"}`,
		"b", "test",
		"C", "defaulttag",
	)
	c := &DecodeConfig{}
	dec, err := NewDecoder(reflect.TypeOf((*TestStruct)(nil)), c)
	assert.Assert(t, err == nil, err)

	p := &TestStruct{}
	changed, err := dec.Decode(nil, args, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, p.X.A == "hello", p.X.A)
	assert.Assert(t, p.X.B == "test", p.X.B)
	assert.Assert(t, p.X.C == "defaulttag", p.X.C)

	p.X.A = ""
	p.X.B = ""
	c.DisableStructFieldResolve = true
	changed, err = dec.Decode(nil, args, reflect.ValueOf(p))
	assert.Assert(t, changed == true && err == nil, changed, err)
	assert.Assert(t, p.X.A == "", p.X.A)
	assert.Assert(t, p.X.B == "test", p.X.B)
	assert.Assert(t, p.X.C == "defaulttag", p.X.C)
}
