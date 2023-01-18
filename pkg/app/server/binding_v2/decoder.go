package binding_v2

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/hertz/pkg/protocol"
)

type decoder interface {
	Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error
}

type CustomizedFieldDecoder interface {
	CustomizedFieldDecode(req *protocol.Request, params PathParams) error
}

type Decoder func(req *protocol.Request, params PathParams, rv reflect.Value) error

var fieldDecoderType = reflect.TypeOf((*CustomizedFieldDecoder)(nil)).Elem()

func getReqDecoder(rt reflect.Type) (Decoder, error) {
	var decoders []decoder

	el := rt.Elem()
	if el.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupport \"%s\" type binding", el.String())
	}

	for i := 0; i < el.NumField(); i++ {
		if !el.Field(i).IsExported() {
			// ignore unexported field
			continue
		}

		dec, err := getFieldDecoder(el.Field(i), i, []int{})
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

func getFieldDecoder(field reflect.StructField, index int, parentIdx []int) ([]decoder, error) {
	// 去掉每一个filed的指针，使其指向最终内容
	for field.Type.Kind() == reflect.Ptr {
		field.Type = field.Type.Elem()
	}
	if reflect.PtrTo(field.Type).Implements(fieldDecoderType) {
		return []decoder{&customizedFieldTextDecoder{
			fieldInfo: fieldInfo{
				index:       index,
				parentIndex: parentIdx,
				fieldName:   field.Name,
				fieldType:   field.Type,
			},
		}}, nil
	}

	fieldTagInfos := lookupFieldTags(field)
	// todo: 没有 tag 也不直接返回
	if len(fieldTagInfos) == 0 {
		fieldTagInfos = getDefaultFieldTags(field)
	}

	if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
		return getSliceFieldDecoder(field, index, fieldTagInfos, parentIdx)
	}

	// todo: reflect Map
	if field.Type.Kind() == reflect.Map {
		return getMapTypeTextDecoder(field, index, fieldTagInfos, parentIdx)
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
			// todo: 优化一下？ idxes := append(parentIdx, index)
			var idxes []int
			if len(parentIdx) > 0 {
				idxes = append(idxes, parentIdx...)
			}
			idxes = append(idxes, index)
			dec, err := getFieldDecoder(el.Field(i), i, idxes)
			if err != nil {
				return nil, err
			}

			if dec != nil {
				decoders = append(decoders, dec...)
			}
		}

		return decoders, nil
	}

	return getBaseTypeTextDecoder(field, index, fieldTagInfos, parentIdx)
}
