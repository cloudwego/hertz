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
 */

package assert

import (
	"reflect"
)

type testingT interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}

// Assert .
func Assert(t testingT, cond bool, val ...interface{}) {
	t.Helper()
	if !cond {
		if len(val) > 0 {
			val = append([]interface{}{"assertion failed:"}, val...)
			t.Fatal(val...)
		} else {
			t.Fatal("assertion failed")
		}
	}
}

// Assertf .
func Assertf(t testingT, cond bool, format string, val ...interface{}) {
	t.Helper()
	if !cond {
		t.Fatalf(format, val...)
	}
}

// DeepEqual .
func DeepEqual(t testingT, expected, actual interface{}) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("assertion failed, unexpected: %v, expected: %v", actual, expected)
	}
}

func isNil(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer,
		reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		if rv.IsNil() {
			return true
		}
	}
	return false
}

func Nil(t testingT, data interface{}) {
	t.Helper()
	if data == nil || isNil(reflect.ValueOf(data)) {
		return
	}
	t.Fatalf("assertion failed, unexpected: %v, expected: nil", data)
}

func NotNil(t testingT, data interface{}) {
	t.Helper()
	if data == nil || isNil(reflect.ValueOf(data)) {
		t.Fatalf("assertion failed, unexpected: %v, expected: not nil", data)
	}
}

// NotEqual .
func NotEqual(t testingT, expected, actual interface{}) {
	t.Helper()
	if expected == nil || actual == nil {
		if expected == actual {
			t.Fatalf("assertion failed: %v == %v", actual, expected)
		}
	}

	if reflect.DeepEqual(actual, expected) {
		t.Fatalf("assertion failed: %v == %v", actual, expected)
	}
}

func True(t testingT, obj interface{}) {
	t.Helper()
	DeepEqual(t, true, obj)
}

func False(t testingT, obj interface{}) {
	t.Helper()
	DeepEqual(t, false, obj)
}

// Panic .
func Panic(t testingT, fn func()) {
	t.Helper()
	defer func() {
		if err := recover(); err == nil {
			t.Fatal("assertion failed: did not panic")
		}
	}()
	fn()
}

// NotPanic .
func NotPanic(t testingT, fn func()) {
	t.Helper()
	defer func() {
		if err := recover(); err != nil {
			t.Fatal("assertion failed: panicked")
		}
	}()
	fn()
}
