package binding_v2

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/common/utils"

	"github.com/cloudwego/hertz/pkg/protocol"
)

type mapTypeFieldTextDecoder struct {
	index       int
	parentIndex []int
	fieldName   string
	tagInfos    []TagInfo // query,param,header,respHeader ...
	fieldType   reflect.Type
}

func (d *mapTypeFieldTextDecoder) Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error {
	var text string
	var defaultValue string
	// 最大努力交付，对齐 hertz 现有设计
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Key == jsonTag {
			continue
		}
		if tagInfo.Key == headerTag {
			tmp := []byte(tagInfo.Value)
			utils.NormalizeHeaderKey(tmp, req.Header.IsDisableNormalizing())
			tagInfo.Value = string(tmp)
		}
		ret := tagInfo.Getter(req, params, tagInfo.Value)
		defaultValue = tagInfo.Default
		if len(ret) != 0 {
			// 非数组/切片类型，只取第一个值作为只
			text = ret[0]
			break
		}
	}
	if len(text) == 0 && len(defaultValue) != 0 {
		text = defaultValue
	}
	if text == "" {
		return nil
	}

	// todo 多重指针
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
		var vv reflect.Value
		vv, err := stringToValue(t, text)
		if err != nil {
			return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.fieldType.Name(), err)
		}
		field.Set(ReferenceValue(vv, ptrDepth))
		return nil
	}

	err := json.Unmarshal(bytesconv.S2b(text), field.Addr().Interface())
	if err != nil {
		return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.fieldType.Name(), err)
	}

	return nil
}

func getMapTypeTextDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int) ([]decoder, error) {
	for idx, tagInfo := range tagInfos {
		switch tagInfo.Key {
		case pathTag:
			tagInfos[idx].Getter = PathParam
		case formTag:
			tagInfos[idx].Getter = Form
		case queryTag:
			tagInfos[idx].Getter = Query
		case cookieTag:
			tagInfos[idx].Getter = Cookie
		case headerTag:
			tagInfos[idx].Getter = Header
		case jsonTag:
			// do nothing
		case rawBodyTag:
			tagInfo.Getter = RawBody
		default:
		}
	}

	fieldType := field.Type
	for field.Type.Kind() == reflect.Ptr {
		fieldType = field.Type.Elem()
	}
	fieldDecoder := &mapTypeFieldTextDecoder{
		index:       index,
		parentIndex: parentIdx,
		fieldName:   field.Name,
		tagInfos:    tagInfos,
		fieldType:   fieldType,
	}

	return []decoder{fieldDecoder}, nil
}
