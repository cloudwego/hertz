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

package tagexpr

import (
	"strings"
)

const (
	// FieldSeparator in the expression selector,
	// the separator between field names
	FieldSeparator = "."
	// ExprNameSeparator in the expression selector,
	// the separator of the field name and expression name
	ExprNameSeparator = "@"
	// DefaultExprName the default name of single model expression
	DefaultExprName = ExprNameSeparator
)

// FieldSelector expression selector
type FieldSelector string

// Name returns the current field name.
func (f FieldSelector) Name() string {
	s := string(f)
	idx := strings.LastIndex(s, FieldSeparator)
	if idx == -1 {
		return s
	}
	return s[idx+1:]
}

// Split returns the path segments and the current field name.
func (f FieldSelector) Split() (paths []string, name string) {
	s := string(f)
	a := strings.Split(s, FieldSeparator)
	idx := len(a) - 1
	if idx > 0 {
		return a[:idx], a[idx]
	}
	return nil, s
}

// Parent returns the parent FieldSelector.
func (f FieldSelector) Parent() (string, bool) {
	s := string(f)
	i := strings.LastIndex(s, FieldSeparator)
	if i < 0 {
		return "", false
	}
	return s[:i], true
}

// String returns string type value.
func (f FieldSelector) String() string {
	return string(f)
}

// ExprSelector expression selector
type ExprSelector string

// Name returns the name of the expression.
func (e ExprSelector) Name() string {
	s := string(e)
	atIdx := strings.LastIndex(s, ExprNameSeparator)
	if atIdx == -1 {
		return DefaultExprName
	}
	return s[atIdx+1:]
}

// Field returns the field selector it belongs to.
func (e ExprSelector) Field() string {
	s := string(e)
	idx := strings.LastIndex(s, ExprNameSeparator)
	if idx != -1 {
		s = s[:idx]
	}
	return s
}

// ParentField returns the parent field selector it belongs to.
func (e ExprSelector) ParentField() (string, bool) {
	return FieldSelector(e.Field()).Parent()
}

// Split returns the field selector and the expression name.
func (e ExprSelector) Split() (field FieldSelector, name string) {
	s := string(e)
	atIdx := strings.LastIndex(s, ExprNameSeparator)
	if atIdx == -1 {
		return FieldSelector(s), DefaultExprName
	}
	return FieldSelector(s[:atIdx]), s[atIdx+1:]
}

// String returns string type value.
func (e ExprSelector) String() string {
	return string(e)
}
