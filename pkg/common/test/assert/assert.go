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
	"testing"
)

// Assert .
func Assert(t testing.TB, cond bool, val ...interface{}) {
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
func Assertf(t testing.TB, cond bool, format string, val ...interface{}) {
	t.Helper()
	if !cond {
		t.Fatalf(format, val...)
	}
}

// DeepEqual .
func DeepEqual(t testing.TB, expected, actual interface{}) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("assertion failed, unexpected: %v, expected: %v", actual, expected)
	}
}

func Nil(t testing.TB, data interface{}) {
	t.Helper()
	if data == nil {
		return
	}
	if !reflect.ValueOf(data).IsNil() {
		t.Fatalf("assertion failed, unexpected: %v, expected: nil", data)
	}
}

func NotNil(t testing.TB, data interface{}) {
	t.Helper()
	if data == nil {
		return
	}

	if reflect.ValueOf(data).IsNil() {
		t.Fatalf("assertion failed, unexpected: %v, expected: not nil", data)
	}
}

// NotEqual .
func NotEqual(t testing.TB, expected, actual interface{}) {
	t.Helper()
	if expected == nil || actual == nil {
		if expected == actual {
			t.Fatalf("assertion failed, unexpected: %v, expected: %v", actual, expected)
		}
	}

	if reflect.DeepEqual(actual, expected) {
		t.Fatalf("assertion failed, unexpected: %v, expected: %v", actual, expected)
	}
}

func True(t testing.TB, obj interface{}) {
	t.Helper()
	DeepEqual(t, true, obj)
}

func False(t testing.TB, obj interface{}) {
	t.Helper()
	DeepEqual(t, false, obj)
}

// Panic .
func Panic(t testing.TB, fn func()) {
	t.Helper()
	defer func() {
		if err := recover(); err == nil {
			t.Fatal("assertion failed: did not panic")
		}
	}()
	fn()
}

// NotPanic .
func NotPanic(t testing.TB, fn func()) {
	t.Helper()
	defer func() {
		if err := recover(); err != nil {
			t.Fatal("assertion failed: did panic")
		}
	}()
	fn()
}
