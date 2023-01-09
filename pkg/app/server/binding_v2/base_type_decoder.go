package binding_v2

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/hertz/pkg/app/server/binding_v2/text_decoder"
	"github.com/cloudwego/hertz/pkg/protocol"
)

type baseTypeFieldTextDecoder struct {
	index     int
	fieldName string
	tagInfos  []TagInfo // query,param,header,respHeader ...
	fieldType reflect.Type
	decoder   text_decoder.TextDecoder
}

func (d *baseTypeFieldTextDecoder) Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error {
	var text string
	var defaultValue string
	// 最大努力交付，对齐 hertz 现有设计
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Key == jsonTag {
			continue
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

	var err error

	// Pointer support for struct elems
	field := reqValue.Field(d.index)
	if field.Kind() == reflect.Ptr {
		elem := reflect.New(d.fieldType)
		err = d.decoder.UnmarshalString(text, elem.Elem())
		if err != nil {
			return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.fieldType.Name(), err)
		}

		field.Set(elem)

		return nil
	}

	// Non-pointer elems
	err = d.decoder.UnmarshalString(text, field)
	if err != nil {
		return fmt.Errorf("unable to decode '%s' as %s: %w", text, d.fieldType.Name(), err)
	}

	return nil
}

func getBaseTypeTextDecoder(field reflect.StructField, index int, tagInfos []TagInfo) ([]decoder, error) {
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
	if field.Type.Kind() == reflect.Ptr {
		fieldType = field.Type.Elem()
	}

	textDecoder, err := text_decoder.SelectTextDecoder(fieldType)
	if err != nil {
		return nil, err
	}

	fieldDecoder := &baseTypeFieldTextDecoder{
		index:     index,
		fieldName: field.Name,
		tagInfos:  tagInfos,
		decoder:   textDecoder,
		fieldType: fieldType,
	}

	return []decoder{fieldDecoder}, nil
}
