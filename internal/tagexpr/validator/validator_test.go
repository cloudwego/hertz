package validator_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	vd "github.com/bytedance/go-tagexpr/v2/validator"
)

func TestNil(t *testing.T) {
	type F struct {
		f struct {
			g int `vd:"$%3==1"`
		}
	}
	assert.EqualError(t, vd.Validate((*F)(nil)), "unsupport data: nil")
}

func TestAll(t *testing.T) {
	type T struct {
		a string `vd:"email($)"`
		f struct {
			g int `vd:"$%3==1"`
		}
	}
	assert.EqualError(t, vd.Validate(new(T), true), "email format is incorrect\tinvalid parameter: f.g")
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
	var invalid = "invalid email"
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
	assert.EqualError(t, vd.Validate(req, false), "email format is incorrect")
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
	assert.NoError(t, v.Validate(A))
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
	assert.NoError(t, v.Validate(a))
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
	assert.NoError(t, v.Validate(a))

	a = &A{F1: new(C)}
	assert.EqualError(t, v.Validate(a), "index is nil")

	a = &A{F2: map[string]*C{"x": &C{Index: new(int32)}}}
	assert.EqualError(t, v.Validate(a), "invalid parameter: F2{v for k=x}.Index2")

	a = &A{F3: []*C{{Index: new(int32)}}}
	assert.EqualError(t, v.Validate(a), "invalid parameter: F3[0].Index2")

	type B struct {
		F1 *C `vd:"$!=nil"`
		F2 *C
	}
	b := &B{}
	assert.EqualError(t, v.Validate(b), "invalid parameter: F1")

	type D struct {
		F1 *C
		F2 *C
	}

	type E struct {
		D []*D
	}
	b.F1 = new(C)
	e := &E{D: []*D{nil}}
	assert.NoError(t, v.Validate(e))
}

func TestIssue5(t *testing.T) {
	type SubSheet struct {
	}
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
	assert.NoError(t, err)
	assert.Equal(t, 1, len(data.Requests))
	assert.Nil(t, data.Requests[0].CopySheet)
	v := vd.New("vd")
	assert.NoError(t, v.Validate(&data))
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
	assert.EqualError(t, err, "invalid parameter: A")
	data.A = "b"
	err = v.Validate(data)
	assert.EqualError(t, err, "invalid parameter: B")
	data.B = 2
	err = v.Validate(data)
	assert.NoError(t, err)

	type T2 struct {
		C string `vd:"in($)"`
	}
	data2 := &T2{}
	err = v.Validate(data2)
	assert.EqualError(t, err, "invalid parameter: C")

	type T3 struct {
		C string `vd:"in($,1)"`
	}
	data3 := &T3{}
	err = v.Validate(data3)
	assert.EqualError(t, err, "invalid parameter: C")
}

type (
	Issue23A struct {
		b *Issue23B
		v int64 `vd:"$==0"`
	}
	Issue23B struct {
		a *Issue23A
		v int64 `vd:"$==0"`
	}
)

func TestIssue23(t *testing.T) {
	var data = &Issue23B{a: &Issue23A{b: new(Issue23B)}}
	err := vd.Validate(data, true)
	assert.NoError(t, err)
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
	var data = &SubmitDoctorImportRequest{SubmitDoctorImport: []*SubmitDoctorImportItem{{}}}
	err := vd.Validate(data, true)
	assert.EqualError(t, err, "invalid parameter: SubmitDoctorImport[0].Idcard\tinvalid parameter: SubmitDoctorImport[0].PracCertNo\temail format is incorrect\tthe phone number supplied is not a number")
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
	assert.EqualError(t, err, "invalid parameter: A{v for k=x}.f.g\tinvalid parameter: B[0]{v for k=y}.f.g\tinvalid parameter: C{v for k=z}[0]{v for k=zz}.f.g")
}

func TestIssue30(t *testing.T) {
	type TStruct struct {
		TOk string `vd:"gt($,'0') && gt($, '1')" json:"t_ok"`
		// TFail string `vd:"gt($,'0')" json:"t_fail"`
	}
	vd.RegFunc("gt", func(args ...interface{}) error {
		return errors.New("force error")
	})
	assert.EqualError(t, vd.Validate(&TStruct{TOk: "1"}), "invalid parameter: TOk")
	// assert.NoError(t, vd.Validate(&TStruct{TOk: "1", TFail: "1"}))
}

func TestIssue31(t *testing.T) {
	type TStruct struct {
		A []int32 `vd:"$ == nil || ($ != nil && range($, in(#v, 1, 2, 3))"`
	}
	assert.EqualError(t, vd.Validate(&TStruct{A: []int32{1}}), "syntax error: \"($ != nil && range($, in(#v, 1, 2, 3))\"")
	assert.EqualError(t, vd.Validate(&TStruct{A: []int32{1}}), "syntax error: \"($ != nil && range($, in(#v, 1, 2, 3))\"")
	assert.EqualError(t, vd.Validate(&TStruct{A: []int32{1}}), "syntax error: \"($ != nil && range($, in(#v, 1, 2, 3))\"")
}

func TestRegexp(t *testing.T) {
	type TStruct struct {
		A string `vd:"regexp('(\\d+\\.){3}\\d+')"`
	}
	assert.NoError(t, vd.Validate(&TStruct{A: "0.0.0.0"}))
	assert.EqualError(t, vd.Validate(&TStruct{A: "0...0"}), "invalid parameter: A")
	assert.EqualError(t, vd.Validate(&TStruct{A: "abc1"}), "invalid parameter: A")
	assert.EqualError(t, vd.Validate(&TStruct{A: "0?0?0?0"}), "invalid parameter: A")
}

func TestRangeIn(t *testing.T) {
	type S struct {
		F []string `vd:"range($, in(#v, '', 'ttp', 'euttp'))"`
	}
	err := vd.Validate(S{
		F: []string{"ttp", "", "euttp"},
	})
	assert.NoError(t, err)
	err = vd.Validate(S{
		F: []string{"ttp", "?", "euttp"},
	})
	assert.EqualError(t, err, "invalid parameter: F")
}
