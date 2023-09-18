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
	"bytes"
	"io"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/mock"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/network"
)

type writeReadTest interface {
	Write(p []byte) (n int, err error)
	Malloc(n int) (buf []byte, err error)
	WriteBinary(b []byte) (n int, err error)
	Flush() error
}

type readerTest interface {
	ReadFrom(r io.Reader) (n int64, err error)
	Malloc(n int) (buf []byte, err error)
	WriteBinary(b []byte) (n int, err error)
	Flush() error
}

type testWriter struct {
	w io.Writer
}

func (t testWriter) Write(p []byte) (n int, err error) {
	return
}

func (t testWriter) Malloc(n int) (buf []byte, err error) {
	return
}

func (t testWriter) WriteBinary(b []byte) (n int, err error) {
	return
}

func (t testWriter) Flush() error {
	return nil
}

type testReader struct {
	r io.ReaderFrom
}

func (t testReader) ReadFrom(r io.Reader) (n int64, err error) {
	return
}

func (t testReader) Malloc(n int) (buf []byte, err error) {
	return
}

func (t testReader) WriteBinary(b []byte) (n int, err error) {
	return
}

func (t testReader) Flush() error {
	return nil
}

func newTestWriter(w io.Writer) writeReadTest {
	return &testWriter{
		w: w,
	}
}

func newTestReaderForm(r io.ReaderFrom) readerTest {
	return &testReader{
		r: r,
	}
}

func TestIoutilCopyBuffer(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := string("hertz is very good!!!")
	src := bytes.NewBufferString(str)
	dst := network.NewWriter(&writeBuffer)
	var buf []byte
	// src.Len() will change, when use src.read(p []byte)
	srcLen := int64(src.Len())
	written, err := CopyBuffer(dst, src, buf)

	assert.DeepEqual(t, written, srcLen)
	assert.DeepEqual(t, err, nil)
	assert.DeepEqual(t, []byte(str), writeBuffer.Bytes())

	// Test when no data is readable
	writeBuffer.Reset()
	emptySrc := bytes.NewBufferString("")
	written, err = CopyBuffer(dst, emptySrc, buf)
	assert.DeepEqual(t, written, int64(0))
	assert.Nil(t, err)
	assert.DeepEqual(t, []byte(""), writeBuffer.Bytes())

	// Test a LimitedReader
	writeBuffer.Reset()
	limit := int64(5)
	limitedSrc := io.LimitedReader{R: bytes.NewBufferString(str), N: limit}
	written, err = CopyBuffer(dst, &limitedSrc, buf)
	assert.DeepEqual(t, written, limit)
	assert.Nil(t, err)
	assert.DeepEqual(t, []byte(str[:limit]), writeBuffer.Bytes())
}

func TestIoutilCopyBufferWithIoWriter(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := "hertz is very good!!!"
	var buf []byte
	src := bytes.NewBuffer([]byte(str))
	ioWriter := newTestWriter(&writeBuffer)
	// to show example about -----w, ok := dst.(io.Writer)-----
	_, ok := ioWriter.(io.Writer)
	assert.DeepEqual(t, true, ok)
	written, err := CopyBuffer(ioWriter, src, buf)
	assert.DeepEqual(t, written, int64(0))
	assert.NotNil(t, err)
	assert.DeepEqual(t, []byte(nil), writeBuffer.Bytes())
}

func TestIoutilCopyBufferWithIoReaderFrom(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := "hertz is very good!!!"
	var buf []byte
	src := bytes.NewBufferString(str)
	ioReaderFrom := newTestReaderForm(&writeBuffer)
	// to show example about -----rf, ok := dst.(io.ReaderFrom)-----
	_, ok := ioReaderFrom.(io.Writer)
	assert.DeepEqual(t, false, ok)
	_, ok = ioReaderFrom.(io.ReaderFrom)
	assert.DeepEqual(t, true, ok)
	written, err := CopyBuffer(ioReaderFrom, src, buf)
	assert.DeepEqual(t, written, int64(0))
	assert.NotNil(t, err)
	assert.DeepEqual(t, []byte(nil), writeBuffer.Bytes())
}

func TestIoutilCopyBufferWithPanic(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := "hertz is very good!!!"
	var buf []byte
	defer func() {
		if r := recover(); r != nil {
			assert.DeepEqual(t, "empty buffer in io.CopyBuffer", r)
		}
	}()
	src := bytes.NewBufferString(str)
	dst := network.NewWriter(&writeBuffer)
	buf = make([]byte, 0)
	_, _ = CopyBuffer(dst, src, buf)
}

func TestIoutilCopyBufferWithNilBuffer(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := string("hertz is very good!!!")
	src := bytes.NewBufferString(str)
	dst := network.NewWriter(&writeBuffer)
	// src.Len() will change, when use src.read(p []byte)
	srcLen := int64(src.Len())
	written, err := CopyBuffer(dst, src, nil)

	assert.DeepEqual(t, written, srcLen)
	assert.NotNil(t, err)
	assert.DeepEqual(t, []byte(str), writeBuffer.Bytes())
}

func TestIoutilCopyBufferWithNilBufferAndIoLimitedReader(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := "hertz is very good!!!"
	src := bytes.NewBufferString(str)
	reader := mock.NewLimitReader(src)
	dst := network.NewWriter(&writeBuffer)
	srcLen := int64(src.Len())
	written, err := CopyBuffer(dst, &reader, nil)

	assert.DeepEqual(t, written, srcLen)
	assert.NotNil(t, err)
	assert.DeepEqual(t, []byte(str), writeBuffer.Bytes())

	// test l.N < 1
	writeBuffer.Reset()
	str = ""
	src = bytes.NewBufferString(str)
	reader = mock.NewLimitReader(src)
	dst = network.NewWriter(&writeBuffer)
	srcLen = int64(src.Len())
	written, err = CopyBuffer(dst, &reader, nil)

	assert.DeepEqual(t, written, srcLen)
	assert.NotNil(t, err)
	assert.DeepEqual(t, []byte(str), writeBuffer.Bytes())
}

func TestIoutilCopyZeroAlloc(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := "hertz is very good!!!"
	src := bytes.NewBufferString(str)
	dst := network.NewWriter(&writeBuffer)
	srcLen := int64(src.Len())
	written, err := CopyZeroAlloc(dst, src)

	assert.DeepEqual(t, written, srcLen)
	assert.DeepEqual(t, err, nil)
	assert.DeepEqual(t, []byte(str), writeBuffer.Bytes())

	// Test when no data is readable
	writeBuffer.Reset()
	emptySrc := bytes.NewBufferString("")
	written, err = CopyZeroAlloc(dst, emptySrc)
	assert.DeepEqual(t, written, int64(0))
	assert.Nil(t, err)
	assert.DeepEqual(t, []byte(""), writeBuffer.Bytes())
}

func TestIoutilCopyBufferWithEmptyBuffer(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := "hertz is very good!!!"
	src := bytes.NewBufferString(str)
	dst := network.NewWriter(&writeBuffer)
	// Use a non-empty buffer of length 0
	emptyBuf := make([]byte, 0)
	func() {
		defer func() {
			if r := recover(); r != nil {
				assert.DeepEqual(t, "empty buffer in io.CopyBuffer", r)
			}
		}()

		written, err := CopyBuffer(dst, src, emptyBuf)
		assert.Nil(t, err)
		assert.DeepEqual(t, written, int64(len(str)))
		assert.DeepEqual(t, []byte(str), writeBuffer.Bytes())
	}()
}

func TestIoutilCopyBufferWithLimitedReader(t *testing.T) {
	var writeBuffer bytes.Buffer
	str := "hertz is very good!!!"
	src := bytes.NewBufferString(str)
	limit := int64(5)
	limitedSrc := io.LimitedReader{R: src, N: limit}
	dst := network.NewWriter(&writeBuffer)
	var buf []byte

	// Test LimitedReader status
	written, err := CopyBuffer(dst, &limitedSrc, buf)
	assert.Nil(t, err)
	assert.DeepEqual(t, written, limit)
	assert.DeepEqual(t, []byte(str[:limit]), writeBuffer.Bytes())
}
