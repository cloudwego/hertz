package binding

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestRawBody(t *testing.T) {
	type Recv struct {
		S []byte   `raw_body:""`
		F **string `raw_body:""`
	}
	bodyBytes := []byte("raw_body.............")
	req := newRequest("", nil, nil, bytes.NewReader(bodyBytes))
	recv := new(Recv)
	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		if err != nil {
			t.Error(err)
		}
	}

	assert.DeepEqual(t, bodyBytes, recv.S)
	assert.DeepEqual(t, string(bodyBytes), **recv.F)
}

func TestQueryString(t *testing.T) {
	type metric string
	type count int32

	type Recv struct {
		X **struct {
			A []string  `query:"a"`
			B string    `query:"b"`
			C *[]string `query:"c,required"`
			D *string   `query:"d"`
			E *[]***int `query:"e"`
			F metric    `query:"f"`
			G []count   `query:"g"`
		}
		Y string  `query:"y,required"`
		Z *string `query:"z"`
	}
	req := newRequest("http://localhost:8080/?a=a1&a=a2&b=b1&c=c1&c=c2&d=d1&d=d&f=qps&g=1002&g=1003&e=&e=2&y=y1", nil, nil, nil)
	recv := new(Recv)
	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 0, ***(*(**recv.X).E)[0])
	assert.DeepEqual(t, 2, ***(*(**recv.X).E)[1])
	assert.DeepEqual(t, []string{"a1", "a2"}, (**recv.X).A)
	assert.DeepEqual(t, "b1", (**recv.X).B)
	assert.DeepEqual(t, []string{"c1", "c2"}, *(**recv.X).C)
	assert.DeepEqual(t, "d1", *(**recv.X).D)
	assert.DeepEqual(t, metric("qps"), (**recv.X).F)
	assert.DeepEqual(t, []count{1002, 1003}, (**recv.X).G)
	assert.DeepEqual(t, "y1", recv.Y)
	assert.DeepEqual(t, (*string)(nil), recv.Z)
}

func TestGetBody(t *testing.T) {
	type Recv struct {
		X **struct {
			E string `json:"e,required" query:"e,required"`
		}
	}
	req := newRequest("http://localhost:8080/", nil, nil, nil)
	recv := new(Recv)
	err := DefaultBinder.Bind(req.Req, nil, recv)
	assert.DeepEqual(t, err.Error(), "'E' field is a 'required' parameter, but the request does not have this parameter")
}

func TestQueryNum(t *testing.T) {
	type Recv struct {
		X **struct {
			A []int     `query:"a"`
			B int32     `query:"b"`
			C *[]uint16 `query:"c,required"`
			D *float32  `query:"d"`
		}
		Y bool   `query:"y,required"`
		Z *int64 `query:"z"`
	}
	req := newRequest("http://localhost:8080/?a=11&a=12&b=21&c=31&c=32&d=41&d=42&y=true", nil, nil, nil)
	recv := new(Recv)
	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		if err != nil {
			t.Error(err)
		}
	}
	assert.DeepEqual(t, []int{11, 12}, (**recv.X).A)
	assert.DeepEqual(t, int32(21), (**recv.X).B)
	assert.DeepEqual(t, &[]uint16{31, 32}, (**recv.X).C)
	assert.DeepEqual(t, float32(41), *(**recv.X).D)
	assert.DeepEqual(t, true, recv.Y)
	assert.DeepEqual(t, (*int64)(nil), recv.Z)
}

func TestHeaderString(t *testing.T) {
	type Recv struct {
		X **struct {
			A []string  `header:"X-A"`
			B string    `header:"X-B"`
			C *[]string `header:"X-C,required"`
			D *string   `header:"X-D"`
		}
		Y string  `header:"X-Y,required"`
		Z *string `header:"X-Z"`
	}
	header := make(http.Header)
	header.Add("X-A", "a1")
	header.Add("X-A", "a2")
	header.Add("X-B", "b1")
	header.Add("X-C", "c1")
	header.Add("X-C", "c2")
	header.Add("X-D", "d1")
	header.Add("X-D", "d2")
	header.Add("X-Y", "y1")
	req := newRequest("", header, nil, nil)
	recv := new(Recv)
	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		if err != nil {
			t.Error(err)
		}
	}
	assert.DeepEqual(t, []string{"a1", "a2"}, (**recv.X).A)
	assert.DeepEqual(t, "b1", (**recv.X).B)
	assert.DeepEqual(t, []string{"c1", "c2"}, *(**recv.X).C)
	assert.DeepEqual(t, "d1", *(**recv.X).D)
	assert.DeepEqual(t, "y1", recv.Y)
	assert.DeepEqual(t, (*string)(nil), recv.Z)
}

func TestHeaderNum(t *testing.T) {
	type Recv struct {
		X **struct {
			A []int     `header:"X-A"`
			B int32     `header:"X-B"`
			C *[]uint16 `header:"X-C,required"`
			D *float32  `header:"X-D"`
		}
		Y bool   `header:"X-Y,required"`
		Z *int64 `header:"X-Z"`
	}
	header := make(http.Header)
	header.Add("X-A", "11")
	header.Add("X-A", "12")
	header.Add("X-B", "21")
	header.Add("X-C", "31")
	header.Add("X-C", "32")
	header.Add("X-D", "41")
	header.Add("X-D", "42")
	header.Add("X-Y", "true")
	req := newRequest("", header, nil, nil)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, []int{11, 12}, (**recv.X).A)
	assert.DeepEqual(t, int32(21), (**recv.X).B)
	assert.DeepEqual(t, &[]uint16{31, 32}, (**recv.X).C)
	assert.DeepEqual(t, float32(41), *(**recv.X).D)
	assert.DeepEqual(t, true, recv.Y)
	assert.DeepEqual(t, (*int64)(nil), recv.Z)
}

// todo: cookie slice
func TestCookieString(t *testing.T) {
	type Recv struct {
		X **struct {
			A []string  `cookie:"a"`
			B string    `cookie:"b"`
			C *[]string `cookie:"c,required"`
			D *string   `cookie:"d"`
		}
		Y string  `cookie:"y,required"`
		Z *string `cookie:"z"`
	}
	cookies := []*http.Cookie{
		{Name: "a", Value: "a1"},
		{Name: "a", Value: "a2"},
		{Name: "b", Value: "b1"},
		{Name: "c", Value: "c1"},
		{Name: "c", Value: "c2"},
		{Name: "d", Value: "d1"},
		{Name: "d", Value: "d2"},
		{Name: "y", Value: "y1"},
	}
	req := newRequest("", nil, cookies, nil)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, []string{"a2"}, (**recv.X).A)
	assert.DeepEqual(t, "b1", (**recv.X).B)
	assert.DeepEqual(t, []string{"c2"}, *(**recv.X).C)
	assert.DeepEqual(t, "d2", *(**recv.X).D)
	assert.DeepEqual(t, "y1", recv.Y)
	assert.DeepEqual(t, (*string)(nil), recv.Z)
}

func TestCookieNum(t *testing.T) {
	type Recv struct {
		X **struct {
			A []int     `cookie:"a"`
			B int32     `cookie:"b"`
			C *[]uint16 `cookie:"c,required"`
			D *float32  `cookie:"d"`
		}
		Y bool   `cookie:"y,required"`
		Z *int64 `cookie:"z"`
	}
	cookies := []*http.Cookie{
		{Name: "a", Value: "11"},
		{Name: "b", Value: "21"},
		{Name: "c", Value: "31"},
		{Name: "d", Value: "41"},
		{Name: "y", Value: "t"},
	}
	req := newRequest("", nil, cookies, nil)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, []int{11}, (**recv.X).A)
	assert.DeepEqual(t, int32(21), (**recv.X).B)
	assert.DeepEqual(t, &[]uint16{31}, (**recv.X).C)
	assert.DeepEqual(t, float32(41), *(**recv.X).D)
	assert.DeepEqual(t, true, recv.Y)
	assert.DeepEqual(t, (*int64)(nil), recv.Z)
}

func TestFormString(t *testing.T) {
	type Recv struct {
		X **struct {
			A []string  `form:"a"`
			B string    `form:"b"`
			C *[]string `form:"c,required"`
			D *string   `form:"d"`
		}
		Y   string                `form:"y,required"`
		Z   *string               `form:"z"`
		F   *multipart.FileHeader `form:"F1"`
		F1  multipart.FileHeader
		Fs  []multipart.FileHeader  `form:"F1"`
		Fs1 []*multipart.FileHeader `form:"F1"`
	}
	values := make(url.Values)
	values.Add("a", "a1")
	values.Add("a", "a2")
	values.Add("b", "b1")
	values.Add("c", "c1")
	values.Add("c", "c2")
	values.Add("d", "d1")
	values.Add("d", "d2")
	values.Add("y", "y1")
	for i, f := range []files{{
		"F1": []file{
			newFile("txt", strings.NewReader("0123")),
		},
	}} {
		contentType, bodyReader := newFormBody2(values, f)
		header := make(http.Header)
		header.Set("Content-Type", contentType)
		req := newRequest("", header, nil, bodyReader)
		recv := new(Recv)
		err := DefaultBinder.Bind(req.Req, nil, recv)
		if err != nil {
			t.Error(err)
		}
		assert.DeepEqual(t, []string{"a1", "a2"}, (**recv.X).A)
		assert.DeepEqual(t, "b1", (**recv.X).B)
		assert.DeepEqual(t, []string{"c1", "c2"}, *(**recv.X).C)
		assert.DeepEqual(t, "d1", *(**recv.X).D)
		assert.DeepEqual(t, "y1", recv.Y)
		assert.DeepEqual(t, (*string)(nil), recv.Z)
		t.Logf("[%d] F: %#v", i, recv.F)
		t.Logf("[%d] F1: %#v", i, recv.F1)
		t.Logf("[%d] Fs: %#v", i, recv.Fs)
		t.Logf("[%d] Fs1: %#v", i, recv.Fs1)
		if len(recv.Fs1) > 0 {
			t.Logf("[%d] Fs1[0]: %#v", i, recv.Fs1[0])
		}
	}
}

func TestFormNum(t *testing.T) {
	type Recv struct {
		X **struct {
			A []int     `form:"a"`
			B int32     `form:"b"`
			C *[]uint16 `form:"c,required"`
			D *float32  `form:"d"`
		}
		Y bool   `form:"y,required"`
		Z *int64 `form:"z"`
	}
	values := make(url.Values)
	values.Add("a", "11")
	values.Add("a", "12")
	values.Add("b", "-21")
	values.Add("c", "31")
	values.Add("c", "32")
	values.Add("d", "41")
	values.Add("d", "42")
	values.Add("y", "1")
	for _, f := range []files{nil, {
		"f1": []file{
			newFile("txt", strings.NewReader("f11 text.")),
		},
	}} {
		contentType, bodyReader := newFormBody2(values, f)
		header := make(http.Header)
		header.Set("Content-Type", contentType)
		req := newRequest("", header, nil, bodyReader)
		recv := new(Recv)

		err := DefaultBinder.Bind(req.Req, nil, recv)
		if err != nil {
			t.Error(err)
		}
		assert.DeepEqual(t, []int{11, 12}, (**recv.X).A)
		assert.DeepEqual(t, int32(-21), (**recv.X).B)
		assert.DeepEqual(t, &[]uint16{31, 32}, (**recv.X).C)
		assert.DeepEqual(t, float32(41), *(**recv.X).D)
		assert.DeepEqual(t, true, recv.Y)
		assert.DeepEqual(t, (*int64)(nil), recv.Z)
	}
}

// FIXME: content-type 裁剪
func TestJSON(t *testing.T) {
	type metric string
	type count int32
	type ZS struct {
		Z *int64
	}
	type Recv struct {
		X **struct {
			A []string          `json:"a"`
			B int32             `json:""`
			C *[]uint16         `json:",required"`
			D *float32          `json:"d"`
			E metric            `json:"e"`
			F count             `json:"f"`
			M map[string]string `json:"m"`
		}
		Y string `json:"y,required"`
		ZS
	}

	bodyReader := strings.NewReader(`{
		"X": {
			"a": ["a1","a2"],
			"B": 21,
			"C": [31,32],
			"d": 41,
			"e": "qps",
			"f": 100,
			"m": {"a":"x"}
		},
		"Z": 6
	}`)

	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	req := newRequest("", header, nil, bodyReader)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	//assert.DeepEqual(t, &binding.Error{ErrType: "binding", FailField: "y", Msg: "missing required parameter"}, err)
	assert.DeepEqual(t, []string{"a1", "a2"}, (**recv.X).A)
	assert.DeepEqual(t, int32(21), (**recv.X).B)
	assert.DeepEqual(t, &[]uint16{31, 32}, (**recv.X).C)
	assert.DeepEqual(t, float32(41), *(**recv.X).D)
	assert.DeepEqual(t, metric("qps"), (**recv.X).E)
	assert.DeepEqual(t, count(100), (**recv.X).F)
	assert.DeepEqual(t, map[string]string{"a": "x"}, (**recv.X).M)
	assert.DeepEqual(t, "", recv.Y)
	assert.DeepEqual(t, (int64)(6), *recv.Z)
}

// FIXME: 非 stuct 绑定，直接走 unmarshal
func TestNonstruct(t *testing.T) {
	bodyReader := strings.NewReader(`{
		"X": {
			"a": ["a1","a2"],
			"B": 21,
			"C": [31,32],
			"d": 41,
			"e": "qps",
			"f": 100
		},
		"Z": 6
	}`)

	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	req := newRequest("", header, nil, bodyReader)
	var recv interface{}

	err := DefaultBinder.Bind(req.Req, nil, &recv)
	if err != nil {
		t.Error(err)
	}
	b, err := json.Marshal(recv)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", b)

	bodyReader = strings.NewReader("b=334ddddd&token=yoMba34uspjVQEbhflgTRe2ceeDFUK32&type=url_verification")
	header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req = newRequest("", header, nil, bodyReader)
	recv = nil
	err = DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	b, err = json.Marshal(recv)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", b)
}

type testPathParams struct{}

func (testPathParams) Get(name string) (string, bool) {
	switch name {
	case "a":
		return "a1", true
	case "b":
		return "-21", true
	case "c":
		return "31", true
	case "d":
		return "41", true
	case "y":
		return "y1", true
	case "name":
		return "henrylee2cn", true
	default:
		return "", false
	}
}

func TestPath(t *testing.T) {
	type Recv struct {
		X **struct {
			A []string  `path:"a"`
			B int32     `path:"b"`
			C *[]uint16 `path:"c,required"`
			D *float32  `path:"d"`
		}
		Y string `path:"y,required"`
		Z *int64
	}

	req := newRequest("", nil, nil, nil)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, new(testPathParams), recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, []string{"a1"}, (**recv.X).A)
	assert.DeepEqual(t, int32(-21), (**recv.X).B)
	assert.DeepEqual(t, &[]uint16{31}, (**recv.X).C)
	assert.DeepEqual(t, float32(41), *(**recv.X).D)
	assert.DeepEqual(t, "y1", recv.Y)
	assert.DeepEqual(t, (*int64)(nil), recv.Z)
}

type testPathParams2 struct{}

func (testPathParams2) Get(name string) (string, bool) {
	switch name {
	case "e":
		return "123", true
	default:
		return "", false
	}
}

// FIXME: 复杂类型的默认值
func TestDefault(t *testing.T) {
	type S struct {
		SS string `json:"ss"`
	}

	type Recv struct {
		X **struct {
			A []string `path:"a" json:"a"`
			B int32    `path:"b" default:"32"`
			C bool     `json:"c" default:"true"`
			D *float32 `default:"123.4"`
			//E          *[]string          `default:"['a','b','c','d,e,f']"`
			//F          map[string]string  `default:"{'a':'\"\\'1','\"b':'c','c':'2'}"`
			//G          map[string]int64   `default:"{'a':1,'b':2,'c':3}"`
			//H          map[string]float64 `default:"{'a':0.1,'b':1.2,'c':2.3}"`
			//I          map[string]float64 `default:"{'\"a\"':0.1,'b':1.2,'c':2.3}"`
			Empty      string `default:""`
			Null       string `default:""`
			CommaSpace string `default:",a:c "`
			Dash       string `default:"-"`
			// InvalidInt int                `default:"abc"`
			// InvalidMap map[string]string  `default:"abc"`
		}
		Y string `json:"y" default:"y1"`
		Z int64
		W string `json:"w"`
		//V []int64   `json:"u" default:"[1,2,3]"`
		//U []float32 `json:"u" default:"[1.1,2,3]"`
		T *string `json:"t" default:"t1"`
		//S S         `default:"{'ss':'test'}"`
		//O *S        `default:"{'ss':'test2'}"`
		//Complex map[string][]map[string][]int64 `default:"{'a':[{'aa':[1,2,3], 'bb':[4,5]}],'b':[{}]}"`
	}

	bodyReader := strings.NewReader(`{
		"X": {
			"a": ["a1","a2"]
		},
		"Z": 6
	}`)

	// var nilMap map[string]string
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	req := newRequest("", header, nil, bodyReader)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, new(testPathParams2), recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, []string{"a1", "a2"}, (**recv.X).A)
	assert.DeepEqual(t, int32(32), (**recv.X).B)
	assert.DeepEqual(t, true, (**recv.X).C)
	assert.DeepEqual(t, float32(123.4), *(**recv.X).D)
	//assert.DeepEqual(t, []string{"a", "b", "c", "d,e,f"}, *(**recv.X).E)
	//assert.DeepEqual(t, map[string]string{"a": "\"'1", "\"b": "c", "c": "2"}, (**recv.X).F)
	//assert.DeepEqual(t, map[string]int64{"a": 1, "b": 2, "c": 3}, (**recv.X).G)
	//assert.DeepEqual(t, map[string]float64{"a": 0.1, "b": 1.2, "c": 2.3}, (**recv.X).H)
	//assert.DeepEqual(t, map[string]float64{"\"a\"": 0.1, "b": 1.2, "c": 2.3}, (**recv.X).I)
	assert.DeepEqual(t, "", (**recv.X).Empty)
	assert.DeepEqual(t, "", (**recv.X).Null)
	assert.DeepEqual(t, ",a:c ", (**recv.X).CommaSpace)
	assert.DeepEqual(t, "-", (**recv.X).Dash)
	// assert.DeepEqual(t, 0, (**recv.X).InvalidInt)
	// assert.DeepEqual(t, nilMap, (**recv.X).InvalidMap)
	assert.DeepEqual(t, "y1", recv.Y)
	assert.DeepEqual(t, "t1", *recv.T)
	assert.DeepEqual(t, int64(6), recv.Z)
	//assert.DeepEqual(t, []int64{1, 2, 3}, recv.V)
	//assert.DeepEqual(t, []float32{1.1, 2, 3}, recv.U)
	//assert.DeepEqual(t, S{SS: "test"}, recv.S)
	//assert.DeepEqual(t, &S{SS: "test2"}, recv.O)
	//assert.DeepEqual(t, map[string][]map[string][]int64{"a": {{"aa": {1, 2, 3}, "bb": []int64{4, 5}}}, "b": {map[string][]int64{}}}, recv.Complex)
}

// FIXME: query 和 form getter 的优先级
func TestAuto(t *testing.T) {
	type Recv struct {
		A string
		B string
		C string
		D string `query:"D,required" form:"D,required"`
		E string `cookie:"e" json:"e"`
	}
	query := make(url.Values)
	query.Add("A", "a")
	query.Add("B", "b")
	query.Add("C", "c")
	query.Add("D", "d-from-query")
	contentType, bodyReader, err := newJSONBody(map[string]string{"e": "e-from-jsonbody"})
	if err != nil {
		t.Error(err)
	}
	header := make(http.Header)
	header.Set("Content-Type", contentType)
	req := newRequest("http://localhost/?"+query.Encode(), header, []*http.Cookie{
		{Name: "e", Value: "e-from-cookie"},
	}, bodyReader)
	recv := new(Recv)

	err = DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "a", recv.A)
	assert.DeepEqual(t, "b", recv.B)
	assert.DeepEqual(t, "c", recv.C)
	assert.DeepEqual(t, "d-from-query", recv.D)
	assert.DeepEqual(t, "e-from-cookie", recv.E)

	query = make(url.Values)
	query.Add("D", "d-from-query")
	form := make(url.Values)
	form.Add("B", "b")
	form.Add("C", "c")
	form.Add("D", "d-from-form")
	contentType, bodyReader = newFormBody2(form, nil)
	header = make(http.Header)
	header.Set("Content-Type", contentType)
	req = newRequest("http://localhost/?"+query.Encode(), header, nil, bodyReader)
	recv = new(Recv)
	err = DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "", recv.A)
	assert.DeepEqual(t, "b", recv.B)
	assert.DeepEqual(t, "c", recv.C)
	assert.DeepEqual(t, "d-from-form", recv.D)
}

// FIXME: 自定义验证函数 & TIME 类型内置
func TestTypeUnmarshal(t *testing.T) {
	type Recv struct {
		A time.Time   `form:"t1"`
		B *time.Time  `query:"t2"`
		C []time.Time `query:"t2"`
	}
	query := make(url.Values)
	query.Add("t2", "2019-09-04T14:05:24+08:00")
	query.Add("t2", "2019-09-04T18:05:24+08:00")
	form := make(url.Values)
	form.Add("t1", "2019-09-03T18:05:24+08:00")
	contentType, bodyReader := newFormBody2(form, nil)
	header := make(http.Header)
	header.Set("Content-Type", contentType)
	req := newRequest("http://localhost/?"+query.Encode(), header, nil, bodyReader)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	t1, err := time.Parse(time.RFC3339, "2019-09-03T18:05:24+08:00")
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, t1, recv.A)
	t21, err := time.Parse(time.RFC3339, "2019-09-04T14:05:24+08:00")
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, t21, *recv.B)
	t22, err := time.Parse(time.RFC3339, "2019-09-04T18:05:24+08:00")
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, []time.Time{t21, t22}, recv.C)
	t.Logf("%v", recv)
}

// FIXME: JSON required 校验
func TestOption(t *testing.T) {
	type Recv struct {
		X *struct {
			C int `json:"c,required"`
			D int `json:"d"`
		} `json:"X"`
		Y string `json:"y"`
	}
	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	bodyReader := strings.NewReader(`{
			"X": {
				"c": 21,
				"d": 41
			},
			"y": "y1"
		}`)
	req := newRequest("", header, nil, bodyReader)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 21, recv.X.C)
	assert.DeepEqual(t, 41, recv.X.D)
	assert.DeepEqual(t, "y1", recv.Y)

	bodyReader = strings.NewReader(`{
			"X": {
			},
			"y": "y1"
		}`)
	req = newRequest("", header, nil, bodyReader)
	recv = new(Recv)
	err = DefaultBinder.Bind(req.Req, nil, recv)
	//assert.DeepEqual(t, err.Error(), "binding: expr_path=X.c, cause=missing required parameter")
	assert.DeepEqual(t, 0, recv.X.C)
	assert.DeepEqual(t, 0, recv.X.D)
	assert.DeepEqual(t, "y1", recv.Y)

	bodyReader = strings.NewReader(`{
			"y": "y1"
		}`)
	req = newRequest("", header, nil, bodyReader)
	recv = new(Recv)
	err = DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, recv.X == nil)
	assert.DeepEqual(t, "y1", recv.Y)

	type Recv2 struct {
		X *struct {
			C int `json:"c,required"`
			D int `json:"d"`
		} `json:"X,required"`
		Y string `json:"y"`
	}
	bodyReader = strings.NewReader(`{
			"y": "y1"
		}`)
	req = newRequest("", header, nil, bodyReader)
	recv2 := new(Recv2)
	err = DefaultBinder.Bind(req.Req, nil, recv2)
	//assert.DeepEqual(t, err.Error(), "binding: expr_path=X, cause=missing required parameter")
	assert.True(t, recv2.X == nil)
	assert.DeepEqual(t, "y1", recv2.Y)
}

func newRequest(u string, header http.Header, cookies []*http.Cookie, bodyReader io.Reader) *mockRequest {
	if header == nil {
		header = make(http.Header)
	}
	var method = "GET"
	var body []byte
	if bodyReader != nil {
		body, _ = ioutil.ReadAll(bodyReader)
		method = "POST"
	}
	if u == "" {
		u = "http://localhost"
	}
	req := newMockRequest()
	req.SetRequestURI(u)
	for k, v := range header {
		for _, val := range v {
			req.Req.Header.Add(k, val)
		}
	}
	if len(body) != 0 {
		req.SetBody(body)
		req.Req.Header.SetContentLength(len(body))
	}
	req.Req.SetMethod(method)
	for _, c := range cookies {
		req.Req.Header.SetCookie(c.Name, c.Value)

	}
	return req
}

func TestQueryStringIssue(t *testing.T) {
	type Timestamp struct {
		time.Time
	}
	type Recv struct {
		Name *string    `query:"name"`
		T    *Timestamp `query:"t"`
	}
	req := newRequest("http://localhost:8080/?name=test", nil, nil, nil)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "test", *recv.Name)
	assert.DeepEqual(t, (*Timestamp)(nil), recv.T)
}

func TestQueryTypes(t *testing.T) {
	type metric string
	type count int32
	type metrics []string
	type filter struct {
		Col1 string
	}

	type Recv struct {
		A metric
		B count
		C *count
		D metrics `query:"D,required" form:"D,required"`
		E metric  `cookie:"e" json:"e"`
		F filter  `json:"f"`
	}
	query := make(url.Values)
	query.Add("A", "qps")
	query.Add("B", "123")
	query.Add("C", "321")
	query.Add("D", "dau")
	query.Add("D", "dnu")
	contentType, bodyReader, err := newJSONBody(
		map[string]interface{}{
			"e": "e-from-jsonbody",
			"f": filter{Col1: "abc"},
		},
	)
	if err != nil {
		t.Error(err)
	}
	header := make(http.Header)
	header.Set("Content-Type", contentType)
	req := newRequest("http://localhost/?"+query.Encode(), header, []*http.Cookie{
		{Name: "e", Value: "e-from-cookie"},
	}, bodyReader)
	recv := new(Recv)

	err = DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, metric("qps"), recv.A)
	assert.DeepEqual(t, count(123), recv.B)
	assert.DeepEqual(t, count(321), *recv.C)
	assert.DeepEqual(t, metrics{"dau", "dnu"}, recv.D)
	assert.DeepEqual(t, metric("e-from-cookie"), recv.E)
	assert.DeepEqual(t, filter{Col1: "abc"}, recv.F)
}

func TestNoTagIssue(t *testing.T) {
	type x int
	type T struct {
		x
		x2 x
		a  int
		B  int
	}
	req := newRequest("http://localhost:8080/?x=11&x2=12&a=1&B=2", nil, nil, nil)
	recv := new(T)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, x(0), recv.x)
	assert.DeepEqual(t, x(0), recv.x2)
	assert.DeepEqual(t, 0, recv.a)
	assert.DeepEqual(t, 2, recv.B)
}

// DIFF: go-tagexpr 会对保留 t.Q的结构体信息，而目前的实现不会 t.Q 做特殊处理，会直接拆开。有需求也可以加上
func TestRegTypeUnmarshal(t *testing.T) {
	type Q struct {
		A int
		B string
	}
	type T struct {
		Q  Q    `query:"q"`
		Qs []*Q `query:"qs"`
	}
	var values = url.Values{}
	b, err := json.Marshal(Q{A: 2, B: "y"})
	if err != nil {
		t.Error(err)
	}
	values.Add("q", string(b))
	bs, err := json.Marshal([]Q{{A: 1, B: "x"}, {A: 2, B: "y"}})
	values.Add("qs", string(bs))
	req := newRequest("http://localhost:8080/?"+values.Encode(), nil, nil, nil)
	recv := new(T)

	err = DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, 2, recv.Q.A)
	assert.DeepEqual(t, "y", recv.Q.B)
	assert.DeepEqual(t, 1, recv.Qs[0].A)
	assert.DeepEqual(t, "x", recv.Qs[0].B)
	assert.DeepEqual(t, 2, recv.Qs[1].A)
	assert.DeepEqual(t, "y", recv.Qs[1].B)
}

//func TestPathnameBUG(t *testing.T) {
//	type Currency struct {
//		CurrencyName   *string `form:"currency_name,required" json:"currency_name,required" protobuf:"bytes,1,req,name=currency_name,json=currencyName" query:"currency_name,required"`
//		CurrencySymbol *string `form:"currency_symbol,required" json:"currency_symbol,required" protobuf:"bytes,2,req,name=currency_symbol,json=currencySymbol" query:"currency_symbol,required"`
//		SymbolPosition *int32  `form:"symbol_position,required" json:"symbol_position,required" protobuf:"varint,3,req,name=symbol_position,json=symbolPosition" query:"symbol_position,required"`
//		DecimalPlaces  *int32  `form:"decimal_places,required" json:"decimal_places,required" protobuf:"varint,4,req,name=decimal_places,json=decimalPlaces" query:"decimal_places,required"` // 56x56
//		DecimalSymbol  *string `form:"decimal_symbol,required" json:"decimal_symbol,required" protobuf:"bytes,5,req,name=decimal_symbol,json=decimalSymbol" query:"decimal_symbol,required"`
//		Separator      *string `form:"separator,required" json:"separator,required" protobuf:"bytes,6,req,name=separator" query:"separator,required"`
//		SeparatorIndex *string `form:"separator_index,required" json:"separator_index,required" protobuf:"bytes,7,req,name=separator_index,json=separatorIndex" query:"separator_index,required"`
//		Between        *string `form:"between,required" json:"between,required" protobuf:"bytes,8,req,name=between" query:"between,required"`
//		MinPrice       *string `form:"min_price" json:"min_price,omitempty" protobuf:"bytes,9,opt,name=min_price,json=minPrice" query:"min_price"`
//		MaxPrice       *string `form:"max_price" json:"max_price,omitempty" protobuf:"bytes,10,opt,name=max_price,json=maxPrice" query:"max_price"`
//	}
//
//	type CurrencyData struct {
//		Amount   *string   `form:"amount,required" json:"amount,required" protobuf:"bytes,1,req,name=amount" query:"amount,required"`
//		Currency *Currency `form:"currency,required" json:"currency,required" protobuf:"bytes,2,req,name=currency" query:"currency,required"`
//	}
//
//	type ExchangeCurrencyRequest struct {
//		PromotionRegion *string       `form:"promotion_region,required" json:"promotion_region,required" protobuf:"bytes,1,req,name=promotion_region,json=promotionRegion" query:"promotion_region,required"`
//		Currency        *CurrencyData `form:"currency,required" json:"currency,required" protobuf:"bytes,2,req,name=currency" query:"currency,required"`
//		Version         *int32        `json:"version,omitempty" path:"version" protobuf:"varint,100,opt,name=version"`
//	}
//
//	z := &ExchangeCurrencyRequest{}
//	v := ameda.InitSampleValue(reflect.TypeOf(z), 10).Interface().(*ExchangeCurrencyRequest)
//	b, err := json.MarshalIndent(v, "", "  ")
//	t.Log(string(b))
//	header := make(http.Header)
//	header.Set("Content-Type", "application/json;charset=utf-8")
//	req := newRequest("http://localhost", header, nil, bytes.NewReader(b))
//	recv := new(ExchangeCurrencyRequest)
//
//	err = DefaultBinder.Bind(req.Req, nil, recv)
//		if err != nil {
//
//	assert.DeepEqual(t, v, recv)
//}

// FIXME: json unmarshal 后其他 required 没必要 校验 required
func TestPathnameBUG2(t *testing.T) {
	type CurrencyData struct {
		z      int
		Amount *string `form:"amount,required" json:"amount,required" protobuf:"bytes,1,req,name=amount" query:"amount,required"`
		Name   *string `form:"name,required" json:"name,required" protobuf:"bytes,2,req,name=name" query:"name,required"`
		Symbol *string `form:"symbol" json:"symbol,omitempty" protobuf:"bytes,3,opt,name=symbol" query:"symbol"`
	}
	type TimeRange struct {
		z         int
		StartTime *int64 `form:"start_time,required" json:"start_time,required" protobuf:"varint,1,req,name=start_time,json=startTime" query:"start_time,required"`
		EndTime   *int64 `form:"end_time,required" json:"end_time,required" protobuf:"varint,2,req,name=end_time,json=endTime" query:"end_time,required"`
	}
	type CreateFreeShippingRequest struct {
		z                int
		PromotionName    *string       `form:"promotion_name,required" json:"promotion_name,required" protobuf:"bytes,1,req,name=promotion_name,json=promotionName" query:"promotion_name,required"`
		PromotionRegion  *string       `form:"promotion_region,required" json:"promotion_region,required" protobuf:"bytes,2,req,name=promotion_region,json=promotionRegion" query:"promotion_region,required"`
		TimeRange        *TimeRange    `form:"time_range,required" json:"time_range,required" protobuf:"bytes,3,req,name=time_range,json=timeRange" query:"time_range,required"`
		PromotionBudget  *CurrencyData `form:"promotion_budget,required" json:"promotion_budget,required" protobuf:"bytes,4,req,name=promotion_budget,json=promotionBudget" query:"promotion_budget,required"`
		Loaded_SellerIds []string      `form:"loaded_Seller_ids" json:"loaded_Seller_ids,omitempty" protobuf:"bytes,5,rep,name=loaded_Seller_ids,json=loadedSellerIds" query:"loaded_Seller_ids"`
		Version          *int32        `json:"version,omitempty" path:"version" protobuf:"varint,100,opt,name=version"`
	}

	// z := &CreateFreeShippingRequest{}
	// v := ameda.InitSampleValue(reflect.TypeOf(z), 10).Interface().(*CreateFreeShippingRequest)
	// b, err := json.MarshalIndent(v, "", "  ")
	// t.Log(string(b))
	b := []byte(`{
    "promotion_name": "mu",
    "promotion_region": "ID",
    "time_range": {
        "start_time": 1616420139,
        "end_time": 1616520139
    },
    "promotion_budget": {
        "amount":"10000000",
        "name":"USD",
        "symbol":"$"
    },
    "loaded_Seller_ids": [
        "7493989780026655762","11111","111212121"
    ]
}`)
	var v = new(CreateFreeShippingRequest)
	err := json.Unmarshal(b, v)
	if err != nil {
		t.Error(err)
	}

	header := make(http.Header)
	header.Set("Content-Type", "application/json;charset=utf-8")
	req := newRequest("http://localhost", header, nil, bytes.NewReader(b))
	recv := new(CreateFreeShippingRequest)

	err = DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}

	assert.DeepEqual(t, v, recv)
}

// FIXME: json unmarshal 后的其他 tag 的 required 的校验
func TestRequiredBUG(t *testing.T) {
	type Currency struct {
		currencyName   *string `form:"currency_name,required" json:"currency_name,required" protobuf:"bytes,1,req,name=currency_name,json=currencyName" query:"currency_name,required"`
		CurrencySymbol *string `form:"currency_symbol,required" json:"currency_symbol,required" protobuf:"bytes,2,req,name=currency_symbol,json=currencySymbol" query:"currency_symbol,required"`
	}

	type CurrencyData struct {
		Amount *string              `form:"amount,required" json:"amount,required" protobuf:"bytes,1,req,name=amount" query:"amount,required"`
		Slice  []*Currency          `form:"slice,required" json:"slice,required" protobuf:"bytes,2,req,name=slice" query:"slice,required"`
		Map    map[string]*Currency `form:"map,required" json:"map,required" protobuf:"bytes,2,req,name=map" query:"map,required"`
	}

	type ExchangeCurrencyRequest struct {
		PromotionRegion *string       `form:"promotion_region,required" json:"promotion_region,required" protobuf:"bytes,1,req,name=promotion_region,json=promotionRegion" query:"promotion_region,required"`
		Currency        *CurrencyData `form:"currency,required" json:"currency,required" protobuf:"bytes,2,req,name=currency" query:"currency,required"`
	}

	z := &ExchangeCurrencyRequest{}
	// v := ameda.InitSampleValue(reflect.TypeOf(z), 10).Interface().(*ExchangeCurrencyRequest)
	b := []byte(`{
          "promotion_region": "?",
          "currency": {
            "amount": "?",
            "slice": [
              {
                "currency_symbol": "?"
              }
            ],
            "map": {
              "?": {
                "currency_name": "?"
              }
            }
          }
        }`)
	json.Unmarshal(b, z)
	header := make(http.Header)
	header.Set("Content-Type", "application/json;charset=utf-8")
	req := newRequest("http://localhost", header, nil, bytes.NewReader(b))
	recv := new(ExchangeCurrencyRequest)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	assert.DeepEqual(t, err.Error(), "validating: expr_path=Currency.Slice[0].currencyName, cause=invalid")
	assert.DeepEqual(t, z, recv)
}

func TestIssue25(t *testing.T) {
	type Recv struct {
		A string
	}
	header := make(http.Header)
	header.Set("A", "from header")
	cookies := []*http.Cookie{
		{Name: "A", Value: "from cookie"},
	}
	req := newRequest("/1", header, cookies, nil)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	//assert.DeepEqual(t, "from cookie", recv.A)

	header2 := make(http.Header)
	header2.Set("A", "from header")
	cookies2 := []*http.Cookie{}
	req2 := newRequest("/2", header2, cookies2, nil)
	recv2 := new(Recv)
	err2 := DefaultBinder.Bind(req2.Req, nil, recv2)
	if err2 != nil {
		t.Error(err2)
	}
	assert.DeepEqual(t, "from header", recv2.A)
}

func TestIssue26(t *testing.T) {
	type Recv struct {
		Type            string `json:"type,required" vd:"($=='update_target_threshold' && (TargetThreshold)$!='-1') || ($=='update_status' && (Status)$!='-1')"`
		RuleName        string `json:"rule_name,required" vd:"regexp('^rule[0-9]+$')"`
		TargetThreshold string `json:"target_threshold" vd:"regexp('^-?[0-9]+(\\.[0-9]+)?$')"`
		Status          string `json:"status" vd:"$=='0' || $=='1'"`
		Operator        string `json:"operator,required" vd:"len($)>0"`
	}

	b := []byte(`{
    "status": "1",
    "adv": "11520",
    "target_deep_external_action": "39",
    "package": "test.bytedance.com",
    "previous_target_threshold": "0.6",
    "deep_external_action": "675",
    "rule_name": "rule2",
    "deep_bid_type": "54",
    "modify_time": "2021-08-24:14:35:20",
    "aid": "111",
    "operator": "yanghaoze",
    "external_action": "76",
    "target_threshold": "0.1",
    "type": "update_status"
}`)

	recv := new(Recv)
	err := json.Unmarshal(b, recv)
	if err != nil {
		t.Error(err)
	}

	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	header.Set("A", "from header")
	cookies := []*http.Cookie{
		{Name: "A", Value: "from cookie"},
	}

	req := newRequest("/1", header, cookies, bytes.NewReader(b))

	recv2 := new(Recv)
	err = DefaultBinder.Bind(req.Req, nil, recv2)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, recv, recv2)
}

// FIXME: json unmarshal 后，默认值的问题
func TestDefault2(t *testing.T) {
	type Recv struct {
		X **struct {
			Dash string `default:"xxxx"`
		}
	}
	bodyReader := strings.NewReader(`{
		"X": {
			"Dash": "hello Dash"
		}
	}`)
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	req := newRequest("", header, nil, bodyReader)
	recv := new(Recv)

	err := DefaultBinder.Bind(req.Req, nil, recv)
	if err != nil {
		t.Error(err)
	}
	assert.DeepEqual(t, "hello Dash", (**recv.X).Dash)
}

func newFormBody(values, files url.Values) (contentType string, bodyReader io.Reader, err error) {
	if len(files) == 0 {
		return "application/x-www-form-urlencoded", strings.NewReader(values.Encode()), nil
	}
	var rw = bytes.NewBuffer(make([]byte, 32*1024*len(files)))
	var bodyWriter = multipart.NewWriter(rw)
	var buf = make([]byte, 32*1024)
	var fileWriter io.Writer
	var f *os.File
	for fieldName, postfiles := range files {
		for _, fileName := range postfiles {
			fileWriter, err = bodyWriter.CreateFormFile(fieldName, fileName)
			if err != nil {
				return
			}
			f, err = os.Open(fileName)
			if err != nil {
				return
			}
			_, err = io.CopyBuffer(fileWriter, f, buf)
			f.Close()
			if err != nil {
				return
			}
		}
	}
	for k, v := range values {
		for _, vv := range v {
			bodyWriter.WriteField(k, vv)
		}
	}
	bodyWriter.Close()
	return bodyWriter.FormDataContentType(), rw, nil
}

type (
	files map[string][]file
	file  interface {
		Name() string
		Read(p []byte) (n int, err error)
	}
)

func newFormBody2(values url.Values, files files) (contentType string, bodyReader io.Reader) {
	if len(files) == 0 {
		return "application/x-www-form-urlencoded", strings.NewReader(values.Encode())
	}
	var pr, pw = io.Pipe()
	var bodyWriter = multipart.NewWriter(pw)
	var fileWriter io.Writer
	var buf = make([]byte, 32*1024)
	go func() {
		for fieldName, postfiles := range files {
			for _, file := range postfiles {
				fileWriter, _ = bodyWriter.CreateFormFile(fieldName, file.Name())
				io.CopyBuffer(fileWriter, file, buf)
			}
		}
		for k, v := range values {
			for _, vv := range v {
				bodyWriter.WriteField(k, vv)
			}
		}
		bodyWriter.Close()
		pw.Close()
	}()
	return bodyWriter.FormDataContentType(), pr
}

func newFile(name string, bodyReader io.Reader) file {
	return &fileReader{name, bodyReader}
}

// fileReader file name and bytes.
type fileReader struct {
	name       string
	bodyReader io.Reader
}

func (f *fileReader) Name() string {
	return f.name
}

func (f *fileReader) Read(p []byte) (int, error) {
	return f.bodyReader.Read(p)
}

func newJSONBody(v interface{}) (contentType string, bodyReader io.Reader, err error) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	return "application/json;charset=utf-8", bytes.NewReader(b), nil
}
