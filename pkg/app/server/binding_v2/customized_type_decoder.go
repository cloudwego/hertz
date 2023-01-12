package binding_v2

import (
	"reflect"

	"github.com/cloudwego/hertz/pkg/protocol"
)

type customizedFieldTextDecoder struct {
	index       int
	parentIndex []int
	fieldName   string
	fieldType   reflect.Type
}

func (d *customizedFieldTextDecoder) Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error {
	var err error
	v := reflect.New(d.fieldType)
	decoder := v.Interface().(CustomizedFieldDecoder)

	if err = decoder.CustomizedFieldDecode(req, params); err != nil {
		return err
	}

	// 找到该 field 的父 struct 的 reflect.Value
	for _, idx := range d.parentIndex {
		if reqValue.Kind() == reflect.Ptr && reqValue.IsNil() {
			nonNilVal, ptrDepth := GetNonNilReferenceValue(reqValue)
			reqValue.Set(ReferenceValue(nonNilVal, ptrDepth))
		}
		for reqValue.Kind() == reflect.Ptr {
			reqValue = reqValue.Elem()
		}
		reqValue = reqValue.Field(idx)
	}

	// 父 struct 有可能也是一个指针，所以需要再处理一次才能得到最终的父Value(非nil的reflect.Value)
	for reqValue.Kind() == reflect.Ptr {
		if reqValue.IsNil() {
			nonNilVal, ptrDepth := GetNonNilReferenceValue(reqValue)
			reqValue.Set(ReferenceValue(nonNilVal, ptrDepth))
		}
		reqValue = reqValue.Elem()
	}

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
