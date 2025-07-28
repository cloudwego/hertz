/*
 * Copyright 2025 CloudWeGo Authors
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

package adaptor

import (
	"runtime"
	"testing"

	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestMethodStr(t *testing.T) {
	var m0, m1 runtime.MemStats
	assertEqual := func(got, expect string) {
		if got != expect {
			t.Helper()
			t.Fatal(got)
		}
	}
	runtime.ReadMemStats(&m0)
	for i := 0; i < 1000; i++ {
		assertEqual(methodstr([]byte(nil)), "GET")
		assertEqual(methodstr(bytestr.StrGet), "GET")
		assertEqual(methodstr(bytestr.StrHead), "HEAD")
		assertEqual(methodstr(bytestr.StrPost), "POST")
		assertEqual(methodstr(bytestr.StrPut), "PUT")
		assertEqual(methodstr(bytestr.StrDelete), "DELETE")
		assertEqual(methodstr(bytestr.StrPatch), "PATCH")
	}
	runtime.ReadMemStats(&m1)

	// should be zero, but in case of other background task running
	diff := m1.Mallocs - m0.Mallocs
	assert.Assert(t, diff < 50, diff)
}

func TestBytesRWCloser(t *testing.T) {
	// Test data
	testData := []byte("test bytesRWCloser")

	// Create a new bytesRWCloser
	rc := newBytesRWCloser(testData)

	// Read from the reader
	buf := make([]byte, len(testData))
	n, err := rc.Read(buf)

	// Verify read was successful
	assert.Nil(t, err)
	assert.DeepEqual(t, n, len(testData))
	assert.DeepEqual(t, buf, testData)

	// Test that Close returns nil
	err = rc.Close()
	assert.Nil(t, err)
}

func TestParseHTTPVersion(t *testing.T) {
	// Test HTTP/1.0
	major, minor, err := parseHTTPVersion("HTTP/1.0")
	assert.Nil(t, err)
	assert.DeepEqual(t, major, 1)
	assert.DeepEqual(t, minor, 0)

	// Test HTTP/1.1
	major, minor, err = parseHTTPVersion("HTTP/1.1")
	assert.Nil(t, err)
	assert.DeepEqual(t, major, 1)
	assert.DeepEqual(t, minor, 1)

	// Test HTTP/2.0
	major, minor, err = parseHTTPVersion("HTTP/2.0")
	assert.Nil(t, err)
	assert.DeepEqual(t, major, 2)
	assert.DeepEqual(t, minor, 0)

	// Test HTTP/3.1
	major, minor, err = parseHTTPVersion("HTTP/3.1")
	assert.Nil(t, err)
	assert.DeepEqual(t, major, 3)
	assert.DeepEqual(t, minor, 1)

	// Test missing HTTP prefix
	major, minor, err = parseHTTPVersion("1.1")
	assert.NotNil(t, err)
	assert.DeepEqual(t, major, 1)
	assert.DeepEqual(t, minor, 1)

	// Test missing dot separator
	major, minor, err = parseHTTPVersion("HTTP/11")
	assert.NotNil(t, err)
	assert.DeepEqual(t, major, 1)
	assert.DeepEqual(t, minor, 1)

	// Test empty string
	major, minor, err = parseHTTPVersion("")
	assert.NotNil(t, err)
	assert.DeepEqual(t, major, 1)
	assert.DeepEqual(t, minor, 1)
}
