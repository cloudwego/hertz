// Copyright 2019 Bytedance Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validator

import (
	"errors"
	"regexp"

	"github.com/nyaruka/phonenumbers"

	"github.com/cloudwego/hertz/internal/tagexpr"
)

// ErrInvalidWithoutMsg verification error without error message.
var ErrInvalidWithoutMsg = errors.New("")

// MustRegFunc registers validator function expression.
// NOTE:
//
//	panic if exist error;
//	example: phone($) or phone($,'CN');
//	If @force=true, allow to cover the existed same @funcName;
//	The go number types always are float64;
//	The go string types always are string.
func MustRegFunc(funcName string, fn func(args ...interface{}) error, force ...bool) {
	err := RegFunc(funcName, fn, force...)
	if err != nil {
		panic(err)
	}
}

// RegFunc registers validator function expression.
// NOTE:
//
//	example: phone($) or phone($,'CN');
//	If @force=true, allow to cover the existed same @funcName;
//	The go number types always are float64;
//	The go string types always are string.
func RegFunc(funcName string, fn func(args ...interface{}) error, force ...bool) error {
	return tagexpr.RegFunc(funcName, func(args ...interface{}) interface{} {
		err := fn(args...)
		if err == nil {
			// nil defaults to false, so returns true
			return true
		}
		return err
	}, force...)
}

func init() {
	pattern := "^([A-Za-z0-9_\\-\\.\u4e00-\u9fa5])+\\@([A-Za-z0-9_\\-\\.])+\\.([A-Za-z]{2,8})$"
	emailRegexp := regexp.MustCompile(pattern)
	MustRegFunc("email", func(args ...interface{}) error {
		if len(args) != 1 {
			return errors.New("number of parameters of email function is not one")
		}
		s, ok := args[0].(string)
		if !ok {
			return errors.New("parameter of email function is not string type")
		}
		matched := emailRegexp.MatchString(s)
		if !matched {
			// return ErrInvalidWithoutMsg
			return errors.New("email format is incorrect")
		}
		return nil
	}, true)
}

func init() {
	// phone: defaultRegion is 'CN'
	MustRegFunc("phone", func(args ...interface{}) error {
		var numberToParse, defaultRegion string
		var ok bool
		switch len(args) {
		default:
			return errors.New("the number of parameters of phone function is not one or two")
		case 2:
			defaultRegion, ok = args[1].(string)
			if !ok {
				return errors.New("the 2nd parameter of phone function is not string type")
			}
			fallthrough
		case 1:
			numberToParse, ok = args[0].(string)
			if !ok {
				return errors.New("the 1st parameter of phone function is not string type")
			}
		}
		if defaultRegion == "" {
			defaultRegion = "CN"
		}
		num, err := phonenumbers.Parse(numberToParse, defaultRegion)
		if err != nil {
			return err
		}
		matched := phonenumbers.IsValidNumber(num)
		if !matched {
			// return ErrInvalidWithoutMsg
			return errors.New("phone format is incorrect")
		}
		return nil
	}, true)
}
