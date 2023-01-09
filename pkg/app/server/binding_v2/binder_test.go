package binding_v2

import (
	"fmt"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server/binding"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

func TestBind_BaseType(t *testing.T) {
	bind := Bind{}
	type Req struct {
		Version int `path:"v"`
		ID      int    `query:"id"`
		Header  string `header:"H"`
		Form    string `form:"f"`
	}

	req := &protocol.Request{}
	req.SetRequestURI("http://foobar.com?id=12")
	req.Header.Set("H", "header") // disableNormalizing
	req.PostArgs().Add("f", "form")
	req.Header.SetContentTypeBytes([]byte("application/x-www-form-urlencoded"))
	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	var result Req

	err := bind.Bind(req, params, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 1, result.Version)
	assert.DeepEqual(t, 12, result.ID)
	assert.DeepEqual(t, "header", result.Header)
	assert.DeepEqual(t,"form", result.Form)
}

func TestBind_SliceType(t *testing.T) {
	bind := Bind{}
	type Req struct {
		ID   []int     `query:"id"`
		Str  [3]string `query:"str"`
		Byte []byte    `query:"b"`
	}
	IDs := []int{11, 12, 13}
	Strs := [3]string{"qwe", "asd", "zxc"}
	Bytes := []byte("123")

	req := &protocol.Request{}
	req.SetRequestURI(fmt.Sprintf("http://foobar.com?id=%d&id=%d&id=%d&str=%s&str=%s&str=%s&b=%d&b=%d&b=%d", IDs[0], IDs[1], IDs[2], Strs[0], Strs[1], Strs[2], Bytes[0], Bytes[1], Bytes[2]))

	var result Req

	err := bind.Bind(req, nil, &result)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 3, len(result.ID))
	for idx, val := range IDs {
		assert.DeepEqual(t, val, result.ID[idx])
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

func TestBind_JSON(t *testing.T) {
	bind := Bind{}
	type Req struct {
		J1 string `json:"j1"`
		J2 int    `json:"j2" query:"j2"` // 1. json unmarshal 2. query binding cover
		// todo: map
		J3 []byte    `json:"j3"`
		J4 [2]string `json:"j4"`
	}
	J3s := []byte("12")
	J4s := [2]string{"qwe", "asd"}

	req := &protocol.Request{}
	req.SetRequestURI("http://foobar.com?j2=13")
	req.Header.SetContentTypeBytes([]byte(jsonContentTypeBytes))
	data := []byte(fmt.Sprintf(`{"j1":"j1", "j2":12, "j3":[%d, %d], "j4":["%s", "%s"]}`, J3s[0], J3s[1], J4s[0], J4s[1]))
	req.SetBody(data)
	req.Header.SetContentLength(len(data))
	var result Req
	err := bind.Bind(req, nil, &result)
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

func Benchmark_V2(b *testing.B) {
	bind := Bind{}
	type Req struct {
		Version string `path:"v"`
		ID      int    `query:"id"`
		Header  string `header:"h"`
		Form    string `form:"f"`
	}

	req := &protocol.Request{}
	req.SetRequestURI("http://foobar.com?id=12")
	req.Header.Set("h", "header")
	req.PostArgs().Add("f", "form")
	req.Header.SetContentTypeBytes([]byte("application/x-www-form-urlencoded"))
	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result Req
		err := bind.Bind(req, params, &result)
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

func Benchmark_V1(b *testing.B) {
	type Req struct {
		Version string `path:"v"`
		ID      int    `query:"id"`
		Header  string `header:"h"`
		Form    string `form:"f"`
	}

	req := &protocol.Request{}
	req.SetRequestURI("http://foobar.com?id=12")
	req.Header.Set("h", "header")
	req.PostArgs().Add("f", "form")
	req.Header.SetContentTypeBytes([]byte("application/x-www-form-urlencoded"))
	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result Req
		err := binding.Bind(req, &result, params)
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
