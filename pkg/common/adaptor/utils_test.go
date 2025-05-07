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
	"io"
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
	rw := newBytesRWCloser(nil)
	assert.Assert(t, rw.Len() == 0)
	assert.Assert(t, rw.Size() == 0)
	n, err := rw.Read(nil)
	assert.Assert(t, n == 0)
	assert.Assert(t, err == io.EOF)

	rw.Write([]byte("hello"))
	b := make([]byte, 2)
	n, err = rw.Read(b)
	assert.Assert(t, n == 2)
	assert.Nil(t, err)
	assert.Assert(t, string(b) == "he")

	rw.ReadAt(b, 1)
	assert.Assert(t, string(b) == "el")

	rw.Seek(0, io.SeekStart)
	assert.Assert(t, rw.Len() == 5)
	rw.Seek(0, io.SeekCurrent)
	assert.Assert(t, rw.Len() == 5)
	rw.Seek(-1, io.SeekEnd)
	assert.Assert(t, rw.Len() == 1)
	n, err = rw.Read(b)
	assert.Assert(t, n == 1, n)
	assert.Nil(t, err)
	assert.Assert(t, b[0] == 'o')

	rw.Seek(0, io.SeekStart)
	buf := newBytesRWCloser(nil)
	_, err = rw.WriteTo(buf)
	assert.Nil(t, err)
	assert.Assert(t, string(buf.b) == "hello")
}
