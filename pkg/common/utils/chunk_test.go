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
	"io"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
)

func TestChunkParseChunkSizeGetCorrect(t *testing.T) {
	// iterate the hexMap, and judge the difference between dec and ParseChunkSize
	hexMap := map[int]string{0: "0", 10: "a", 100: "64", 1000: "3e8"}
	for dec, hex := range hexMap {
		chunkSizeBody := hex + "\r\n"
		zr := mock.NewZeroCopyReader(chunkSizeBody)
		chunkSize, err := ParseChunkSize(zr)
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, chunkSize, dec)
	}
}

func TestChunkParseChunkSizeGetError(t *testing.T) {
	// test err from -----n, err := bytesconv.ReadHexInt(r)-----
	chunkSizeBody := ""
	zr := mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err := ParseChunkSize(zr)
	assert.NotNil(t, err)
	assert.DeepEqual(t, -1, chunkSize)
	// test err from -----c, err := r.ReadByte()-----
	chunkSizeBody = "0"
	zr = mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err = ParseChunkSize(zr)
	assert.NotNil(t, err)
	assert.DeepEqual(t, -1, chunkSize)
	// test err from -----c, err := r.ReadByte()-----
	chunkSizeBody = "0" + "\r"
	zr = mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err = ParseChunkSize(zr)
	assert.NotNil(t, err)
	assert.DeepEqual(t, -1, chunkSize)
	// test err from -----c, err := r.ReadByte()-----
	chunkSizeBody = "0" + "\r" + "\r"
	zr = mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err = ParseChunkSize(zr)
	assert.NotNil(t, err)
	assert.DeepEqual(t, -1, chunkSize)
}

func TestChunkParseChunkSizeCorrectWhiteSpace(t *testing.T) {
	// test the whitespace
	whiteSpace := ""
	for i := 0; i < 10; i++ {
		whiteSpace += " "
		chunkSizeBody := "0" + whiteSpace + "\r\n"
		zr := mock.NewZeroCopyReader(chunkSizeBody)
		chunkSize, err := ParseChunkSize(zr)
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, 0, chunkSize)
	}
}

func TestChunkParseChunkSizeNonCRLF(t *testing.T) {
	// test non-"\r\n"
	chunkSizeBody := "0" + "\n\r"
	zr := mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err := ParseChunkSize(zr)
	assert.DeepEqual(t, true, err != nil)
	assert.DeepEqual(t, -1, chunkSize)
}

func TestChunkReadTrueCRLF(t *testing.T) {
	CRLF := "\r\n"
	zr := mock.NewZeroCopyReader(CRLF)
	err := SkipCRLF(zr)
	assert.DeepEqual(t, nil, err)
}

func TestChunkReadFalseCRLF(t *testing.T) {
	CRLF := "\n\r"
	zr := mock.NewZeroCopyReader(CRLF)
	err := SkipCRLF(zr)
	assert.DeepEqual(t, errBrokenChunk, err)
}

// mockReader is a mock implementation of network.Reader interface
type mockReader struct {
	data []byte
	pos  int
}

func (r *mockReader) Release() error {
	r.pos = 0
	r.data = r.data[:0]
	return nil
}

func (r *mockReader) Len() int {
	return len(r.data)
}

func (r *mockReader) ReadBinary(n int) (p []byte, err error) {
	return
}

func (r *mockReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *mockReader) ReadByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *mockReader) Peek(n int) ([]byte, error) {
	if r.pos+n > len(r.data) {
		return nil, io.EOF
	}
	return r.data[r.pos : r.pos+n], nil
}

func (r *mockReader) Skip(n int) error {
	if r.pos+n > len(r.data) {
		return io.EOF
	}
	r.pos += n
	return nil
}

// BenchmarkParseChunkSize benchmarks the ParseChunkSize function with different inputs
func BenchmarkParseChunkSize(b *testing.B) {
	// create a slice of mock readers with different chunk sizes
	readers := []*mockReader{
		{data: []byte("1\r\n")},
		{data: []byte("10\r\n")},
		{data: []byte("100\r\n")},
		{data: []byte("1000\r\n")},
		{data: []byte("10000\r\n")},
		{data: []byte("100000\r\n")},
		{data: []byte("1000000\r\n")},
	}

	// run the ParseChunkSize function b.N times for each reader
	for _, r := range readers {
		b.Run(string(r.data), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				ParseChunkSize(r)
				r.pos = 0 // reset the reader position
			}
		})
	}
}

// BenchmarkSkipCRLF benchmarks the SkipCRLF function with different inputs
func BenchmarkSkipCRLF(b *testing.B) {
	// create a slice of mock readers with different data
	readers := []*mockReader{
		{data: []byte("\r\n")},
		{data: []byte("foo\r\n")},
		{data: []byte("bar\r\n")},
		{data: []byte("baz\r\n")},
	}

	// run the SkipCRLF function b.N times for each reader
	for _, r := range readers {
		b.Run(string(r.data), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				SkipCRLF(r)
				r.pos = 0 // reset the reader position
			}
		})
	}
}
