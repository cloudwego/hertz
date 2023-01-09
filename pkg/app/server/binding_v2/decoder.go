package binding_v2

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/hertz/pkg/protocol"
)

type decoder interface {
	Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error
}

type FieldCustomizedDecoder interface {
	CustomizedFieldDecode(req *protocol.Request, params PathParams) error
}

type Decoder func(req *protocol.Request, params PathParams, rv reflect.Value) error

var fieldDecoderType = reflect.TypeOf((*FieldCustomizedDecoder)(nil)).Elem()

func getReqDecoder(rt reflect.Type) (Decoder, error) {
	var decoders []decoder

	el := rt.Elem()
	if el.Kind() != reflect.Struct {
		// todo: 增加对map的支持
		return nil, fmt.Errorf("unsupport non-struct type binding")
	}

	for i := 0; i < el.NumField(); i++ {
		if !el.Field(i).IsExported() {
			// ignore unexported field
			continue
		}

		dec, err := getFieldDecoder(el.Field(i), i)
		if err != nil {
			return nil, err
		}

		if dec != nil {
			decoders = append(decoders, dec...)
		}
	}

	return func(req *protocol.Request, params PathParams, rv reflect.Value) error {
		for _, decoder := range decoders {
			err := decoder.Decode(req, params, rv)
			if err != nil {
				return err
			}
		}

		return nil
	}, nil
}

func getFieldDecoder(field reflect.StructField, index int) ([]decoder, error) {
	if reflect.PtrTo(field.Type).Implements(fieldDecoderType) {
		return []decoder{&customizedFieldTextDecoder{index: index, fieldName: field.Name, fieldType: field.Type}}, nil
	}

	fieldTagInfos := lookupFieldTags(field)
	if len(fieldTagInfos) == 0 {
		// todo: 如果没定义尝试给其赋值所有 tag
		return nil, nil
	}

	// todo: 用户自定义text信息解析
	//if reflect.PtrTo(field.Type).Implements(textUnmarshalerType) {
	//	return compileTextBasedDecoder(field, index, tagScope, tagContent)
	//}

	// todo: reflect Map
	if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
		return getSliceFieldDecoder(field, index, fieldTagInfos)
	}

	// Nested binding support
	if field.Type.Kind() == reflect.Ptr {
		field.Type = field.Type.Elem()
	}
	// 递归每一个 struct
	if field.Type.Kind() == reflect.Struct {
		var decoders []decoder
		el := field.Type

		for i := 0; i < el.NumField(); i++ {
			if !el.Field(i).IsExported() {
				// ignore unexported field
				continue
			}
			dec, err := getFieldDecoder(el.Field(i), i)
			if err != nil {
				return nil, err
			}

			if dec != nil {
				decoders = append(decoders, dec...)
			}
		}

		return decoders, nil
	}

	return getBaseTypeTextDecoder(field, index, fieldTagInfos)
}
