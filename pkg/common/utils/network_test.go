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

package utils

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestTLSRecordHeaderLooksLikeHTTP(t *testing.T) {
	HeaderValueAndExpectedResult := [][]interface{}{
		{[5]byte{'G', 'E', 'T', ' ', '/'}, true},
		{[5]byte{'H', 'E', 'A', 'D', ' '}, true},
		{[5]byte{'P', 'O', 'S', 'T', ' '}, true},
		{[5]byte{'P', 'U', 'T', ' ', '/'}, true},
		{[5]byte{'O', 'P', 'T', 'I', 'O'}, true},
		{[5]byte{'G', 'E', 'T', '/', ' '}, false},
		{[5]byte{' ', 'H', 'E', 'A', 'D'}, false},
		{[5]byte{' ', 'P', 'O', 'S', 'T'}, false},
		{[5]byte{'P', 'U', 'T', '/', ' '}, false},
		{[5]byte{'H', 'E', 'R', 'T', 'Z'}, false},
	}

	for _, testCase := range HeaderValueAndExpectedResult {
		value, expectedResult := testCase[0].([5]byte), testCase[1].(bool)
		assert.DeepEqual(t, expectedResult, TLSRecordHeaderLooksLikeHTTP(value))
	}
}

func TestLocalIP(t *testing.T) {
	// Mock the localIP variable for testing purposes.
	localIP = "192.168.0.1"

	// Ensure that LocalIP() returns the expected local IP.
	expectedIP := "192.168.0.1"
	if got := LocalIP(); got != expectedIP {
		assert.DeepEqual(t, got, expectedIP)
	}
}
