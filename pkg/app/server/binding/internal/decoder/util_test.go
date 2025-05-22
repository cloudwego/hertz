/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package decoder

import (
	"encoding"
	"errors"
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route/param"
)

type testTextUnmarshaler struct {
	Value string
}

func (t *testTextUnmarshaler) UnmarshalText(text []byte) error {
	t.Value = string(text)
	return nil
}

var _ encoding.TextUnmarshaler = (*testTextUnmarshaler)(nil)

func TestStringToValue(t *testing.T) {
	tests := []struct {
		name        string
		elemType    reflect.Type
		text        string
		config      *DecodeConfig
		expectValue interface{}
		expectError bool
	}{
		{
			name:        "string type",
			elemType:    reflect.TypeOf(""),
			text:        "test string",
			expectValue: "test string",
		},
		{
			name:        "int type",
			elemType:    reflect.TypeOf(0),
			text:        "42",
			expectValue: 42,
		},
		{
			name:        "bool type",
			elemType:    reflect.TypeOf(false),
			text:        "true",
			expectValue: true,
		},
		{
			name:        "float type",
			elemType:    reflect.TypeOf(0.0),
			text:        "3.14",
			expectValue: 3.14,
		},
		{
			name:        "text unmarshaler",
			elemType:    reflect.TypeOf(testTextUnmarshaler{}),
			text:        "custom text",
			expectValue: testTextUnmarshaler{Value: "custom text"},
		},
		{
			name:        "invalid int",
			elemType:    reflect.TypeOf(0),
			text:        "not an int",
			expectError: true,
		},
		{
			name:        "struct type",
			elemType:    reflect.TypeOf(struct{ Name string }{}),
			text:        `{"Name":"test"}`,
			expectValue: struct{ Name string }{Name: "test"},
		},
		{
			name:        "struct type err",
			elemType:    reflect.TypeOf(struct{ Name string }{}),
			text:        `{"Name":1}`,
			expectError: true,
		},
		{
			name:        "list type",
			elemType:    reflect.TypeOf([]int{}),
			expectValue: *new([]int),
		},
		{
			name:        "map type",
			elemType:    reflect.TypeOf(map[string]interface{}{}),
			text:        `{"key":"value"}`,
			expectValue: map[string]interface{}{"key": "value"},
		},
		{
			name:        "unsupported type",
			elemType:    reflect.TypeOf(complex64(0)),
			expectError: true,
		},
		{
			name:     "custom type unmarshal func",
			elemType: reflect.TypeOf(testTextUnmarshaler{}),
			text:     "custom func",
			config: &DecodeConfig{
				TypeUnmarshalFuncs: map[reflect.Type]CustomizeDecodeFunc{
					reflect.TypeOf(testTextUnmarshaler{}): func(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
						return reflect.ValueOf(testTextUnmarshaler{Value: "from custom func"}), nil
					},
				},
			},
			expectValue: testTextUnmarshaler{Value: "from custom func"},
		},
		{
			name:     "custom type unmarshal func err",
			elemType: reflect.TypeOf(testTextUnmarshaler{}),
			config: &DecodeConfig{
				TypeUnmarshalFuncs: map[reflect.Type]CustomizeDecodeFunc{
					reflect.TypeOf(testTextUnmarshaler{}): func(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
						return reflect.Value{}, errors.New("err")
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.Request{}
			params := param.Params{}
			config := tt.config
			if config == nil {
				config = &DecodeConfig{}
			}
			val, err := stringToValue(tt.elemType, tt.text, req, params, config)
			if tt.expectError {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.DeepEqual(t, tt.expectValue, val.Interface())
		})
	}
}

func TestTryTextUnmarshaler(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		text     string
		expected bool
	}{
		{
			name:     "text unmarshaler",
			value:    &testTextUnmarshaler{},
			text:     "test text",
			expected: true,
		},
		{
			name:     "non text unmarshaler",
			value:    &struct{}{},
			text:     "test text",
			expected: false,
		},
		{
			name:     "nil value",
			value:    nil,
			text:     "test text",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v reflect.Value
			if tt.value != nil {
				v = reflect.ValueOf(tt.value)
			} else {
				v = reflect.ValueOf(&tt.value).Elem()
			}

			result := tryTextUnmarshaler(v, tt.text)
			assert.DeepEqual(t, tt.expected, result)

			if tt.expected && tt.value != nil {
				// Verify the value was actually set
				unmarshaler := tt.value.(*testTextUnmarshaler)
				assert.DeepEqual(t, tt.text, unmarshaler.Value)
			}
		})
	}
}
