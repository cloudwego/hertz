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
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

type testTextUnmarshaler struct {
	Value string
}

func (t *testTextUnmarshaler) UnmarshalText(text []byte) error {
	t.Value = string(text)
	return nil
}

var _ textUnmarshaler = (*testTextUnmarshaler)(nil)

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
