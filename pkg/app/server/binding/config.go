package binding

import (
	standardJson "encoding/json"

	hjson "github.com/cloudwego/hertz/pkg/common/json"
)

func ResetJSONUnmarshaler(fn func(data []byte, v interface{}) error) {
	hjson.Unmarshal = fn
}

func ResetStdJSONUnmarshaler() {
	ResetJSONUnmarshaler(standardJson.Unmarshal)
}
