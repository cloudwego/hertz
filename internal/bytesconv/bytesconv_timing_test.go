/*
 * Copyright 2024 CloudWeGo Authors
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

package bytesconv

import (
	"strings"
	"testing"

	"golang.org/x/net/http/httpguts"
)

func BenchmarkValidHeaderFiledValueTable(b *testing.B) {
	// Test all characters
	allBytes := make([]string, 0)
	for i := 0; i < 256; i++ {
		allBytes = append(allBytes, string([]byte{byte(i)}))
	}

	for i := 0; i < b.N; i++ {
		for _, s := range allBytes {
			_ = httpguts.ValidHeaderFieldValue(s)
		}
	}
}

func BenchmarkValidHeaderFiledValueTableHertz(b *testing.B) {
	// Test all characters
	allBytes := make([]byte, 0)
	for i := 0; i < 256; i++ {
		allBytes = append(allBytes, byte(i))
	}

	for i := 0; i < b.N; i++ {
		for _, s := range allBytes {
			_ = func() bool {
				return ValidHeaderFieldValueTable[s] != 0
			}()
		}
	}
}

func BenchmarkNewlineToSpace(b *testing.B) {
	// Test all characters
	allBytes := make([]byte, 0)
	for i := 0; i < 256; i++ {
		allBytes = append(allBytes, byte(i))
	}
	headerNewlineToSpace := strings.NewReplacer("\n", " ", "\r", " ")

	for i := 0; i < b.N; i++ {
		_ = headerNewlineToSpace.Replace(string(allBytes))
	}
}

func BenchmarkNewlineToSpaceHertz01(b *testing.B) {
	// Test all characters
	allBytes := make([]byte, 0)
	for i := 0; i < 256; i++ {
		allBytes = append(allBytes, byte(i))
	}

	for i := 0; i < b.N; i++ {
		filteredVal := make([]byte, 0, len(allBytes))
		for i := 0; i < len(allBytes); i++ {
			filteredVal = append(filteredVal, NewlineToSpaceTable[allBytes[i]])
		}
		_ = filteredVal
	}
}

func BenchmarkNewlineToSpaceHertz02(b *testing.B) {
	// Test all characters
	allBytes := make([]byte, 0)
	for i := 0; i < 256; i++ {
		allBytes = append(allBytes, byte(i))
	}

	for i := 0; i < b.N; i++ {
		filteredVal := make([]byte, len(allBytes))
		copy(filteredVal, allBytes)
		for ii := 0; ii < len(allBytes); ii++ {
			filteredVal[ii] = NewlineToSpaceTable[filteredVal[ii]]
		}
	}
}
