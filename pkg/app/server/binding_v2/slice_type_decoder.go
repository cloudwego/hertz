package binding_v2

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/cloudwego/hertz/pkg/common/utils"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/app/server/binding_v2/text_decoder"
	"github.com/cloudwego/hertz/pkg/protocol"
)

type sliceTypeFieldTextDecoder struct {
	index       int
	parentIndex []int
	fieldName   string
	isArray     bool
	tagInfos    []TagInfo // query,param,header,respHeader ...
	fieldType   reflect.Type
}

func (d *sliceTypeFieldTextDecoder) Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error {
	var texts []string
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Key == jsonTag {
			continue
		}
		if tagInfo.Key == headerTag {
			tmp := []byte(tagInfo.Value)
			utils.NormalizeHeaderKey(tmp, req.Header.IsDisableNormalizing())
			tagInfo.Value = string(tmp)
		}
		texts = tagInfo.Getter(req, params, tagInfo.Value)
		// todo: 数组默认值
		// defaultValue = tagInfo.Default
		if len(texts) != 0 {
			break
		}
	}
	if len(texts) == 0 {
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

	if d.isArray {
		if len(texts) != field.Len() {
			return fmt.Errorf("%q is not valid value for %s", texts, field.Type().String())
		}
	} else {
		// slice need creating enough capacity
		field = reflect.MakeSlice(field.Type(), len(texts), len(texts))
	}

	// handle multiple pointer
	var ptrDepth int
	t := d.fieldType.Elem()
	elemKind := t.Kind()
	for elemKind == reflect.Ptr {
		t = t.Elem()
		elemKind = t.Kind()
		ptrDepth++
	}

	for idx, text := range texts {
		var vv reflect.Value
		vv, err := stringToValue(t, text)
		if err != nil {
			return err
		}
		field.Index(idx).Set(ReferenceValue(vv, ptrDepth))
	}
	reqValue.Field(d.index).Set(field)

	return nil
}

// 数组/切片类型的decoder，
// 对于map和struct类型的数组元素直接使用unmarshal，不做嵌套处理
func getSliceFieldDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int) ([]decoder, error) {
	if !(field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) {
		return nil, fmt.Errorf("unexpected type %s, expected slice or array", field.Type.String())
	}
	isArray := false
	if field.Type.Kind() == reflect.Array {
		isArray = true
	}
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

	fieldDecoder := &sliceTypeFieldTextDecoder{
		index:       index,
		parentIndex: parentIdx,
		fieldName:   field.Name,
		tagInfos:    tagInfos,
		fieldType:   fieldType,
		isArray:     isArray,
	}

	return []decoder{fieldDecoder}, nil
}

func stringToValue(elemType reflect.Type, text string) (v reflect.Value, err error) {
	v = reflect.New(elemType).Elem()
	// todo：自定义类型解析

	switch elemType.Kind() {
	case reflect.Struct:
		err = json.Unmarshal(bytesconv.S2b(text), v.Addr().Interface())
	case reflect.Map:
		err = json.Unmarshal(bytesconv.S2b(text), v.Addr().Interface())
	case reflect.Array, reflect.Slice:
		// do nothing
	default:
		decoder, err := text_decoder.SelectTextDecoder(elemType)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("unsupport type %s for slice/array", elemType.String())
		}
		err = decoder.UnmarshalString(text, v)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("unable to decode '%s' as %s: %w", text, elemType.String(), err)
		}
	}

	return v, nil
}
