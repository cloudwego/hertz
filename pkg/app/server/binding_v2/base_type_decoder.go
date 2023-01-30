package binding_v2

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/hertz/pkg/app/server/binding_v2/text_decoder"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
)

type fieldInfo struct {
	index       int
	parentIndex []int
	fieldName   string
	tagInfos    []TagInfo // query,param,header,respHeader ...
	fieldType   reflect.Type
}

type baseTypeFieldTextDecoder struct {
	fieldInfo
	decoder text_decoder.TextDecoder
}

func (d *baseTypeFieldTextDecoder) Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error {
	var err error
	var text string
	var defaultValue string
	// 最大努力交付，对齐 hertz 现有设计
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Key == jsonTag {
			continue
		}
		if tagInfo.Key == headerTag {
			tagInfo.Value = utils.GetNormalizeHeaderKey(tagInfo.Value, req.Header.IsDisableNormalizing())
		}
		ret := tagInfo.Getter(req, params, tagInfo.Value)
		defaultValue = tagInfo.Default
		if len(ret) != 0 {
			// 非数组/切片类型，只取第一个值作为只
			text = ret[0]
			err = nil
			break
		}
		if tagInfo.Required {
			err = fmt.Errorf("'%s' field is a 'required' parameter, but the request does not have this parameter", d.fieldName)
		}
	}
	if err != nil {
		return err
	}
	if len(text) == 0 && len(defaultValue) != 0 {
		text = defaultValue
	}
	if text == "" {
		return nil
	}

	// 得到该field的非nil值
	reqValue = GetFieldValue(reqValue, d.parentIndex)
	// 根据最终的 Struct，获取对应 field 的 reflect.Value
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
			return err
		}
		field.Set(ReferenceValue(vv, ptrDepth))
		return nil
	}

	// Non-pointer elems
	err = d.decoder.UnmarshalString(text, field)
	if err != nil {
		return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.fieldType.Name(), err)
	}

	return nil
}

func getBaseTypeTextDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int) ([]decoder, error) {
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

	textDecoder, err := text_decoder.SelectTextDecoder(fieldType)
	if err != nil {
		return nil, err
	}

	fieldDecoder := &baseTypeFieldTextDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
		},
		decoder: textDecoder,
	}

	return []decoder{fieldDecoder}, nil
}
