/*
 * Copyright 2023 CloudWeGo Authors
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

package resp

import (
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func TestNewChunkedBodyWriter(t *testing.T) {
	response := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(response)

	mockConn := mock.NewConn("")
	w := NewChunkedBodyWriter(response, mockConn)
	_, _ = w.Write([]byte("hello"))
	_, _ = w.Write(nil) // noop
	assert.Nil(t, w.Flush())
	out, _ := mockConn.WriterRecorder().Peek(mockConn.WriterRecorder().WroteLen())
	resp := string(out)
	assert.True(t, strings.Contains(resp, "Transfer-Encoding: chunked"))
	assert.True(t, strings.HasSuffix(resp, "5\r\nhello\r\n"))

	// Finalize adds 0\r\n\r\n
	assert.Nil(t, w.Finalize())
	assert.Nil(t, w.Finalize()) // noop
	out, _ = mockConn.WriterRecorder().Peek(mockConn.WriterRecorder().WroteLen())
	resp = string(out)
	assert.True(t, strings.HasSuffix(resp, "5\r\nhello\r\n0\r\n\r\n"))

	_, err := w.Write([]byte("world"))
	assert.True(t, err == errChunkedFinished)
}

func TestNewChunkedBodyWriter_Err(t *testing.T) {
	response := protocol.AcquireResponse()
	defer protocol.ReleaseResponse(response)

	mw := mock.NewMockWriter(nil)
	w := NewChunkedBodyWriter(response, mw)

	expectErr := errors.New("mock malloc err")

	mw.MockMalloc = func(n int) ([]byte, error) {
		return nil, expectErr
	}
	_, err := w.Write([]byte("hello"))
	assert.True(t, err == expectErr)

	mw.MockMalloc = nil
	_, err = w.Write([]byte("world"))
	assert.True(t, err == expectErr) // next call will return last err

	w = NewChunkedBodyWriter(response, mw)

	mw.MockMalloc = func(n int) ([]byte, error) {
		return nil, expectErr
	}
	err = w.Finalize()
	assert.True(t, err == expectErr)
}
