// Copyright 2019 Bytedance Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tagexpr_test

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/cloudwego/hertz/internal/tagexpr"
)

func assertEqual(t *testing.T, v1, v2 interface{}, msgs ...interface{}) {
	t.Helper()
	if reflect.DeepEqual(v1, v2) {
		return
	}
	t.Fatal(fmt.Sprintf("not equal %v %v", v1, v2) + "\n" + fmt.Sprint(msgs...))
}

func BenchmarkTagExpr(b *testing.B) {
	type T struct {
		a int `bench:"$%3"`
	}
	vm := tagexpr.New("bench")
	vm.MustRun(new(T)) // warm up
	b.ReportAllocs()
	b.ResetTimer()
	t := &T{10}
	for i := 0; i < b.N; i++ {
		tagExpr, err := vm.Run(t)
		if err != nil {
			b.FailNow()
		}
		if tagExpr.EvalFloat("a") != 1 {
			b.FailNow()
		}
	}
}

func BenchmarkReflect(b *testing.B) {
	type T struct {
		a int `remainder:"3"`
	}
	b.ReportAllocs()
	b.ResetTimer()
	t := &T{1}
	for i := 0; i < b.N; i++ {
		v := reflect.ValueOf(t).Elem()
		ft, ok := v.Type().FieldByName("a")
		if !ok {
			b.FailNow()
		}
		x, err := strconv.ParseInt(ft.Tag.Get("remainder"), 10, 64)
		if err != nil {
			b.FailNow()
		}
		fv := v.FieldByName("a")
		if fv.Int()%x != 1 {
			b.FailNow()
		}
	}
}

func Test(t *testing.T) {
	g := &struct {
		_ int
		h string `tagexpr:"$"`
		s []string
		m map[string][]string
	}{
		h: "haha",
		s: []string{"1"},
		m: map[string][]string{"0": {"2"}},
	}
	d := "ddd"
	e := new(int)
	*e = 3
	type iface interface{}
	cases := []struct {
		tagName   string
		structure interface{}
		tests     map[string]interface{}
	}{
		{
			tagName: "tagexpr",
			structure: &struct {
				A     int              `tagexpr:"$>0&&$<10&&!''&&!!!0&&!nil&&$"`
				A2    int              `tagexpr:"@:$>0&&$<10"`
				b     string           `tagexpr:"is:$=='test';msg:sprintf('expect: test, but got: %s',$)"`
				c     float32          `tagexpr:"(A)$+$"`
				d     *string          `tagexpr:"$"`
				e     **int            `tagexpr:"$"`
				f     *[3]int          `tagexpr:"x:len($)"`
				g     string           `tagexpr:"x:!regexp('xxx',$);y:regexp('g\\d{3}$')"`
				h     []string         `tagexpr:"x:$[1];y:$[10]"`
				i     map[string]int   `tagexpr:"x:$['a'];y:$[0];z:$==nil"`
				i2    *map[string]int  `tagexpr:"x:$['a'];y:$[0];z:$"`
				j, j2 iface            `tagexpr:"@:$==1;y:$"`
				k     *iface           `tagexpr:"$==nil"`
				m     *struct{ i int } `tagexpr:"@:$;x:$['a']['x']"`
			}{
				A:  5.0,
				A2: 5.0,
				b:  "x",
				c:  1,
				d:  &d,
				e:  &e,
				f:  new([3]int),
				g:  "g123",
				h:  []string{"", "hehe"},
				i:  map[string]int{"a": 7},
				j2: iface(1),
				m:  &struct{ i int }{1},
			},
			tests: map[string]interface{}{
				"A":     true,
				"A2":    true,
				"b@is":  false,
				"b@msg": "expect: test, but got: x",
				"c":     6.0,
				"d":     d,
				"e":     float64(*e),
				"f@x":   float64(3),
				"g@x":   true,
				"g@y":   true,
				"h@x":   "hehe",
				"h@y":   nil,
				"i@x":   7.0,
				"i@y":   nil,
				"i@z":   false,
				"i2@x":  nil,
				"i2@y":  nil,
				"i2@z":  nil,
				"j":     false,
				"j@y":   nil,
				"j2":    true,
				"j2@y":  1.0,
				"k":     true,
				"m":     &struct{ i int }{1},
				"m@x":   nil,
			},
		},
		{
			tagName: "tagexpr",
			structure: &struct {
				A int    `tagexpr:"$>0&&$<10"`
				b string `tagexpr:"is:$=='test';msg:sprintf('expect: test, but got: %s',$)"`
				c struct {
					_ int
					d bool `tagexpr:"$"`
				}
				e *struct {
					_ int
					f bool `tagexpr:"$"`
				}
				g **struct {
					_ int
					h string `tagexpr:"$"`
					s []string
					m map[string][]string
				} `tagexpr:"$['h']"`
				i string  `tagexpr:"(g.s)$[0]+(g.m)$['0'][0]==$"`
				j bool    `tagexpr:"!$"`
				k int     `tagexpr:"!$"`
				m *int    `tagexpr:"$==nil"`
				n *bool   `tagexpr:"$==nil"`
				p *string `tagexpr:"$"`
			}{
				A: 5,
				b: "x",
				c: struct {
					_ int
					d bool `tagexpr:"$"`
				}{d: true},
				e: &struct {
					_ int
					f bool `tagexpr:"$"`
				}{f: true},
				g: &g,
				i: "12",
			},
			tests: map[string]interface{}{
				"A":     true,
				"b@is":  false,
				"b@msg": "expect: test, but got: x",
				"c.d":   true,
				"e.f":   true,
				"g":     "haha",
				"g.h":   "haha",
				"i":     true,
				"j":     true,
				"k":     true,
				"m":     true,
				"n":     true,
				"p":     nil,
			},
		},
		{
			tagName: "p",
			structure: &struct {
				q *struct {
					x int
				} `p:"(q.x)$"`
			}{},
			tests: map[string]interface{}{
				"q": nil,
			},
		},
	}
	for i, c := range cases {
		vm := tagexpr.New(c.tagName)
		// vm.WarmUp(c.structure)
		tagExpr, err := vm.Run(c.structure)
		if err != nil {
			t.Fatal(err)
		}
		for selector, value := range c.tests {
			val := tagExpr.Eval(selector)
			if !reflect.DeepEqual(val, value) {
				t.Fatalf("Eval Serial: %d, selector: %q, got: %v, expect: %v", i, selector, val, value)
			}
		}
		tagExpr.Range(func(eh *tagexpr.ExprHandler) error {
			es := eh.ExprSelector()
			t.Logf("Range selector: %s, field: %q exprName: %q", es, es.Field(), es.Name())
			value := c.tests[es.String()]
			val := eh.Eval()
			if !reflect.DeepEqual(val, value) {
				t.Fatalf("Range NO: %d, selector: %q, got: %v, expect: %v", i, es, val, value)
			}
			return nil
		})
	}
}

func TestFieldNotInit(t *testing.T) {
	g := &struct {
		_ int
		h string
		s []string
		m map[string][]string
	}{
		h: "haha",
		s: []string{"1"},
		m: map[string][]string{"0": {"2"}},
	}
	structure := &struct {
		A int
		b string
		c struct {
			_ int
			d *bool `expr:"test:nil"`
		}
		e *struct {
			_ int
			f bool
		}
		g **struct {
			_ int
			h string
			s []string
			m map[string][]string
		}
		i string
		j bool
		k int
		m *int
		n *bool
		p *string
	}{
		A: 5,
		b: "x",
		e: &struct {
			_ int
			f bool
		}{f: true},
		g: &g,
		i: "12",
	}
	vm := tagexpr.New("expr")
	e, err := vm.Run(structure)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		fieldSelector string
		value         interface{}
	}{
		{"A", structure.A},
		{"b", structure.b},
		{"c", structure.c},
		{"c._", 0},
		{"c.d", structure.c.d},
		{"e", structure.e},
		{"e._", 0},
		{"e.f", structure.e.f},
		{"g", structure.g},
		{"g._", 0},
		{"g.h", (*structure.g).h},
		{"g.s", (*structure.g).s},
		{"g.m", (*structure.g).m},
		{"i", structure.i},
		{"j", structure.j},
		{"k", structure.k},
		{"m", structure.m},
		{"n", structure.n},
		{"p", structure.p},
	}
	for _, c := range cases {
		fh, _ := e.Field(c.fieldSelector)
		val := fh.Value(false).Interface()
		assertEqual(t, c.value, val, c.fieldSelector)
	}
	var i int
	e.RangeFields(func(fh *tagexpr.FieldHandler) bool {
		val := fh.Value(false).Interface()
		if fh.StringSelector() == "c.d" {
			if fh.EvalFuncs()["c.d@test"] == nil {
				t.Fatal("nil")
			}
		}
		assertEqual(t, cases[i].value, val, fh.StringSelector())
		i++
		return true
	})
	var wall uint64 = 1024
	unix := time.Unix(1549186325, int64(wall))
	e, err = vm.Run(&unix)
	if err != nil {
		t.Fatal(err)
	}
	fh, _ := e.Field("wall")
	val := fh.Value(false).Interface()
	if !reflect.DeepEqual(val, wall) {
		t.Fatalf("Time.wall: got: %v(%[1]T), expect: %v(%[2]T)", val, wall)
	}
}

func TestFieldInitZero(t *testing.T) {
	g := &struct {
		_ int
		h string
		s []string
		m map[string][]string
	}{
		h: "haha",
		s: []string{"1"},
		m: map[string][]string{"0": {"2"}},
	}

	structure := &struct {
		A int
		b string
		c struct {
			_ int
			d *bool
		}
		e *struct {
			_ int
			f bool
		}
		g **struct {
			_ int
			h string
			s []string
			m map[string][]string
		}
		g2 ****struct {
			_ int
			h string
			s []string
			m map[string][]string
		}
		i string
		j bool
		k int
		m *int
		n *bool
		p *string
	}{
		A: 5,
		b: "x",
		e: &struct {
			_ int
			f bool
		}{f: true},
		g: &g,
		i: "12",
	}

	vm := tagexpr.New("")
	e, err := vm.Run(structure)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		fieldSelector string
		value         interface{}
	}{
		{"A", structure.A},
		{"b", structure.b},
		{"c", struct {
			_ int
			d *bool
		}{}},
		{"c._", 0},
		{"c.d", new(bool)},
		{"e", structure.e},
		{"e._", 0},
		{"e.f", structure.e.f},
		{"g", structure.g},
		{"g._", 0},
		{"g.h", (*structure.g).h},
		{"g.s", (*structure.g).s},
		{"g.m", (*structure.g).m},
		{"g2.m", (map[string][]string)(nil)},
		{"i", structure.i},
		{"j", structure.j},
		{"k", structure.k},
		{"m", new(int)},
		{"n", new(bool)},
		{"p", new(string)},
	}
	for _, c := range cases {
		fh, _ := e.Field(c.fieldSelector)
		val := fh.Value(true).Interface()
		assertEqual(t, c.value, val, c.fieldSelector)
	}
}

func TestOperator(t *testing.T) {
	type Tmp1 struct {
		A string `tagexpr:$=="1"||$=="2"||$="3"` //nolint:govet
		B []int  `tagexpr:len($)>=10&&$[0]<10`   //nolint:govet
		C interface{}
	}

	type Tmp2 struct {
		A *Tmp1
		B interface{}
	}

	type Target struct {
		A int             `tagexpr:"-$+$<=10"`
		B int             `tagexpr:"+$-$<=10"`
		C int             `tagexpr:"-$+(M)$*(N)$/$%(D.B)$[2]+$==1"`
		D *Tmp1           `tagexpr:"(D.A)$!=nil"`
		E string          `tagexpr:"((D.A)$=='1'&&len($)>1)||((D.A)$=='2'&&len($)>2)||((D.A)$=='3'&&len($)>3)"`
		F map[string]int  `tagexpr:"x:len($);y:$['a']>10&&$['b']>1"`
		G *map[string]int `tagexpr:"x:$['a']+(F)$['a']>20"`
		H []string        `tagexpr:"len($)>=1&&len($)<10&&$[0]=='123'&&$[1]!='456'"`
		I interface{}     `tagexpr:"$!=nil"`
		K *string         `tagexpr:"len((D.A)$)+len($)<10&&len((D.A)$+$)<10"`
		L **string        `tagexpr:"false"`
		M float64         `tagexpr:"$/2>10&&$%2==0"`
		N *float64        `tagexpr:"($+$*$-$/$+1)/$==$+1"`
		O *[3]float64     `tagexpr:"$[0]>10&&$[0]<20||$[0]>20&&$[0]<30"`
		P *Tmp2           `tagexpr:"x:$!=nil;y:len((P.A.A)$)<=1&&(P.A.B)$[0]==1;z:$['A']['C']==nil;w:$['A']['B'][0]==1;r:$[0][1][2]==3;s1:$[2]==nil;s2:$[0][3]==nil;s3:(ZZ)$;s4:(P.B)$!=nil"`
		Q *Tmp2           `tagexpr:"s1:$['A']['B']!=nil;s2:(Q.A)$['B']!=nil;s3:$['A']['C']==nil;s4:(Q.A)$['C']==nil;s5:(Q.A)$['B'][0]==1;s6:$['X']['Z']==nil"`
	}

	k := "123456"
	n := float64(-12.5)
	o := [3]float64{15, 9, 9}
	cases := []struct {
		tagName   string
		structure interface{}
		tests     map[string]interface{}
	}{
		{
			tagName: "tagexpr",
			structure: &Target{
				A: 5,
				B: 10,
				C: -10,
				D: &Tmp1{A: "3", B: []int{1, 2, 3}},
				E: "1234",
				F: map[string]int{"a": 11, "b": 9},
				G: &map[string]int{"a": 11},
				H: []string{"123", "45"},
				I: struct{}{},
				K: &k,
				L: nil,
				M: float64(30),
				N: &n,
				O: &o,
				P: &Tmp2{A: &Tmp1{A: "3", B: []int{1, 2, 3}}, B: struct{}{}},
				Q: &Tmp2{A: &Tmp1{A: "3", B: []int{1, 2, 3}}, B: struct{}{}},
			},
			tests: map[string]interface{}{
				"A":   true,
				"B":   true,
				"C":   true,
				"D":   true,
				"E":   true,
				"F@x": float64(2),
				"F@y": true,
				"G@x": true,
				"H":   true,
				"I":   true,
				"K":   true,
				"L":   false,
				"M":   true,
				"N":   true,
				"O":   true,

				"P@x":  true,
				"P@y":  true,
				"P@z":  true,
				"P@w":  true,
				"P@r":  true,
				"P@s1": true,
				"P@s2": true,
				"P@s3": nil,
				"P@s4": true,

				"Q@s1": true,
				"Q@s2": true,
				"Q@s3": true,
				"Q@s4": true,
				"Q@s5": true,
				"Q@s6": true,
			},
		},
	}

	for i, c := range cases {
		vm := tagexpr.New(c.tagName)
		// vm.WarmUp(c.structure)
		tagExpr, err := vm.Run(c.structure)
		if err != nil {
			t.Fatal(err)
		}
		for selector, value := range c.tests {
			val := tagExpr.Eval(selector)
			if !reflect.DeepEqual(val, value) {
				t.Fatalf("Eval NO: %d, selector: %q, got: %v, expect: %v", i, selector, val, value)
			}
		}
		tagExpr.Range(func(eh *tagexpr.ExprHandler) error {
			es := eh.ExprSelector()
			t.Logf("Range selector: %s, field: %q exprName: %q", es, es.Field(), es.Name())
			value := c.tests[es.String()]
			val := eh.Eval()
			if !reflect.DeepEqual(val, value) {
				t.Fatalf("Range NO: %d, selector: %q, got: %v, expect: %v", i, es, val, value)
			}
			return nil
		})
	}
}

func TestStruct(t *testing.T) {
	type A struct {
		B struct {
			C struct {
				D struct {
					X string `vd:"$"`
				}
			} `vd:"@:$['D']['X']"`
			C2 string `vd:"@:(C)$['D']['X']"`
			C3 string `vd:"@:(C.D.X)$"`
		}
	}
	a := new(A)
	a.B.C.D.X = "xxx"
	vm := tagexpr.New("vd")
	expr := vm.MustRun(a)
	assertEqual(t, "xxx", expr.EvalString("B.C2"))
	assertEqual(t, "xxx", expr.EvalString("B.C3"))
	assertEqual(t, "xxx", expr.EvalString("B.C"))
	assertEqual(t, "xxx", expr.EvalString("B.C.D.X"))
	expr.Range(func(eh *tagexpr.ExprHandler) error {
		es := eh.ExprSelector()
		t.Logf("Range selector: %s, field: %q exprName: %q", es, es.Field(), es.Name())
		if eh.Eval().(string) != "xxx" {
			t.FailNow()
		}
		return nil
	})
}

func TestStruct2(t *testing.T) {
	type IframeBlock struct {
		XBlock struct {
			BlockType string `vd:"$"`
		}
		Props struct {
			Data struct {
				DataType string `vd:"$"`
			}
		}
	}
	b := new(IframeBlock)
	b.XBlock.BlockType = "BlockType"
	b.Props.Data.DataType = "DataType"
	vm := tagexpr.New("vd")
	expr := vm.MustRun(b)
	if expr.EvalString("XBlock.BlockType") != "BlockType" {
		t.Fatal(expr.EvalString("XBlock.BlockType"))
	}
	if expr.EvalString("Props.Data.DataType") != "DataType" {
		t.Fatal(expr.EvalString("Props.Data.DataType"))
	}
}

func TestStruct3(t *testing.T) {
	type Data struct {
		DataType string `vd:"$"`
	}
	type Prop struct {
		PropType string       `vd:"$"`
		DD       []*Data      `vd:"$"`
		DD2      []*Data      `vd:"$"`
		DataMap  map[int]Data `vd:"$"`
		DataMap2 map[int]Data `vd:"$"`
	}
	type IframeBlock struct {
		XBlock struct {
			BlockType string `vd:"$"`
		}
		Props    []Prop        `vd:"$"`
		Props1   [2]Prop       `vd:"$"`
		Props2   []Prop        `vd:"$"`
		PropMap  map[int]*Prop `vd:"$"`
		PropMap2 map[int]*Prop `vd:"$"`
	}

	b := new(IframeBlock)
	b.XBlock.BlockType = "BlockType"
	p1 := Prop{
		PropType: "p1",
		DD: []*Data{
			{"p1s1"},
			{"p1s2"},
			nil,
		},
		DataMap: map[int]Data{
			1: {"p1m1"},
			2: {"p1m2"},
			0: {},
		},
	}
	b.Props = []Prop{p1}
	p2 := &Prop{
		PropType: "p2",
		DD: []*Data{
			{"p2s1"},
			{"p2s2"},
			nil,
		},
		DataMap: map[int]Data{
			1: {"p2m1"},
			2: {"p2m2"},
			0: {},
		},
	}
	b.Props1 = [2]Prop{p1, {}}
	b.PropMap = map[int]*Prop{
		9: p2,
	}

	vm := tagexpr.New("vd")
	expr := vm.MustRun(b)
	if expr.EvalString("XBlock.BlockType") != "BlockType" {
		t.Fatal(expr.EvalString("XBlock.BlockType"))
	}
	err := expr.Range(func(eh *tagexpr.ExprHandler) error {
		es := eh.ExprSelector()
		t.Logf("Range selector: %s, field: %q exprName: %q, eval: %v", eh.Path(), es.Field(), es.Name(), eh.Eval())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNilField(t *testing.T) {
	type P struct {
		X **struct {
			A *[]uint16 `tagexpr:"$"`
		} `tagexpr:"$"`
		Y **struct{} `tagexpr:"$"`
	}
	vm := tagexpr.New("tagexpr")
	te := vm.MustRun(&P{})
	te.Range(func(eh *tagexpr.ExprHandler) error {
		r := eh.Eval()
		if r != nil {
			t.Fatal(eh.Path())
		}
		return nil
	})

	type G struct {
		// Nil1 *int `tagexpr:"nil!=$"`
		Nil2 *int `tagexpr:"$!=nil"`
	}
	g := &G{
		// Nil1: new(int),
		Nil2: new(int),
	}
	vm.MustRun(g).Range(func(eh *tagexpr.ExprHandler) error {
		r, ok := eh.Eval().(bool)
		if !ok || !r {
			t.Fatal(eh.Path())
		}
		return nil
	})
}

func TestDeepNested(t *testing.T) {
	type testInner struct {
		Address string `tagexpr:"name:$"`
	}
	type struct1 struct {
		I *testInner
		A []*testInner
		X interface{}
	}
	type struct2 struct {
		S *struct1
	}
	type Data struct {
		S1 *struct2
		S2 *struct2
	}
	data := &Data{
		S1: &struct2{
			S: &struct1{
				I: &testInner{Address: "I:address"},
				A: []*testInner{{Address: "A:address"}},
				X: []*testInner{{Address: "X:address"}},
			},
		},
		S2: &struct2{
			S: &struct1{
				A: []*testInner{nil},
			},
		},
	}
	expectKey := [...]interface{}{"S1.S.I.Address@name", "S2.S.I.Address@name", "S1.S.A[0].Address@name", "S2.S.A[0].Address@name", "S1.S.X[0].Address@name"}
	expectValue := [...]interface{}{"I:address", nil, "A:address", nil, "X:address"}
	var i int
	vm := tagexpr.New("tagexpr")
	vm.MustRun(data).Range(func(eh *tagexpr.ExprHandler) error {
		assertEqual(t, expectKey[i], eh.Path())
		assertEqual(t, expectValue[i], eh.Eval())
		i++
		t.Log(eh.Path(), eh.ExprSelector(), eh.Eval())
		return nil
	})
	assertEqual(t, 5, i)
}

func TestIssue3(t *testing.T) {
	type C struct {
		Id    string
		Index int32 `vd:"$"`
		P     *int  `vd:"$!=nil"`
	}
	type A struct {
		F1 *C
		F2 *C
	}
	a := &A{
		F1: &C{
			Id:    "test",
			Index: 1,
			P:     new(int),
		},
	}
	vm := tagexpr.New("vd")
	err := vm.MustRun(a).Range(func(eh *tagexpr.ExprHandler) error {
		switch eh.Path() {
		case "F1.Index":
			assertEqual(t, float64(1), eh.Eval(), eh.Path())
		case "F2.Index":
			assertEqual(t, nil, eh.Eval(), eh.Path())
		case "F1.P":
			assertEqual(t, true, eh.Eval(), eh.Path())
		case "F2.P":
			assertEqual(t, false, eh.Eval(), eh.Path())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIssue4(t *testing.T) {
	type T struct {
		A *string `te:"len($)+mblen($)"`
		B *string `te:"len($)+mblen($)"`
		C *string `te:"len($)+mblen($)"`
	}
	c := "c"
	v := &T{
		B: new(string),
		C: &c,
	}
	vm := tagexpr.New("te")
	err := vm.MustRun(v).Range(func(eh *tagexpr.ExprHandler) error {
		t.Logf("eval:%v, path:%s", eh.EvalFloat(), eh.Path())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIssue5(t *testing.T) {
	type A struct {
		F1 int `vd:"true && $ <= 24*60*60"`        // 1500 ok
		F2 int `vd:"$%60 == 0 && $ <= (24*60*60)"` // 1500 ok
		F3 int `vd:"$ <= 24*60*60"`                // 1500 ok
	}
	a := &A{
		F1: 1500,
		F2: 1500,
		F3: 1500,
	}
	vm := tagexpr.New("vd")
	err := vm.MustRun(a).Range(func(eh *tagexpr.ExprHandler) error {
		switch eh.Path() {
		case "F1":
			assertEqual(t, true, eh.Eval(), eh.Path())
		case "F2":
			assertEqual(t, true, eh.Eval(), eh.Path())
		case "F3":
			assertEqual(t, true, eh.Eval(), eh.Path())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
