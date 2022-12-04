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
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/compress"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/valyala/bytebufferpool"
)

func TestResponseCopyTo(t *testing.T) {
	t.Parallel()

	var resp Response

	// empty copy
	testResponseCopyTo(t, &resp)

	// init resp
	// resp.laddr = zeroTCPAddr
	resp.SkipBody = true
	resp.Header.SetStatusCode(200)
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

func testResponseCopyTo(t *testing.T, src *Response) {
	var dst Response
	src.CopyTo(&dst)

	if !reflect.DeepEqual(src, &dst) { //nolint:govet
		t.Fatalf("ResponseCopyTo fail, src: \n%+v\ndst: \n%+v\n", src, &dst) //nolint:govet
	}
}

func TestResponseMustSkipBody(t *testing.T) {
	resp := Response{}
	resp.SetStatusCode(200)
	resp.SetBodyString("test")
	assert.False(t, resp.MustSkipBody())
	// no content 204 means that skip body is necessary
	resp.SetStatusCode(204)
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
	resp1 := AcquireResponse()
	assert.NotNil(t, resp1)
	resp1.SetBody([]byte("test"))
	resp1.SetStatusCode(200)
	ReleaseResponse(resp1)
	assert.Nil(t, resp1.body)
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
