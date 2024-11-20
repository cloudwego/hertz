// Package validator is a powerful validator that supports struct tag expression.
//
// Copyright 2019 Bytedance Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package validator

import (
	"errors"
	"io"
	"reflect"
	"strings"
	_ "unsafe"

	tagexpr "github.com/bytedance/go-tagexpr/v2"
)

const (
	// MatchExprName the name of the expression used for validation
	MatchExprName = tagexpr.DefaultExprName
	// ErrMsgExprName the name of the expression used to specify the message
	// returned when validation failed
	ErrMsgExprName = "msg"
)

// Validator struct fields validator
type Validator struct {
	vm         *tagexpr.VM
	errFactory func(failPath, msg string) error
}

// New creates a struct fields validator.
func New(tagName string) *Validator {
	v := &Validator{
		vm:         tagexpr.New(tagName),
		errFactory: defaultErrorFactory,
	}
	return v
}

// VM returns the struct tag expression interpreter.
func (v *Validator) VM() *tagexpr.VM {
	return v.vm
}

// Validate validates whether the fields of value is valid.
// NOTE:
//  If checkAll=true, validate all the error.
func (v *Validator) Validate(value interface{}, checkAll ...bool) error {
	var all bool
	if len(checkAll) > 0 {
		all = checkAll[0]
	}
	var errs = make([]error, 0, 8)
	err := v.vm.RunAny(value, func(te *tagexpr.TagExpr, err error) error {
		if err != nil {
			errs = append(errs, err)
			if all {
				return nil
			}
			return io.EOF
		}
		nilParentFields := make(map[string]bool, 16)
		err = te.Range(func(eh *tagexpr.ExprHandler) error {
			if strings.Contains(eh.StringSelector(), tagexpr.ExprNameSeparator) {
				return nil
			}
			r := eh.Eval()
			if r == nil {
				return nil
			}
			rerr, ok := r.(error)
			if !ok && tagexpr.FakeBool(r) {
				return nil
			}
			// Ignore this error if the value of the parent is nil
			if pfs, ok := eh.ExprSelector().ParentField(); ok {
				if nilParentFields[pfs] {
					return nil
				}
				if fh, ok := eh.TagExpr().Field(pfs); ok {
					v := fh.Value(false)
					if !v.IsValid() || (v.Kind() == reflect.Ptr && v.IsNil()) {
						nilParentFields[pfs] = true
						return nil
					}
				}
			}
			msg := eh.TagExpr().EvalString(eh.StringSelector() + tagexpr.ExprNameSeparator + ErrMsgExprName)
			if msg == "" && rerr != nil {
				msg = rerr.Error()
			}
			errs = append(errs, v.errFactory(eh.Path(), msg))
			if all {
				return nil
			}
			return io.EOF
		})
		if err != nil && !all {
			return err
		}
		return nil
	})
	if err != io.EOF && err != nil {
		return err
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		var errStr string
		for _, e := range errs {
			errStr += e.Error() + "\t"
		}
		return errors.New(errStr[:len(errStr)-1])
	}
}

// SetErrorFactory customizes the factory of validation error.
// NOTE:
//  If errFactory==nil, the default is used
func (v *Validator) SetErrorFactory(errFactory func(failPath, msg string) error) *Validator {
	if errFactory == nil {
		errFactory = defaultErrorFactory
	}
	v.errFactory = errFactory
	return v
}

// Error validate error
type Error struct {
	FailPath, Msg string
}

// Error implements error interface.
func (e *Error) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "invalid parameter: " + e.FailPath
}

//go:nosplit
func defaultErrorFactory(failPath, msg string) error {
	return &Error{
		FailPath: failPath,
		Msg:      msg,
	}
}
