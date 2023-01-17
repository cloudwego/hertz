package binding_v2

import (
	"reflect"

	"github.com/cloudwego/hertz/pkg/protocol"
)

type customizedFieldTextDecoder struct {
	fieldInfo
}

func (d *customizedFieldTextDecoder) Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error {
	var err error
	v := reflect.New(d.fieldType)
	decoder := v.Interface().(CustomizedFieldDecoder)

	if err = decoder.CustomizedFieldDecode(req, params); err != nil {
		return err
	}

	reqValue = GetFieldValue(reqValue, d.parentIndex)
	field := reqValue.Field(d.index)
	if field.Kind() == reflect.Ptr {
		// 如果是指针则新建一个reflect.Value，然后赋值给指针
		t := field.Type()
		var ptrDepth int
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
			ptrDepth++
		}
		field.Set(ReferenceValue(v.Elem(), ptrDepth))
		return nil
	}

	field.Set(v)
	return nil
}
