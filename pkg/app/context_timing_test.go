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
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
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

// go test -v -run=^$ -bench=BenchmarkCtxBody -benchmem -count=4
func BenchmarkCtxBody(b *testing.B) {
	ctx := NewContext(0)
	defer ctx.Reset()
	data := []byte("hello world")
	ctx.Request.SetBodyRaw(data)
	for n := 0; n < b.N; n++ {
		_ = ctx.Request.Body()
	}
	assert.Equal(b, data, ctx.Request.Body())
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
			assert.Equal(b, nil, err)

			c.Request.SetBody(compressedBody)
			for i := 0; i < b.N; i++ {
				_ = c.Request.Body()

			}
		})
	}
}

func BenchmarkBindJSON(b *testing.B) {
	c := NewContext(0)
	defer c.Reset()
	type Demo struct {
		Name string `json:"name"`
	}
	body := []byte(`{"name":"john"}`)
	c.Request.SetBody(body)
	c.Request.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationJSON))
	c.Request.Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = c.BindJSON(d) //nolint:errcheck // It is fine to ignore the error here as we check it once further below
	}
	assert.Equal(b, nil, c.BindJSON(d))
	assert.Equal(b, "john", d.Name)
}

func BenchmarkBindQuery(b *testing.B) {
	c := NewContext(0)
	defer c.Reset()
	type Demo struct {
		Name string `query:"name"`
	}
	body := []byte(`{"name":"john"}`)
	c.Request.SetBody(body)
	c.Request.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationJSON))
	c.Request.Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = c.BindQuery(d) //nolint:errcheck // It is fine to ignore the error here as we check it once further below
	}
	assert.Equal(b, nil, c.BindQuery(d))
	assert.Equal(b, "john", d.Name)
}

func BenchmarkBindForm(b *testing.B) {
	c := NewContext(0)
	defer c.Reset()
	type Demo struct {
		Name string `form:"name"`
	}
	body := []byte("name=john")
	c.Request.SetBody(body)
	c.Request.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationHTMLForm))
	c.Request.Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = c.BindForm(d) //nolint:errcheck // It is fine to ignore the error here as we check it once further below
	}
	assert.Equal(b, nil, c.BindForm(d))
	assert.Equal(b, "john", d.Name)
}
