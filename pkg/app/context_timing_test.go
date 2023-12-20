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

package app

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func BenchmarkNewContext(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := NewContext(0)
		c.Reset()
	}
}

// go test -v -run=^$ -bench=BenchmarkCtxJSON -benchmem -count=4
func BenchmarkCtxJSON(b *testing.B) {
	ctx := NewContext(0)
	defer ctx.Reset()
	type SomeStruct struct {
		Name string
		Age  uint8
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ctx.JSON(200, &data)
	}
}

func BenchmarkCtxString(b *testing.B) {
	c := NewContext(0)
	defer c.Reset()
	s := "Hello, World!"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.String(200, s)
	}
}

// go test -v -run=^$ -bench=BenchmarkCtxBody -benchmem -count=4
func BenchmarkCtxBody(b *testing.B) {
	ctx := NewContext(0)
	defer ctx.Reset()
	data := []byte("hello world")
	ctx.Request.SetBodyRaw(data)
	for n := 0; n < b.N; n++ {
		_ = ctx.Request.Body()
	}
	assert.DeepEqual(b, data, ctx.Request.Body())
}

// go test -v -run=^$ -bench=BenchmarkCtxBodyWithCompression -benchmem -count=4
func BenchmarkCtxBodyWithCompression(b *testing.B) {
	encodingErr := errors.New("failed to encoding data")
	var (
		compressGzip = func(data []byte) ([]byte, error) {
			var buf bytes.Buffer
			writer := gzip.NewWriter(&buf)
			if _, err := writer.Write(data); err != nil {
				return nil, encodingErr
			}
			if err := writer.Flush(); err != nil {
				return nil, encodingErr
			}
			if err := writer.Close(); err != nil {
				return nil, encodingErr
			}
			return buf.Bytes(), nil
		}
		compressDeflate = func(data []byte) ([]byte, error) {
			var buf bytes.Buffer
			writer := zlib.NewWriter(&buf)
			if _, err := writer.Write(data); err != nil {
				return nil, encodingErr
			}
			if err := writer.Flush(); err != nil {
				return nil, encodingErr
			}
			if err := writer.Close(); err != nil {
				return nil, encodingErr
			}
			return buf.Bytes(), nil
		}
	)
	compressionTests := []struct {
		contentEncoding string
		compressWriter  func([]byte) ([]byte, error)
	}{
		{
			contentEncoding: "gzip",
			compressWriter:  compressGzip,
		},
		{
			contentEncoding: "gzip,invalid",
			compressWriter:  compressGzip,
		},
		{
			contentEncoding: "deflate",
			compressWriter:  compressDeflate,
		},
		{
			contentEncoding: "gzip,deflate",
			compressWriter: func(data []byte) ([]byte, error) {
				var (
					buf    bytes.Buffer
					writer interface {
						io.WriteCloser
						Flush() error
					}
					err error
				)
				// deflate
				{
					writer = zlib.NewWriter(&buf)
					if _, err = writer.Write(data); err != nil {
						return nil, encodingErr
					}
					if err = writer.Flush(); err != nil {
						return nil, encodingErr
					}
					if err = writer.Close(); err != nil {
						return nil, encodingErr
					}
				}

				data = make([]byte, buf.Len())
				copy(data, buf.Bytes())
				buf.Reset()

				// gzip
				{
					writer = gzip.NewWriter(&buf)
					if _, err = writer.Write(data); err != nil {
						return nil, encodingErr
					}
					if err = writer.Flush(); err != nil {
						return nil, encodingErr
					}
					if err = writer.Close(); err != nil {
						return nil, encodingErr
					}
				}

				return buf.Bytes(), nil
			},
		},
	}

	for _, ct := range compressionTests {
		b.Run(ct.contentEncoding, func(b *testing.B) {
			c := NewContext(0)
			defer c.Reset()
			const input = "john=doe"

			c.Request.Header.Set("Content-Encoding", ct.contentEncoding)
			compressedBody, err := ct.compressWriter([]byte(input))
			assert.DeepEqual(b, nil, err)

			c.Request.SetBody(compressedBody)
			for i := 0; i < b.N; i++ {
				_ = c.Request.Body()
			}
		})
	}
}

func BenchmarkCtxWrite(b *testing.B) {
	c := NewContext(0)
	byt := []byte("Hello, World!")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Write(byt)
	}
}

func BenchmarkCtxWriteString(b *testing.B) {
	c := NewContext(0)
	defer c.Reset()
	s := "Hello, World!"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.WriteString(s)
	}
}
