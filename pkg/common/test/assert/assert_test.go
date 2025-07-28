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
	"fmt"
	"strings"
	"testing"
)

type mockTestingT struct {
	fatalstr string
}

func (mockTestingT) Helper() {}

func (m *mockTestingT) Reset() {
	m.fatalstr = ""
}

func (m *mockTestingT) Fatal(args ...any) {
	m.fatalstr = fmt.Sprintln(args...)
}

func (m *mockTestingT) Fatalf(fm string, args ...any) {
	m.fatalstr = fmt.Sprintf(fm, args...)
}

func (m *mockTestingT) String() string {
	return m.fatalstr
}

func (m *mockTestingT) Expect(t *testing.T, s string) {
	t.Helper()
	got := strings.TrimSpace(m.fatalstr)
	if got != s {
		t.Fatalf("got: %q expect: %q", got, s)
	}
	m.Reset()
}

func TestAssert(t *testing.T) {
	m := &mockTestingT{}
	Assert(m, true)
	m.Expect(t, "")

	Assert(m, false)
	m.Expect(t, "assertion failed")

	Assert(m, false, "hello")
	m.Expect(t, "assertion failed: hello")

	Assertf(m, true, "hello %s", "world")
	m.Expect(t, "")
	Assertf(m, false, "hello %s", "world")
	m.Expect(t, "hello world")
}

func TestNil(t *testing.T) {
	m := &mockTestingT{}

	Nil(m, nil)
	m.Expect(t, "")
	Nil(m, (*testing.T)(nil))
	m.Expect(t, "")

	Nil(m, 1)
	m.Expect(t, "assertion failed, unexpected: 1, expected: nil")

	Nil(m, "hello")
	m.Expect(t, "assertion failed, unexpected: hello, expected: nil")

	NotNil(m, 1)
	m.Expect(t, "")

	NotNil(m, "hello")
	m.Expect(t, "")

	NotNil(m, struct {
		hello string
	}{})
	m.Expect(t, "")

	NotNil(m, (*testing.T)(nil))
	m.Expect(t, `assertion failed, unexpected: <nil>, expected: not nil`)

}

func TestDeepEqual(t *testing.T) {
	m := &mockTestingT{}

	DeepEqual(m, 1, 1)
	m.Expect(t, "")

	DeepEqual(m, 1, 2)
	m.Expect(t, `assertion failed, unexpected: 2, expected: 1`)

}

func TestNotEqual(t *testing.T) {
	m := &mockTestingT{}

	NotEqual(m, 1, 2)
	m.Expect(t, "")

	NotEqual(m, nil, nil)
	m.Expect(t, `assertion failed: <nil> == <nil>`)
}

func TestTrueFalse(t *testing.T) {
	m := &mockTestingT{}

	True(m, true)
	m.Expect(t, "")

	False(m, false)
	m.Expect(t, "")
}

func TestPanic(t *testing.T) {
	m := &mockTestingT{}

	Panic(m, func() {
		panic("hello")
	})
	m.Expect(t, "")
	Panic(m, func() {
	})
	m.Expect(t, `assertion failed: did not panic`)

	NotPanic(m, func() {
	})
	m.Expect(t, "")

	NotPanic(m, func() {
		panic("hello")
	})
	m.Expect(t, `assertion failed: panicked`)

}
