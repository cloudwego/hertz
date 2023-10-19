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
package ext

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func createChunkedBody(body, rest []byte, trailer map[string]string, hasTrailer bool) []byte {
	var b []byte
	chunkSize := 1
	for len(body) > 0 {
		if chunkSize > len(body) {
			chunkSize = len(body)
		}
		b = append(b, []byte(fmt.Sprintf("%x\r\n", chunkSize))...)
		b = append(b, body[:chunkSize]...)
		b = append(b, []byte("\r\n")...)
		body = body[chunkSize:]
		chunkSize++
	}
	if hasTrailer {
		b = append(b, "0\r\n"...)
		for k, v := range trailer {
			b = append(b, k...)
			b = append(b, ": "...)
			b = append(b, v...)
			b = append(b, "\r\n"...)
		}
		b = append(b, "\r\n"...)
	}
	return append(b, rest...)
}

func testChunkedSkipRest(t *testing.T, data, rest string) {
	var pool bytebufferpool.Pool
	reader := mock.NewZeroCopyReader(data)

	bs := AcquireBodyStream(pool.Get(), reader, &protocol.Trailer{}, -1)
	err := bs.(*bodyStream).skipRest()
	assert.Nil(t, err)

	rest_data, err := io.ReadAll(reader)
	assert.Nil(t, err)
	assert.DeepEqual(t, rest, string(rest_data))
}

func testChunkedSkipRestWithBodySize(t *testing.T, bodySize int) {
	body := mock.CreateFixedBody(bodySize)
	rest := mock.CreateFixedBody(bodySize)
	data := createChunkedBody(body, rest, map[string]string{"foo": "bar"}, true)

	testChunkedSkipRest(t, string(data), string(rest))
}

func TestChunkedSkipRest(t *testing.T) {
	t.Parallel()

	testChunkedSkipRest(t, "0\r\n\r\n", "")
	testChunkedSkipRest(t, "0\r\n\r\nHTTP/1.1 / POST", "HTTP/1.1 / POST")
	testChunkedSkipRest(t, "0\r\nHertz: test\r\nfoo: bar\r\n\r\nHTTP/1.1 / POST", "HTTP/1.1 / POST")

	testChunkedSkipRestWithBodySize(t, 5)

	// medium-size body
	testChunkedSkipRestWithBodySize(t, 43488)

	// big body
	testChunkedSkipRestWithBodySize(t, 3*1024*1024)
}

func TestBodyStream_Reset(t *testing.T) {
	t.Parallel()
	bs := bodyStream{
		prefetchedBytes: bytes.NewReader([]byte("aaa")),
		reader:          mock.NewZeroCopyReader("bbb"),
		trailer:         &protocol.Trailer{},
		offset:          10,
		contentLength:   20,
		chunkLeft:       50,
		chunkEOF:        true,
	}

	bs.reset()

	assert.Nil(t, bs.prefetchedBytes)
	assert.Nil(t, bs.reader)
	assert.Nil(t, bs.trailer)
	assert.DeepEqual(t, 0, bs.offset)
	assert.DeepEqual(t, 0, bs.contentLength)
	assert.DeepEqual(t, 0, bs.chunkLeft)
	assert.False(t, bs.chunkEOF)
}

func TestReadBodyWithStreaming(t *testing.T) {
	t.Run("TestBodyFixedSize", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, bodySize, -1, nil)
		assert.Nil(t, err)
		assert.DeepEqual(t, body, dst)
	})

	t.Run("TestBodyFixedSizeMaxContentLength", func(t *testing.T) {
		bodySize := 8 * 1024 * 2
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, bodySize, 8*1024*10, nil)
		assert.Nil(t, err)
		assert.DeepEqual(t, body[:maxContentLengthInStream], dst)
	})

	t.Run("TestBodyIdentity", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, -2, 512, nil)
		assert.Nil(t, err)
		assert.DeepEqual(t, body, dst)
	})

	t.Run("TestErrBodyTooLarge", func(t *testing.T) {
		bodySize := 2048
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, bodySize, 1024, nil)
		assert.True(t, errors.Is(err, errBodyTooLarge))
		assert.DeepEqual(t, body[:len(dst)], dst)
	})

	t.Run("TestErrChunkedStream", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, -1, bodySize, nil)
		assert.True(t, errors.Is(err, errChunkedStream))
		assert.Nil(t, dst)
	})
}

func TestBodyStream(t *testing.T) {
	t.Run("TestBodyStreamPrereadBuffer", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		byteBuffer := &bytebufferpool.ByteBuffer{}
		byteBuffer.Set(body)

		bs := AcquireBodyStream(byteBuffer, mock.NewSlowReadConn(""), nil, len(body))
		defer func() {
			ReleaseBodyStream(bs)
		}()

		b := make([]byte, bodySize)
		err := bodyStreamRead(bs, b)
		assert.Nil(t, err)
		assert.DeepEqual(t, len(body), len(b))
		assert.DeepEqual(t, string(body), string(b))
	})

	t.Run("TestBodyStreamRelease", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		byteBuffer := &bytebufferpool.ByteBuffer{}
		byteBuffer.Set(body)
		bs := AcquireBodyStream(byteBuffer, mock.NewSlowReadConn(string(body)), nil, bodySize*2)
		err := ReleaseBodyStream(bs)
		assert.Nil(t, err)
	})

	t.Run("TestBodyStreamChunked", func(t *testing.T) {
		bodySize := 5
		body := mock.CreateFixedBody(bodySize)
		expectedTrailer := map[string]string{"Foo": "chunked shit"}
		chunkedBody := mock.CreateChunkedBody(body, expectedTrailer, true)

		byteBuffer := &bytebufferpool.ByteBuffer{}
		byteBuffer.Set(chunkedBody)

		bs := AcquireBodyStream(byteBuffer, mock.NewSlowReadConn(string(chunkedBody)), &protocol.Trailer{}, -1)
		defer func() {
			ReleaseBodyStream(bs)
		}()

		b := make([]byte, bodySize)
		err := bodyStreamRead(bs, b)
		assert.Nil(t, err)
		assert.DeepEqual(t, len(body), len(b))
		assert.DeepEqual(t, string(body), string(b))
	})

	t.Run("TestBodyStreamReadFromWire", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		byteBuffer := &bytebufferpool.ByteBuffer{}
		byteBuffer.Set(body)

		rcBodySize := 128
		rcBody := mock.CreateFixedBody(rcBodySize)
		bs := AcquireBodyStream(byteBuffer, mock.NewSlowReadConn(string(rcBody)), nil, -2)
		defer func() {
			ReleaseBodyStream(bs)
		}()

		b := make([]byte, bodySize)
		err := bodyStreamRead(bs, b)
		assert.Nil(t, err)
		assert.DeepEqual(t, len(body), len(b))
		assert.DeepEqual(t, string(body), string(b))
	})
}

func bodyStreamRead(bs io.Reader, b []byte) (err error) {
	nb := 0
	for {
		p := make([]byte, 64)
		n, rErr := bs.Read(p)
		if n > 0 {
			copy(b[nb:], p[:])
			nb = nb + n
		}

		if rErr != nil {
			if rErr != io.EOF {
				err = rErr
			}
			break
		}
	}
	return
}
