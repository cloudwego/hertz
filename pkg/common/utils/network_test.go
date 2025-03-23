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

// TestGetLocalIp tests the getLocalIp function with different network interface scenarios
func TestGetLocalIp(t *testing.T) {
	// Since getLocalIp is not exported, we can test it indirectly through LocalIP
	// The actual value will depend on the test environment, but we can at least
	// verify that it returns a non-empty value that is not the unknown marker
	ip := LocalIP()
	assert.NotEqual(t, "", ip)
	
	// We can also verify that the returned IP is either a valid IP or the unknown marker
	if ip != UNKNOWN_IP_ADDR {
		parsedIP := net.ParseIP(ip)
		assert.NotNil(t, parsedIP, "LocalIP should return a valid IP address")
	}
}

// TestTLSRecordHeaderLooksLikeHTTPWithEdgeCases tests additional edge cases for TLS header detection
func TestTLSRecordHeaderLooksLikeHTTPWithEdgeCases(t *testing.T) {
	// Test with partial HTTP method matches
	partialMatches := [][5]byte{
		{'G', 'E', 'T', 'x', '/'}, // Almost "GET /"
		{'H', 'E', 'A', 'x', ' '}, // Almost "HEAD "
		{'P', 'O', 'S', 'x', ' '}, // Almost "POST "
		{'P', 'U', 'T', 'x', '/'}, // Almost "PUT /"
		{'O', 'P', 'T', 'x', 'O'}, // Almost "OPTIO"
	}
	
	for _, header := range partialMatches {
		assert.False(t, TLSRecordHeaderLooksLikeHTTP(header), 
			"Partial HTTP method match should not be detected as HTTP")
	}
	
	// Test with empty header
	emptyHeader := [5]byte{}
	assert.False(t, TLSRecordHeaderLooksLikeHTTP(emptyHeader),
		"Empty header should not be detected as HTTP")
	
	// Test with other common HTTP methods that are not in the list
	otherMethods := [][5]byte{
		{'D', 'E', 'L', 'E', 'T'}, // DELETE
		{'P', 'A', 'T', 'C', 'H'}, // PATCH
		{'T', 'R', 'A', 'C', 'E'}, // TRACE
	}
	
	for _, header := range otherMethods {
		assert.False(t, TLSRecordHeaderLooksLikeHTTP(header),
			"Other HTTP methods should not be detected as HTTP by this function")
	}
}
