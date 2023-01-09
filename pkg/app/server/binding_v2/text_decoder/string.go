package text_decoder

import "reflect"

type stringDecoder struct{}

func (d *stringDecoder) UnmarshalString(s string, fieldValue reflect.Value) error {
	// todo: 优化一下
	fieldValue.SetString(s)
	return nil
}
