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
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik Dubbelboer, FastHTTP Authors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/compress"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestSetMaxPooledBodySize(t *testing.T) {
	// Do not run in parallel: these pools are shared by all protocol tests.
	SetMaxPooledBodySize(512 << 10)
	defer SetMaxPooledBodySize(0)

	for _, size := range []int{1, 4 << 20} {
		reqBody := &bytebufferpool.ByteBuffer{B: make([]byte, size, 4<<20)}
		respBody := &bytebufferpool.ByteBuffer{B: make([]byte, size, 4<<20)}
		req := Request{body: reqBody}
		resp := Response{body: respBody}
		req.ResetBody()
		resp.ResetBody()
		if req.body != nil || resp.body != nil {
			t.Fatal("request/response retained body after reset")
		}
		// Rejected buffers are not reset by Pool.Put, even if their length shrank.
		if len(reqBody.B) != size || len(respBody.B) != size {
			t.Fatal("oversized body accepted by pool")
		}
	}

	// Object-level retention remains controlled by MaxKeepBodySize.
	resp := Response{body: &bytebufferpool.ByteBuffer{B: make([]byte, 1, 4<<20)}}
	resp.SetMaxKeepBodySize(4 << 20)
	resp.ResetBody()
	if resp.body == nil || cap(resp.body.B) != 4<<20 {
		t.Fatal("pool limit unexpectedly changed object-level retention")
	}
}

func TestSetMaxPooledBodySizeIndependent(t *testing.T) {
	// Do not run in parallel: these pools are shared by all protocol tests.
	defer SetMaxPooledBodySize(0)
	for _, tt := range []struct {
		name         string
		configure    func()
		keepRequest  bool
		keepResponse bool
	}{
		{"request", func() { SetMaxPooledRequestBodySize(64) }, true, false},
		{"response", func() { SetMaxPooledResponseBodySize(64) }, false, true},
		{"disable_request", func() { SetMaxPooledRequestBodySize(0) }, true, false},
		{"disable_response", func() { SetMaxPooledResponseBodySize(0) }, false, true},
		{"both_override", func() {
			SetMaxPooledRequestBodySize(16)
			SetMaxPooledResponseBodySize(8)
			SetMaxPooledBodySize(64)
		}, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			SetMaxPooledBodySize(32)
			tt.configure()
			// 64 bytes is the smallest calibration bucket, so the automatic
			// limit cannot reject these buffers and mask the configured limits.
			reqBody := &bytebufferpool.ByteBuffer{B: make([]byte, 1, 64)}
			respBody := &bytebufferpool.ByteBuffer{B: make([]byte, 1, 64)}
			req := Request{body: reqBody}
			resp := Response{body: respBody}
			req.ResetBody()
			resp.ResetBody()
			// Pool.Put resets accepted buffers. Inspect that effect rather than
			// requiring sync.Pool to retain them, which it does not guarantee.
			if kept := len(reqBody.B) == 0; kept != tt.keepRequest {
				t.Fatalf("request buffer accepted = %v, want %v", kept, tt.keepRequest)
			}
			if kept := len(respBody.B) == 0; kept != tt.keepResponse {
				t.Fatalf("response buffer accepted = %v, want %v", kept, tt.keepResponse)
			}
		})
	}
}

func TestResponseCopyTo(t *testing.T) {
	t.Parallel()

	var resp Response

	// empty copy
	testResponseCopyTo(t, &resp)

	// init resp
	// resp.laddr = zeroTCPAddr
	resp.SkipBody = true
	resp.Header.SetStatusCode(consts.StatusOK)
	resp.SetBodyString("test")
	testResponseCopyTo(t, &resp)
}

func TestResponseBodyStreamMultipleBodyCalls(t *testing.T) {
	t.Parallel()

	var r Response

	s := "foobar baz abc"
	if r.IsBodyStream() {
		t.Fatalf("IsBodyStream must return false")
	}
	r.SetBodyStream(bytes.NewBufferString(s), len(s))
	if !r.IsBodyStream() {
		t.Fatalf("IsBodyStream must return true")
	}
	for i := 0; i < 10; i++ {
		body := r.Body()
		if string(body) != s {
			t.Fatalf("unexpected body %q. Expecting %q. iteration %d", body, s, i)
		}
	}
}

func TestResponseBodyWriteToPlain(t *testing.T) {
	t.Parallel()

	var r Response

	expectedS := "foobarbaz"
	r.AppendBodyString(expectedS)

	testBodyWriteTo(t, &r, expectedS, true)
}

func TestResponseBodyWriteToStream(t *testing.T) {
	t.Parallel()

	var r Response

	expectedS := "aaabbbccc"
	buf := bytes.NewBufferString(expectedS)
	if r.IsBodyStream() {
		t.Fatalf("IsBodyStream must return false")
	}
	r.SetBodyStream(buf, len(expectedS))
	if !r.IsBodyStream() {
		t.Fatalf("IsBodyStream must return true")
	}

	testBodyWriteTo(t, &r, expectedS, false)
}

func TestResponseBodyWriter(t *testing.T) {
	t.Parallel()

	var r Response
	w := r.BodyWriter()
	for i := 0; i < 10; i++ {
		fmt.Fprintf(w, "%d", i)
	}
	if string(r.Body()) != "0123456789" {
		t.Fatalf("unexpected body %q. Expecting %q", r.Body(), "0123456789")
	}
}

func TestResponseRawBodySet(t *testing.T) {
	t.Parallel()

	var resp Response

	expectedS := "test"
	body := []byte(expectedS)
	resp.SetBodyRaw(body)

	testBodyWriteTo(t, &resp, expectedS, true)
}

func TestResponseRawBodyReset(t *testing.T) {
	t.Parallel()

	var resp Response

	body := []byte("test")
	resp.SetBodyRaw(body)
	resp.ResetBody()

	testBodyWriteTo(t, &resp, "", true)
}

func TestResponseResetBody(t *testing.T) {
	resp := Response{}
	resp.BodyBuffer()
	assert.NotNil(t, resp.body)
	resp.maxKeepBodySize = math.MaxUint32
	resp.ResetBody()
	assert.NotNil(t, resp.body)
	resp.maxKeepBodySize = -1
	resp.ResetBody()
	assert.Nil(t, resp.body)
}

func TestResponseBodyReuse(t *testing.T) {
	resp := Response{}
	resp.maxKeepBodySize = 1024

	buf := resp.BodyBuffer()
	// set a big body
	buf.Write(make([]byte, resp.maxKeepBodySize+1))
	resp.ResetBody()
	assert.Nil(t, resp.body)
	// NOTICE: bytebufferpool may not get a big enough buffer,
	// so we just mock a new one here
	resp.body = &bytebufferpool.ByteBuffer{
		B: make([]byte, 0, resp.maxKeepBodySize+1),
	}
	// set a small body
	buf.Write(make([]byte, 1))
	resp.ResetBody()
	assert.Nil(t, resp.body)
}

func testResponseCopyTo(t *testing.T, src *Response) {
	var dst Response
	src.CopyTo(&dst)

	if !reflect.DeepEqual(src, &dst) { //nolint:govet
		t.Fatalf("ResponseCopyTo fail, src: \n%+v\ndst: \n%+v\n", src, &dst) //nolint:govet
	}
}

func TestResponseMustSkipBody(t *testing.T) {
	resp := Response{}
	resp.SetStatusCode(consts.StatusOK)
	resp.SetBodyString("test")
	assert.False(t, resp.MustSkipBody())
	// no content 204 means that skip body is necessary
	resp.SetStatusCode(consts.StatusNoContent)
	resp.ResetBody()
	assert.True(t, resp.MustSkipBody())
}

func TestResponseBodyGunzip(t *testing.T) {
	t.Parallel()
	dst1 := []byte("")
	src1 := []byte("hello")
	res1 := compress.AppendGzipBytes(dst1, src1)
	resp := Response{}
	resp.SetBody(res1)
	zipData, err := resp.BodyGunzip()
	assert.Nil(t, err)
	assert.DeepEqual(t, zipData, src1)
}

func TestResponseSwapResponseBody(t *testing.T) {
	t.Parallel()
	resp1 := Response{}
	str1 := "resp1"
	byteBuffer1 := &bytebufferpool.ByteBuffer{}
	byteBuffer1.Set([]byte(str1))
	resp1.ConstructBodyStream(byteBuffer1, bytes.NewBufferString(str1))
	assert.True(t, resp1.HasBodyBytes())
	resp2 := Response{}
	str2 := "resp2"
	byteBuffer2 := &bytebufferpool.ByteBuffer{}
	byteBuffer2.Set([]byte(str2))
	resp2.ConstructBodyStream(byteBuffer2, bytes.NewBufferString(str2))
	SwapResponseBody(&resp1, &resp2)
	assert.DeepEqual(t, resp1.body.B, []byte(str2))
	assert.DeepEqual(t, resp1.BodyStream(), bytes.NewBufferString(str2))
	assert.DeepEqual(t, resp2.body.B, []byte(str1))
	assert.DeepEqual(t, resp2.BodyStream(), bytes.NewBufferString(str1))
}

func TestResponseAcquireResponse(t *testing.T) {
	t.Parallel()
	for i := 0; i < 10; i++ {
		resp1 := AcquireResponse()
		assert.NotNil(t, resp1)
		assert.Nil(t, resp1.body)
		assert.Assert(t, resp1.BodyStream() == NoResponseBody)
		assert.Assert(t, resp1.IsBodyStream() == false)

		resp1.SetBody([]byte("test"))
		resp1.SetStatusCode(consts.StatusOK)
		ReleaseResponse(resp1)
	}
}

type closeBuffer struct {
	*bytes.Buffer
}

func (b *closeBuffer) Close() error {
	b.Reset()
	return nil
}

func TestSetBodyStreamNoReset(t *testing.T) {
	t.Parallel()
	resp := Response{}
	bsA := &closeBuffer{bytes.NewBufferString("A")}
	bsB := &closeBuffer{bytes.NewBufferString("B")}
	bsC := &closeBuffer{bytes.NewBufferString("C")}

	resp.SetBodyStream(bsA, 1)
	resp.SetBodyStreamNoReset(bsB, 1)
	// resp.Body() has closed bsB
	assert.DeepEqual(t, string(resp.Body()), "B")
	assert.DeepEqual(t, bsA.String(), "A")

	resp.bodyStream = bsA
	resp.SetBodyStream(bsC, 1)
	assert.DeepEqual(t, bsA.String(), "")
}

func TestRespSafeCopy(t *testing.T) {
	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	resp.bodyRaw = make([]byte, 1)
	resps := make([]*Response, 10)
	for i := 0; i < 10; i++ {
		resp.bodyRaw[0] = byte(i)
		tmpResq := AcquireResponse()
		resp.CopyTo(tmpResq)
		resps[i] = tmpResq
	}
	for i := 0; i < 10; i++ {
		assert.DeepEqual(t, []byte{byte(i)}, resps[i].Body())
	}
}

func TestResponse_HijackWriter(t *testing.T) {
	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	buf := new(bytes.Buffer)
	isFinal := false
	resp.HijackWriter(&mock.ExtWriter{Buf: buf, IsFinal: &isFinal})
	resp.AppendBody([]byte("hello"))
	assert.DeepEqual(t, 0, buf.Len())
	resp.GetHijackWriter().Flush()
	assert.DeepEqual(t, "hello", buf.String())
	resp.AppendBodyString(", world")
	assert.DeepEqual(t, "hello", buf.String())
	resp.GetHijackWriter().Flush()
	assert.DeepEqual(t, "hello, world", buf.String())
	resp.SetBody([]byte("hello, hertz"))
	resp.GetHijackWriter().Flush()
	assert.DeepEqual(t, "hello, hertz", buf.String())
	assert.False(t, isFinal)
	resp.GetHijackWriter().Finalize()
	assert.True(t, isFinal)
}

type HijackerFunc func() (network.Conn, error)

func (h HijackerFunc) Read(_ []byte) (int, error)    { return 0, errors.New("not implemented") }
func (h HijackerFunc) Hijack() (network.Conn, error) { return h() }

func TestResponse_Hijack(t *testing.T) {
	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	_, err := resp.Hijack()
	assert.NotNil(t, err)

	resp.SetBodyStream(HijackerFunc(func() (network.Conn, error) { return nil, nil }), -1)
	_, err = resp.Hijack()
	assert.Nil(t, err)
}
