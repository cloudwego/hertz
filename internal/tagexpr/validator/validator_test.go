// Copyright 2019 Bytedance Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validator_test

import (
	"encoding/json"
	"errors"
	"testing"

	vd "github.com/cloudwego/hertz/internal/tagexpr/validator"
)

func assertEqualError(t *testing.T, err error, s string) {
	t.Helper()
	if err.Error() != s {
		t.Fatal("not equal", err, s)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNil(t *testing.T) {
	type F struct {
		F struct {
			G int `vd:"$%3==1"`
		}
	}
	assertEqualError(t, vd.Validate((*F)(nil)), "unsupported data: nil")
}

func TestAll(t *testing.T) {
	type T struct {
		A string `vd:"email($)"`
		F struct {
			G int `vd:"$%3==1"`
		}
	}
	assertEqualError(t, vd.Validate(new(T), true), "email format is incorrect\tinvalid parameter: F.G")
}

func TestIssue1(t *testing.T) {
	type MailBox struct {
		Address *string `vd:"email($)"`
		Name    *string
	}
	type EmailMsg struct {
		Recipients       []*MailBox
		RecipientsCc     []*MailBox
		RecipientsBcc    []*MailBox
		Subject          *string
		Content          *string
		AttachmentIDList []string
		ReplyTo          *string
		Params           map[string]string
		FromEmailAddress *string
		FromEmailName    *string
	}
	type EmailTaskInfo struct {
		Msg         *EmailMsg
		StartTimeMS *int64
		LogTag      *string
	}
	type BatchCreateEmailTaskRequest struct {
		InfoList []*EmailTaskInfo
	}
	invalid := "invalid email"
	req := &BatchCreateEmailTaskRequest{
		InfoList: []*EmailTaskInfo{
			{
				Msg: &EmailMsg{
					Recipients: []*MailBox{
						{
							Address: &invalid,
						},
					},
				},
			},
		},
	}
	assertEqualError(t, vd.Validate(req, false), "email format is incorrect")
}

func TestIssue2(t *testing.T) {
	type a struct {
		m map[string]interface{}
	}
	A := &a{
		m: map[string]interface{}{
			"1": 1,
			"2": nil,
		},
	}
	v := vd.New("vd")
	assertNoError(t, v.Validate(A))
}

func TestIssue3(t *testing.T) {
	type C struct {
		Id    string
		Index int32 `vd:"$==1"`
	}
	type A struct {
		F1 *C
		F2 *C
	}
	a := &A{
		F1: &C{
			Id:    "test",
			Index: 1,
		},
	}
	v := vd.New("vd")
	assertNoError(t, v.Validate(a))
}

func TestIssue4(t *testing.T) {
	type C struct {
		Index  *int32 `vd:"@:$!=nil;msg:'index is nil'"`
		Index2 *int32 `vd:"$!=nil"`
		Index3 *int32 `vd:"$!=nil"`
	}
	type A struct {
		F1 *C
		F2 map[string]*C
		F3 []*C
	}
	v := vd.New("vd")

	a := &A{}
	assertNoError(t, v.Validate(a))

	a = &A{F1: new(C)}
	assertEqualError(t, v.Validate(a), "index is nil")

	a = &A{F2: map[string]*C{"x": {Index: new(int32)}}}
	assertEqualError(t, v.Validate(a), "invalid parameter: F2{v for k=x}.Index2")

	a = &A{F3: []*C{{Index: new(int32)}}}
	assertEqualError(t, v.Validate(a), "invalid parameter: F3[0].Index2")

	type B struct {
		F1 *C `vd:"$!=nil"`
		F2 *C
	}
	b := &B{}
	assertEqualError(t, v.Validate(b), "invalid parameter: F1")

	type D struct {
		F1 *C
		F2 *C
	}

	type E struct {
		D []*D
	}
	b.F1 = new(C)
	e := &E{D: []*D{nil}}
	assertNoError(t, v.Validate(e))
}

func TestIssue5(t *testing.T) {
	type SubSheet struct{}
	type CopySheet struct {
		Source      *SubSheet `json:"source" vd:"$!=nil"`
		Destination *SubSheet `json:"destination" vd:"$!=nil"`
	}
	type UpdateSheetsRequest struct {
		CopySheet *CopySheet `json:"copySheet"`
	}
	type BatchUpdateSheetRequestArg struct {
		Requests []*UpdateSheetsRequest `json:"requests"`
	}
	b := `{"requests": [{}]}`
	var data BatchUpdateSheetRequestArg
	err := json.Unmarshal([]byte(b), &data)
	assertNoError(t, err)
	if len(data.Requests) != 1 {
		t.Fatal(len(data.Requests))
	}
	if data.Requests[0].CopySheet != nil {
		t.Fatal(data.Requests[0].CopySheet)
	}
	v := vd.New("vd")
	assertNoError(t, v.Validate(&data))
}

func TestIn(t *testing.T) {
	type S string
	type I int16
	type T struct {
		X *int `vd:"$==nil || len($)>0"`
		A S    `vd:"in($,'a','b','c')"`
		B I    `vd:"in($,1,2.0,3)"`
	}
	v := vd.New("vd")
	data := &T{}
	err := v.Validate(data)
	assertEqualError(t, err, "invalid parameter: A")
	data.A = "b"
	err = v.Validate(data)
	assertEqualError(t, err, "invalid parameter: B")
	data.B = 2
	err = v.Validate(data)
	assertNoError(t, err)

	type T2 struct {
		C string `vd:"in($)"`
	}
	data2 := &T2{}
	err = v.Validate(data2)
	assertEqualError(t, err, "invalid parameter: C")

	type T3 struct {
		C string `vd:"in($,1)"`
	}
	data3 := &T3{}
	err = v.Validate(data3)
	assertEqualError(t, err, "invalid parameter: C")
}

type (
	Issue23A struct {
		B *Issue23B
		V int64 `vd:"$==0"`
	}
	Issue23B struct {
		A *Issue23A
		V int64 `vd:"$==0"`
	}
)

func TestIssue23(t *testing.T) {
	data := &Issue23B{A: &Issue23A{B: new(Issue23B)}}
	err := vd.Validate(data, true)
	assertNoError(t, err)
}

func TestIssue24(t *testing.T) {
	type SubmitDoctorImportItem struct {
		Name       string   `form:"name,required" json:"name,required" query:"name,required"`
		Avatar     *string  `form:"avatar,omitempty" json:"avatar,omitempty" query:"avatar,omitempty"`
		Idcard     string   `form:"idcard,required" json:"idcard,required" query:"idcard,required" vd:"len($)==18"`
		IdcardPics []string `form:"idcard_pics,omitempty" json:"idcard_pics,omitempty" query:"idcard_pics,omitempty"`
		Hosp       string   `form:"hosp,required" json:"hosp,required" query:"hosp,required"`
		HospDept   string   `form:"hosp_dept,required" json:"hosp_dept,required" query:"hosp_dept,required"`
		HospProv   *string  `form:"hosp_prov,omitempty" json:"hosp_prov,omitempty" query:"hosp_prov,omitempty"`
		HospCity   *string  `form:"hosp_city,omitempty" json:"hosp_city,omitempty" query:"hosp_city,omitempty"`
		HospCounty *string  `form:"hosp_county,omitempty" json:"hosp_county,omitempty" query:"hosp_county,omitempty"`
		ProTit     string   `form:"pro_tit,required" json:"pro_tit,required" query:"pro_tit,required"`
		ThTit      *string  `form:"th_tit,omitempty" json:"th_tit,omitempty" query:"th_tit,omitempty"`
		ServDepts  *string  `form:"serv_depts,omitempty" json:"serv_depts,omitempty" query:"serv_depts,omitempty"`
		TitCerts   []string `form:"tit_certs,omitempty" json:"tit_certs,omitempty" query:"tit_certs,omitempty"`
		ThTitCerts []string `form:"th_tit_certs,omitempty" json:"th_tit_certs,omitempty" query:"th_tit_certs,omitempty"`
		PracCerts  []string `form:"prac_certs,omitempty" json:"prac_certs,omitempty" query:"prac_certs,omitempty"`
		QualCerts  []string `form:"qual_certs,omitempty" json:"qual_certs,omitempty" query:"qual_certs,omitempty"`
		PracCertNo string   `form:"prac_cert_no,required" json:"prac_cert_no,required" query:"prac_cert_no,required" vd:"len($)==15"`
		Goodat     *string  `form:"goodat,omitempty" json:"goodat,omitempty" query:"goodat,omitempty"`
		Intro      *string  `form:"intro,omitempty" json:"intro,omitempty" query:"intro,omitempty"`
		Linkman    string   `form:"linkman,required" json:"linkman,required" query:"linkman,required" vd:"email($)"`
		Phone      string   `form:"phone,required" json:"phone,required" query:"phone,required" vd:"phone($,'CN')"`
	}

	type SubmitDoctorImportRequest struct {
		SubmitDoctorImport []*SubmitDoctorImportItem `form:"submit_doctor_import,required" json:"submit_doctor_import,required"`
	}
	data := &SubmitDoctorImportRequest{SubmitDoctorImport: []*SubmitDoctorImportItem{{}}}
	err := vd.Validate(data, true)
	assertEqualError(t, err, "invalid parameter: SubmitDoctorImport[0].Idcard\tinvalid parameter: SubmitDoctorImport[0].PracCertNo\temail format is incorrect\tthe phone number supplied is not a number")
}

func TestStructSliceMap(t *testing.T) {
	type F struct {
		f struct {
			g int `vd:"$%3==0"`
		}
	}
	f := &F{}
	f.f.g = 10
	type S struct {
		A map[string]*F
		B []map[string]*F
		C map[string][]map[string]F
		// _ int
	}
	s := S{
		A: map[string]*F{"x": f},
		B: []map[string]*F{{"y": f}},
		C: map[string][]map[string]F{"z": {{"zz": *f}}},
	}
	err := vd.Validate(s, true)
	assertEqualError(t, err, "invalid parameter: A{v for k=x}.f.g\tinvalid parameter: B[0]{v for k=y}.f.g\tinvalid parameter: C{v for k=z}[0]{v for k=zz}.f.g")
}

func TestIssue30(t *testing.T) {
	type TStruct struct {
		TOk string `vd:"gt($,'0') && gt($, '1')" json:"t_ok"`
		// TFail string `vd:"gt($,'0')" json:"t_fail"`
	}
	vd.RegFunc("gt", func(args ...interface{}) error {
		return errors.New("force error")
	})
	assertEqualError(t, vd.Validate(&TStruct{TOk: "1"}), "invalid parameter: TOk")
	// assertNoError(t, vd.Validate(&TStruct{TOk: "1", TFail: "1"}))
}

func TestIssue31(t *testing.T) {
	type TStruct struct {
		A []int32 `vd:"$ == nil || ($ != nil && range($, in(#v, 1, 2, 3))"`
	}
	assertEqualError(t, vd.Validate(&TStruct{A: []int32{1}}), "syntax error: \"($ != nil && range($, in(#v, 1, 2, 3))\"")
	assertEqualError(t, vd.Validate(&TStruct{A: []int32{1}}), "syntax error: \"($ != nil && range($, in(#v, 1, 2, 3))\"")
	assertEqualError(t, vd.Validate(&TStruct{A: []int32{1}}), "syntax error: \"($ != nil && range($, in(#v, 1, 2, 3))\"")
}

func TestRegexp(t *testing.T) {
	type TStruct struct {
		A string `vd:"regexp('(\\d+\\.){3}\\d+')"`
	}
	assertNoError(t, vd.Validate(&TStruct{A: "0.0.0.0"}))
	assertEqualError(t, vd.Validate(&TStruct{A: "0...0"}), "invalid parameter: A")
	assertEqualError(t, vd.Validate(&TStruct{A: "abc1"}), "invalid parameter: A")
	assertEqualError(t, vd.Validate(&TStruct{A: "0?0?0?0"}), "invalid parameter: A")
}

func TestRangeIn(t *testing.T) {
	type S struct {
		F []string `vd:"range($, in(#v, '', 'ttp', 'euttp'))"`
	}
	err := vd.Validate(S{
		F: []string{"ttp", "", "euttp"},
	})
	assertNoError(t, err)
	err = vd.Validate(S{
		F: []string{"ttp", "?", "euttp"},
	})
	assertEqualError(t, err, "invalid parameter: F")
}
