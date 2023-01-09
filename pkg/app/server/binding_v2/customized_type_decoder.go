package binding_v2

import (
	"reflect"

	"github.com/cloudwego/hertz/pkg/protocol"
)

type customizedFieldTextDecoder struct {
	index     int
	fieldName string
	fieldType reflect.Type
}

func (d *customizedFieldTextDecoder) Decode(req *protocol.Request, params PathParams, reqValue reflect.Value) error {
	v := reflect.New(d.fieldType)
	decoder := v.Interface().(FieldCustomizedDecoder)

	if err := decoder.CustomizedFieldDecode(req, params); err != nil {
		return err
	}

	reqValue.Field(d.index).Set(v.Elem())
	return nil
}
