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
	"fmt"
	"reflect"
	"strings"
)

const (
	specialChar = "\x07"
)

// toDefaultValue will preprocess the default value and transfer it to be standard format
func toDefaultValue(typ reflect.Type, defaultValue string) string {
	switch typ.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		// escape single quote and double quote, replace single quote with double quote
		defaultValue = strings.Replace(defaultValue, `"`, `\"`, -1)
		defaultValue = strings.Replace(defaultValue, `\'`, specialChar, -1)
		defaultValue = strings.Replace(defaultValue, `'`, `"`, -1)
		defaultValue = strings.Replace(defaultValue, specialChar, `'`, -1)
	}
	return defaultValue
}

func newJSONDecodeErr(typ reflect.Type, err error) error {
	return fmt.Errorf("json.Unmarshal for type %s err: %w", typ, err)
}
