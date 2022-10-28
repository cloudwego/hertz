/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2014 Manuel Martínez-Almeida
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.

 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package render

import (
	"bytes"
	"encoding/json"

	hjson "github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// JSONMarshaler customize json.Marshal as you like
type JSONMarshaler func(v interface{}) ([]byte, error)

var jsonMarshalFunc JSONMarshaler

func init() {
	ResetJSONMarshal(hjson.Marshal)
}

func ResetJSONMarshal(fn JSONMarshaler) {
	jsonMarshalFunc = fn
}

func ResetStdJSONMarshal() {
	ResetJSONMarshal(json.Marshal)
}

// JSONRender JSON contains the given interface object.
type JSONRender struct {
	Data interface{}
}

var jsonContentType = "application/json; charset=utf-8"

// Render (JSON) writes data with custom ContentType.
func (r JSONRender) Render(resp *protocol.Response) error {
	writeContentType(resp, jsonContentType)
	jsonBytes, err := jsonMarshalFunc(r.Data)
	if err != nil {
		return err
	}

	resp.AppendBody(jsonBytes)
	return nil
}

// WriteContentType (JSON) writes JSON ContentType.
func (r JSONRender) WriteContentType(resp *protocol.Response) {
	writeContentType(resp, jsonContentType)
}

// PureJSON contains the given interface object.
type PureJSON struct {
	Data interface{}
}

// Render (JSON) writes data with custom ContentType.
func (r PureJSON) Render(resp *protocol.Response) (err error) {
	r.WriteContentType(resp)
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(r.Data)
	if err != nil {
		return
	}
	resp.AppendBody(buffer.Bytes())
	return
}

// WriteContentType (JSON) writes JSON ContentType.
func (r PureJSON) WriteContentType(resp *protocol.Response) {
	writeContentType(resp, jsonContentType)
}

// IndentedJSON contains the given interface object.
type IndentedJSON struct {
	Data interface{}
}

// Render (IndentedJSON) marshals the given interface object and writes it with custom ContentType.
func (r IndentedJSON) Render(resp *protocol.Response) (err error) {
	writeContentType(resp, jsonContentType)
	jsonBytes, err := jsonMarshalFunc(r.Data)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = json.Indent(&buf, jsonBytes, "", "    ")
	if err != nil {
		return err
	}
	resp.AppendBody(buf.Bytes())
	return nil
}

// WriteContentType (JSON) writes JSON ContentType.
func (r IndentedJSON) WriteContentType(resp *protocol.Response) {
	writeContentType(resp, jsonContentType)
}
