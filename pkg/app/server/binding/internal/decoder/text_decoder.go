/*
 * Copyright 2023 CloudWeGo Authors
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
 * Modifications are Copyright 2023 CloudWeGo Authors
 */

package decoder

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/cloudwego/hertz/internal/bytesconv"
	hJson "github.com/cloudwego/hertz/pkg/common/json"
)

type TextDecoder interface {
	UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error
}

func SelectTextDecoder(rt reflect.Type) (TextDecoder, error) {
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
	case reflect.Interface:
		return &interfaceDecoder{}, nil
	}

	return nil, fmt.Errorf("unsupported type " + rt.String())
}

type boolDecoder struct{}

func (d *boolDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "false"
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	fieldValue.SetBool(v)
	return nil
}

type floatDecoder struct {
	bitSize int
}

func (d *floatDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "0.0"
	}
	v, err := strconv.ParseFloat(s, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetFloat(v)
	return nil
}

type intDecoder struct {
	bitSize int
}

func (d *intDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "0"
	}
	v, err := strconv.ParseInt(s, 10, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetInt(v)
	return nil
}

type stringDecoder struct{}

func (d *stringDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	fieldValue.SetString(s)
	return nil
}

type uintDecoder struct {
	bitSize int
}

func (d *uintDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "0"
	}
	v, err := strconv.ParseUint(s, 10, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetUint(v)
	return nil
}

type interfaceDecoder struct{}

func (d *interfaceDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "0"
	}
	return hJson.Unmarshal(bytesconv.S2b(s), fieldValue.Addr().Interface())
}
