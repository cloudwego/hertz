/*
 * Copyright 2022 CloudWeGo Authors
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
 * MIT License
 *
 * Copyright (c) 2019-present Fenny and Contributors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package text_decoder

import (
	"fmt"
	"reflect"
)

type TextDecoder interface {
	UnmarshalString(s string, fieldValue reflect.Value) error
}

// var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func SelectTextDecoder(rt reflect.Type) (TextDecoder, error) {
	// todo: encoding.TextUnmarshaler
	//if reflect.PtrTo(rt).Implements(textUnmarshalerType) {
	//	return &textUnmarshalEncoder{fieldType: rt}, nil
	//}

	switch rt.Kind() {
	case reflect.Bool:
		return &boolDecoder{}, nil
	case reflect.Uint8:
		return &uintDecoder{bitSize: 8}, nil
	case reflect.Uint16:
		return &uintDecoder{bitSize: 16}, nil
	case reflect.Uint32:
		return &uintDecoder{bitSize: 32}, nil
	case reflect.Uint64:
		return &uintDecoder{bitSize: 64}, nil
	case reflect.Uint:
		return &uintDecoder{}, nil
	case reflect.Int8:
		return &intDecoder{bitSize: 8}, nil
	case reflect.Int16:
		return &intDecoder{bitSize: 16}, nil
	case reflect.Int32:
		return &intDecoder{bitSize: 32}, nil
	case reflect.Int64:
		return &intDecoder{bitSize: 64}, nil
	case reflect.Int:
		return &intDecoder{}, nil
	case reflect.String:
		return &stringDecoder{}, nil
	case reflect.Float32:
		return &floatDecoder{bitSize: 32}, nil
	case reflect.Float64:
		return &floatDecoder{bitSize: 64}, nil
	}

	return nil, fmt.Errorf("unsupported type " + rt.String())
}
