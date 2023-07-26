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
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors
 */

package errors

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	// These errors are the base error, which are used for checking in errors.Is()
	ErrNeedMore           = errors.New("need more data")
	ErrChunkedStream      = errors.New("chunked stream")
	ErrBodyTooLarge       = errors.New("body size exceeds the given limit")
	ErrHijacked           = errors.New("connection has been hijacked")
	ErrTimeout            = errors.New("timeout")
	ErrIdleTimeout        = errors.New("idle timeout")
	ErrNothingRead        = errors.New("nothing read")
	ErrShortConnection    = errors.New("short connection")
	ErrNoFreeConns        = errors.New("no free connections available to host")
	ErrConnectionClosed   = errors.New("connection closed")
	ErrNotSupportProtocol = errors.New("not support protocol")
	ErrNoMultipartForm    = errors.New("request has no multipart/form-data Content-Type")
	ErrBadPoolConn        = errors.New("connection is closed by peer while being in the connection pool")
)

// ErrorType is an unsigned 64-bit error code as defined in the hertz spec.
type ErrorType uint64

type Error struct {
	Err  error
	Type ErrorType
	Meta interface{}
}

const (
	// ErrorTypeBind is used when Context.Bind() fails.
	ErrorTypeBind ErrorType = 1 << iota
	// ErrorTypeRender is used when Context.Render() fails.
	ErrorTypeRender
	// ErrorTypePrivate indicates a private error.
	ErrorTypePrivate
	// ErrorTypePublic indicates a public error.
	ErrorTypePublic
	// ErrorTypeAny indicates any other error.
	ErrorTypeAny
)

type ErrorChain []*Error

var _ error = (*Error)(nil)

// SetType sets the error's type.
func (msg *Error) SetType(flags ErrorType) *Error {
	msg.Type = flags
	return msg
}

// AbortWithMsg implements the error interface.
func (msg *Error) Error() string {
	return msg.Err.Error()
}

func (a ErrorChain) String() string {
	if len(a) == 0 {
		return ""
	}
	var buffer strings.Builder
	for i, msg := range a {
		fmt.Fprintf(&buffer, "Error #%02d: %s\n", i+1, msg.Err)
		if msg.Meta != nil {
			fmt.Fprintf(&buffer, "     Meta: %v\n", msg.Meta)
		}
	}
	return buffer.String()
}

func (msg *Error) Unwrap() error {
	return msg.Err
}

// SetMeta sets the error's meta data.
func (msg *Error) SetMeta(data interface{}) *Error {
	msg.Meta = data
	return msg
}

// IsType judges one error.
func (msg *Error) IsType(flags ErrorType) bool {
	return (msg.Type & flags) > 0
}

// JSON creates a properly formatted JSON
func (msg *Error) JSON() interface{} {
	jsonData := make(map[string]interface{})
	if msg.Meta != nil {
		value := reflect.ValueOf(msg.Meta)
		switch value.Kind() {
		case reflect.Struct:
			return msg.Meta
		case reflect.Map:
			for _, key := range value.MapKeys() {
				jsonData[key.String()] = value.MapIndex(key).Interface()
			}
		default:
			jsonData["meta"] = msg.Meta
		}
	}
	if _, ok := jsonData["error"]; !ok {
		jsonData["error"] = msg.Error()
	}
	return jsonData
}

// Errors returns an array will all the error messages.
// Example:
//
//	c.Error(errors.New("first"))
//	c.Error(errors.New("second"))
//	c.Error(errors.New("third"))
//	c.Errors.Errors() // == []string{"first", "second", "third"}
func (a ErrorChain) Errors() []string {
	if len(a) == 0 {
		return nil
	}
	errorStrings := make([]string, len(a))
	for i, err := range a {
		errorStrings[i] = err.Error()
	}
	return errorStrings
}

// ByType returns a readonly copy filtered the byte.
// ie ByType(hertz.ErrorTypePublic) returns a slice of errors with type=ErrorTypePublic.
func (a ErrorChain) ByType(typ ErrorType) ErrorChain {
	if len(a) == 0 {
		return nil
	}
	if typ == ErrorTypeAny {
		return a
	}
	var result ErrorChain
	for _, msg := range a {
		if msg.IsType(typ) {
			result = append(result, msg)
		}
	}
	return result
}

// Last returns the last error in the slice. It returns nil if the array is empty.
// Shortcut for errors[len(errors)-1].
func (a ErrorChain) Last() *Error {
	if length := len(a); length > 0 {
		return a[length-1]
	}
	return nil
}

func (a ErrorChain) JSON() interface{} {
	switch length := len(a); length {
	case 0:
		return nil
	case 1:
		return a.Last().JSON()
	default:
		jsonData := make([]interface{}, length)
		for i, err := range a {
			jsonData[i] = err.JSON()
		}
		return jsonData
	}
}

func New(err error, t ErrorType, meta interface{}) *Error {
	return &Error{
		Err:  err,
		Type: t,
		Meta: meta,
	}
}

// shortcut for creating a public *Error from string
func NewPublic(err string) *Error {
	return New(errors.New(err), ErrorTypePublic, nil)
}

func NewPrivate(err string) *Error {
	return New(errors.New(err), ErrorTypePrivate, nil)
}

func Newf(t ErrorType, meta interface{}, format string, v ...interface{}) *Error {
	return New(fmt.Errorf(format, v...), t, meta)
}

func NewPublicf(format string, v ...interface{}) *Error {
	return New(fmt.Errorf(format, v...), ErrorTypePublic, nil)
}

func NewPrivatef(format string, v ...interface{}) *Error {
	return New(fmt.Errorf(format, v...), ErrorTypePrivate, nil)
}
