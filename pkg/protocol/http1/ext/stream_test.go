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
