package binding_v2

import (
	"encoding/json"

	hjson "github.com/cloudwego/hertz/pkg/common/json"
)

// JSONUnmarshaler is the interface implemented by types
// that can unmarshal a JSON description of themselves.
type JSONUnmarshaler func(data []byte, v interface{}) error

var jsonUnmarshalFunc JSONUnmarshaler

func init() {
	ResetJSONUnmarshaler(hjson.Unmarshal)
}

func ResetJSONUnmarshaler(fn JSONUnmarshaler) {
	jsonUnmarshalFunc = fn
}

func ResetStdJSONUnmarshaler() {
	ResetJSONUnmarshaler(json.Unmarshal)
}
