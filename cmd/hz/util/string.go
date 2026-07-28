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

package util

import (
	"reflect"
	"strings"
	"unicode/utf8"
	"unsafe"
)

// Str2Bytes performs a zero-copy string to byte slice conversion using unsafe pointer casting.
// The returned slice shares memory with the input string; do NOT modify it.
func Str2Bytes(in string) (out []byte) {
	op := (*reflect.SliceHeader)(unsafe.Pointer(&out))
	ip := (*reflect.StringHeader)(unsafe.Pointer(&in))
	op.Data = ip.Data
	op.Cap = ip.Len
	op.Len = ip.Len
	return
}

func Bytes2Str(in []byte) (out string) {
	op := (*reflect.StringHeader)(unsafe.Pointer(&out))
	ip := (*reflect.SliceHeader)(unsafe.Pointer(&in))
	op.Data = ip.Data
	op.Len = ip.Len
	return
}

// TrimLastChar can remove the last char for s
func TrimLastChar(s string) string {
	r, size := utf8.DecodeLastRuneInString(s)
	if r == utf8.RuneError && (size == 0 || size == 1) {
		size = 0
	}
	return s[:len(s)-size]
}

// AddSlashForComments can adjust the format of multi-line comments
func AddSlashForComments(s string) string {
	s = strings.Replace(s, "\n", "\n//", -1)
	return s
}

// CamelString converts the string 's' to PascalCase (upper camel case).
// Underscores before lowercase letters are removed and the following letter is capitalized.
// State flags: j = next char should be uppercased (after underscore), k = seen first letter already.
func CamelString(s string) string {
	data := make([]byte, 0, len(s))
	j := false // capitalize next character
	k := false // have we seen the first letter
	num := len(s) - 1
	for i := 0; i <= num; i++ {
		d := s[i]
		if k == false && d >= 'A' && d <= 'Z' {
			k = true
		}
		if d >= 'a' && d <= 'z' && (j || k == false) {
			d = d - 32 // to uppercase
			j = false
			k = true
		}
		if k && d == '_' && num > i && s[i+1] >= 'a' && s[i+1] <= 'z' {
			j = true
			continue // skip the underscore
		}
		data = append(data, d)
	}
	return Bytes2Str(data[:])
}

// SnakeString converts the string 's' to snake_case.
// Inserts '_' before uppercase letters that follow a lowercase letter or non-underscore.
// j tracks whether we've seen a non-underscore lowercase char (to avoid leading underscores).
func SnakeString(s string) string {
	data := make([]byte, 0, len(s)*2)
	j := false // true after seeing a non-underscore char (prevents "_" before first upper)
	for _, d := range Str2Bytes(s) {
		if d >= 'A' && d <= 'Z' {
			if j {
				data = append(data, '_')
				j = false
			}
		} else if d != '_' {
			j = true
		}
		data = append(data, d)
	}
	return strings.ToLower(Bytes2Str(data))
}
