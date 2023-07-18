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

package resp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/internal/bytestr"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/ext"
	"github.com/cloudwego/netpoll"
)

var errBodyTooLarge = errs.New(errs.ErrBodyTooLarge, errs.ErrorTypePublic, "test")

type ErroneousBodyStream struct {
	errOnRead  bool
	errOnClose bool
}

type testReader struct {
	read chan (int)
	cb   chan (struct{})
}

func (r *testReader) Read(b []byte) (int, error) {
	read := <-r.read

	if read == -1 {
		return 0, io.EOF
	}

	r.cb <- struct{}{}

	total := len(b)
	if total > read {
		total = read
	}

	for i := 0; i < total; i++ {
		b[i] = 'x'
	}

	return total, nil
}

func (ebs *ErroneousBodyStream) Read(p []byte) (n int, err error) {
	if ebs.errOnRead {
		panic("reading erroneous body stream")
	}
	return 0, io.EOF
}

func (ebs *ErroneousBodyStream) Close() error {
	if ebs.errOnClose {
		panic("closing erroneous body stream")
	}
	return nil
}

func TestResponseBodyStreamErrorOnPanicDuringClose(t *testing.T) {
	t.Parallel()
	var resp protocol.Response
	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)

	ebs := &ErroneousBodyStream{errOnRead: false, errOnClose: true}
	resp.SetBodyStream(ebs, 42)
	err := Write(&resp, zw)
	if err == nil {
		t.Fatalf("expected error when writing response.")
	}
	e, ok := err.(*ErrBodyStreamWritePanic)
	if !ok {
		t.Fatalf("expected error struct to be *ErrBodyStreamWritePanic, got: %+v.", e)
	}
	if e.Error() != "panic while writing body stream: closing erroneous body stream" {
		t.Fatalf("unexpected error value, got: %+v.", e.Error())
	}
}

func TestResponseBodyStreamErrorOnPanicDuringRead(t *testing.T) {
	t.Parallel()
	var resp protocol.Response
	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)

	ebs := &ErroneousBodyStream{errOnRead: true, errOnClose: false}
	resp.SetBodyStream(ebs, 42)
	err := Write(&resp, zw)
	if err == nil {
		t.Fatalf("expected error when writing response.")
	}
	e, ok := err.(*ErrBodyStreamWritePanic)
	if !ok {
		t.Fatalf("expected error struct to be *ErrBodyStreamWritePanic, got: %+v.", e)
	}
	if e.Error() != "panic while writing body stream: reading erroneous body stream" {
		t.Fatalf("unexpected error value, got: %+v.", e.Error())
	}
}

func testResponseReadError(t *testing.T, resp *protocol.Response, response string) {
	zr := mock.NewZeroCopyReader(response)
	err := Read(resp, zr)
	if err == nil {
		t.Fatalf("Expecting error for response=%q", response)
	}

	testResponseReadSuccess(t, resp, "HTTP/1.1 303 Redisred sedfs sdf\r\nContent-Type: aaa\r\nContent-Length: 5\r\n\r\nHELLOaaa",
		consts.StatusSeeOther, 5, "aaa", "HELLO", nil, consts.HTTP11)
}

func testResponseReadSuccess(t *testing.T, resp *protocol.Response, response string, expectedStatusCode, expectedContentLength int,
	expectedContentType, expectedBody string, expectedTrailer map[string]string, expectedProtocol string,
) {
	zr := mock.NewZeroCopyReader(response)
	err := Read(resp, zr)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	verifyResponseHeader(t, &resp.Header, expectedStatusCode, expectedContentLength, expectedContentType, "", expectedProtocol)
	if !bytes.Equal(resp.Body(), []byte(expectedBody)) {
		t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), []byte(expectedBody))
	}
	verifyResponseTrailer(t, &resp.Header, expectedTrailer)
}

func TestResponseReadSuccess(t *testing.T) {
	t.Parallel()

	resp := &protocol.Response{}

	// usual response
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Length: 10\r\nContent-Type: foo/bar\r\n\r\n0123456789",
		consts.StatusOK, 10, "foo/bar", "0123456789", nil, consts.HTTP11)

	// zero response
	testResponseReadSuccess(t, resp, "HTTP/1.1 500 OK\r\nContent-Length: 0\r\nContent-Type: foo/bar\r\n\r\n",
		consts.StatusInternalServerError, 0, "foo/bar", "", nil, consts.HTTP11)

	// response with trailer
	testResponseReadSuccess(t, resp, "HTTP/1.1 300 OK\r\nTransfer-Encoding: chunked\r\nTrailer: foo\r\nContent-Type: bar\r\n\r\n5\r\n56789\r\n0\r\nfoo: bar\r\n\r\n",
		consts.StatusMultipleChoices, 5, "bar", "56789", map[string]string{"Foo": "bar"}, consts.HTTP11)

	// response with trailer disableNormalizing
	resp.Header.DisableNormalizing()
	resp.Header.Trailer().DisableNormalizing()
	testResponseReadSuccess(t, resp, "HTTP/1.1 300 OK\r\nTransfer-Encoding: chunked\r\nTrailer: foo\r\nContent-Type: bar\r\n\r\n5\r\n56789\r\n0\r\nfoo: bar\r\n\r\n",
		consts.StatusMultipleChoices, 5, "bar", "56789", map[string]string{"foo": "bar"}, consts.HTTP11)

	// no content-length ('identity' transfer-encoding)
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: foobar\r\n\r\nzxxxx",
		consts.StatusOK, 5, "foobar", "zxxxx", nil, consts.HTTP11)

	// explicitly stated 'Transfer-Encoding: identity'
	testResponseReadSuccess(t, resp, "HTTP/1.1 234 ss\r\nContent-Type: xxx\r\n\r\nxag",
		234, 3, "xxx", "xag", nil, consts.HTTP11)

	// big 'identity' response
	body := string(mock.CreateFixedBody(100500))
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\n\r\n"+body,
		consts.StatusOK, 100500, "aa", body, nil, consts.HTTP11)

	// chunked response
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTrailer: Foo2\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nqwer\r\n2\r\nty\r\n0\r\nFoo2: bar2\r\n\r\n",
		200, 6, "text/html", "qwerty", map[string]string{"Foo2": "bar2"}, consts.HTTP11)

	// chunked response with non-chunked Transfer-Encoding.
	testResponseReadSuccess(t, resp, "HTTP/1.1 230 OK\r\nContent-Type: text\r\nTrailer: Foo3\r\nTransfer-Encoding: aaabbb\r\n\r\n2\r\ner\r\n2\r\nty\r\n0\r\nFoo3: bar3\r\n\r\n",
		230, 4, "text", "erty", map[string]string{"Foo3": "bar3"}, consts.HTTP11)

	// chunked response with empty body
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTrailer: Foo5\r\nTransfer-Encoding: chunked\r\n\r\n0\r\nFoo5: bar5\r\n\r\n",
		consts.StatusOK, 0, "text/html", "", map[string]string{"Foo5": "bar5"}, consts.HTTP11)
}

func TestResponseReadError(t *testing.T) {
	t.Parallel()

	resp := &protocol.Response{}

	// empty response
	testResponseReadError(t, resp, "")

	// invalid header
	testResponseReadError(t, resp, "foobar")

	// empty body
	testResponseReadError(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: aaa\r\nContent-Length: 1234\r\n\r\n")

	// short body
	testResponseReadError(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: aaa\r\nContent-Length: 1234\r\n\r\nshort")
}

func TestResponseImmediateHeaderFlushChunked(t *testing.T) {
	t.Parallel()

	var r protocol.Response

	r.ImmediateHeaderFlush = true

	ch := make(chan int)
	cb := make(chan struct{})

	buf := &testReader{read: ch, cb: cb}

	r.SetBodyStream(buf, -1)

	w := bytes.NewBuffer([]byte{})
	zw := netpoll.NewWriter(w)

	waitForIt := make(chan struct{})

	go func() {
		if err := Write(&r, zw); err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		waitForIt <- struct{}{}
	}()

	ch <- 3

	if !strings.Contains(w.String(), "Transfer-Encoding: chunked") {
		t.Fatalf("Expected headers to be flushed")
	}

	if strings.Contains(w.String(), "xxx") {
		t.Fatalf("Did not expect body to be written yet")
	}

	<-cb
	ch <- -1

	<-waitForIt
}

func TestResponseImmediateHeaderFlushFixedLength(t *testing.T) {
	t.Parallel()

	var r protocol.Response

	r.ImmediateHeaderFlush = true

	ch := make(chan int)
	cb := make(chan struct{})

	buf := &testReader{read: ch, cb: cb}

	r.SetBodyStream(buf, 3)

	w := bytes.NewBuffer([]byte{})
	zw := netpoll.NewWriter(w)

	waitForIt := make(chan struct{})

	go func() {
		if err := Write(&r, zw); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		waitForIt <- struct{}{}
	}()

	// reader have more data than bodySize, but only the bodySize length of data will be send.
	ch <- 10

	if !strings.Contains(w.String(), "Content-Length: 3") {
		t.Fatalf("Expected headers to be flushed")
	}

	if strings.Contains(w.String(), "xxx") {
		t.Fatalf("Did not expect body to be written yet")
	}

	<-cb
	// ch <- -1

	<-waitForIt
}

func TestResponseImmediateHeaderFlushFixedLengthWithFewerData(t *testing.T) {
	t.Parallel()

	var r protocol.Response

	r.ImmediateHeaderFlush = true

	ch := make(chan int)
	cb := make(chan struct{})

	buf := &testReader{read: ch, cb: cb}

	r.SetBodyStream(buf, 3)

	w := bytes.NewBuffer([]byte{})
	zw := netpoll.NewWriter(w)

	waitForIt := make(chan struct{})

	go func() {
		if err := Write(&r, zw); err != nil {
			assert.NotNil(t, err)
		}
		waitForIt <- struct{}{}
	}()

	// reader have less data than bodySize, server should raise a error in this case
	ch <- 2

	<-cb
	ch <- -1

	<-waitForIt
}

func TestResponseSuccess(t *testing.T) {
	t.Parallel()

	// 200 response
	testResponseSuccess(t, consts.StatusOK, "test/plain", "server", "foobar",
		consts.StatusOK, "test/plain", "server")

	// response with missing statusCode
	testResponseSuccess(t, 0, "text/plain", "server", "foobar",
		consts.StatusOK, "text/plain", "server")

	// response with missing server
	testResponseSuccess(t, consts.StatusInternalServerError, "aaa", "", "aaadfsd",
		consts.StatusInternalServerError, "aaa", "")

	// empty body
	testResponseSuccess(t, consts.StatusOK, "bbb", "qwer", "",
		consts.StatusOK, "bbb", "qwer")

	// missing content-type
	testResponseSuccess(t, consts.StatusOK, "", "asdfsd", "asdf",
		consts.StatusOK, string(bytestr.DefaultContentType), "asdfsd")
}

func TestResponseReadLimitBody(t *testing.T) {
	t.Parallel()

	// response with content-length
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Length: 10\r\n\r\n9876543210", 10)
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Length: 10\r\n\r\n9876543210", 100)
	testResponseReadLimitBodyError(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Length: 10\r\n\r\n9876543210", 9)
	// response with content-encoding
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Encoding: gzip\r\n\r\n9876543210", 10)
	// chunked response
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 9)
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\nFoo: bar\r\n\r\n", 9)
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 100)
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\nfoobar\r\n\r\n", 100)
	testResponseReadLimitBodyError(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 2)

	// identity response
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 400 OK\r\nContent-Type: aa\r\n\r\n123456", 6)
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 400 OK\r\nContent-Type: aa\r\n\r\n123456", 106)
	testResponseReadLimitBodyError(t, "HTTP/1.1 400 OK\r\nContent-Type: aa\r\n\r\n123456", 5)
}

func TestResponseReadWithoutBody(t *testing.T) {
	var resp protocol.Response

	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 304 Not Modified\r\nContent-Type: aa\r\nContent-Encoding: gzip\r\nContent-Length: 1235\r\n\r\n", false,
		consts.StatusNotModified, 1235, "aa", nil, "gzip", consts.HTTP11)

	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 200 Foo Bar\r\nContent-Type: aab\r\nTrailer: Foo\r\nContent-Encoding: deflate\r\nTransfer-Encoding: chunked\r\n\r\n0\r\nFoo: bar\r\n\r\nHTTP/1.2", false,
		consts.StatusOK, 0, "aab", map[string]string{"Foo": "bar"}, "deflate", consts.HTTP11)

	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 204 Foo Bar\r\nContent-Type: aab\r\nTrailer: Foo\r\nContent-Encoding: deflate\r\nTransfer-Encoding: chunked\r\n\r\n0\r\nFoo: bar\r\n\r\nHTTP/1.2", true,
		consts.StatusNoContent, -1, "aab", nil, "deflate", consts.HTTP11)

	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 123 AAA\r\nContent-Type: xxx\r\nContent-Encoding: gzip\r\nContent-Length: 3434\r\n\r\n", false,
		123, 3434, "xxx", nil, "gzip", consts.HTTP11)

	testResponseReadWithoutBody(t, &resp, "HTTP 200 OK\r\nContent-Type: text/xml\r\nContent-Encoding: deflate\r\nContent-Length: 123\r\n\r\nfoobar\r\n", true,
		consts.StatusOK, 123, "text/xml", nil, "deflate", consts.HTTP10)

	// '100 Continue' must be skipped.
	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 100 Continue\r\nFoo-bar: baz\r\n\r\nHTTP/1.1 329 aaa\r\nContent-Type: qwe\r\nContent-Encoding: gzip\r\nContent-Length: 894\r\n\r\n", true,
		329, 894, "qwe", nil, "gzip", consts.HTTP11)
}

func verifyResponseHeader(t *testing.T, h *protocol.ResponseHeader, expectedStatusCode, expectedContentLength int, expectedContentType, expectedContentEncoding, expectedProtocol string) {
	if h.StatusCode() != expectedStatusCode {
		t.Fatalf("Unexpected status code %d. Expected %d", h.StatusCode(), expectedStatusCode)
	}
	if h.ContentLength() != expectedContentLength {
		t.Fatalf("Unexpected content length %d. Expected %d", h.ContentLength(), expectedContentLength)
	}
	if string(h.ContentType()) != expectedContentType {
		t.Fatalf("Unexpected content type %q. Expected %q", h.ContentType(), expectedContentType)
	}
	if string(h.ContentEncoding()) != expectedContentEncoding {
		t.Fatalf("Unexpected content encoding %q. Expected %q", h.ContentEncoding(), expectedContentEncoding)
	}

	if h.GetProtocol() != expectedProtocol {
		t.Fatalf("Unexpected protocol %q. Expected %q", h.GetProtocol(), expectedProtocol)
	}
}

func testResponseSuccess(t *testing.T, statusCode int, contentType, serverName, body string,
	expectedStatusCode int, expectedContentType, expectedServerName string,
) {
	var resp protocol.Response
	resp.SetStatusCode(statusCode)
	resp.Header.Set("Content-Type", contentType)
	resp.Header.Set("Server", serverName)
	resp.SetBody([]byte(body))

	w := &bytes.Buffer{}
	// bw := bufio.NewWriter(w)
	zw := netpoll.NewWriter(w)
	err := Write(&resp, zw)
	if err != nil {
		t.Fatalf("Unexpected error when calling Response.Write(): %s", err)
	}

	if err = zw.Flush(); err != nil {
		t.Fatalf("Unexpected error when flushing bufio.Writer: %s", err)
	}

	var resp1 protocol.Response
	br := bufio.NewReader(w)
	zr := netpoll.NewReader(br)
	if err = Read(&resp1, zr); err != nil {
		t.Fatalf("Unexpected error when calling Response.Read(): %s", err)
	}
	if resp1.StatusCode() != expectedStatusCode {
		t.Fatalf("Unexpected status code: %d. Expected %d", resp1.StatusCode(), expectedStatusCode)
	}
	if resp1.Header.ContentLength() != len(body) {
		t.Fatalf("Unexpected content-length: %d. Expected %d", resp1.Header.ContentLength(), len(body))
	}
	if string(resp1.Header.Peek(consts.HeaderContentType)) != expectedContentType {
		t.Fatalf("Unexpected content-type: %q. Expected %q", resp1.Header.Peek(consts.HeaderContentType), expectedContentType)
	}
	if string(resp1.Header.Peek(consts.HeaderServer)) != expectedServerName {
		t.Fatalf("Unexpected server: %q. Expected %q", resp1.Header.Peek(consts.HeaderServer), expectedServerName)
	}
	if !bytes.Equal(resp1.Body(), []byte(body)) {
		t.Fatalf("Unexpected body: %q. Expected %q", resp1.Body(), body)
	}
}

func testResponseReadWithoutBody(t *testing.T, resp *protocol.Response, s string, skipBody bool,
	expectedStatusCode, expectedContentLength int, expectedContentType string, expectedTrailer map[string]string, expectedContentEncoding, expectedProtocol string,
) {
	zr := mock.NewZeroCopyReader(s)
	resp.SkipBody = skipBody
	err := Read(resp, zr)
	if err != nil {
		t.Fatalf("Unexpected error when reading response without body: %s. response=%q", err, s)
	}
	if len(resp.Body()) != 0 {
		t.Fatalf("Unexpected response body %q. Expected %q. response=%q", resp.Body(), "", s)
	}

	verifyResponseHeader(t, &resp.Header, expectedStatusCode, expectedContentLength, expectedContentType, expectedContentEncoding, expectedProtocol)
	verifyResponseTrailer(t, &resp.Header, expectedTrailer)

	// verify that ordinal response is read after null-body response
	resp.SkipBody = false
	testResponseReadSuccess(t, resp, "HTTP/1.1 300 OK\r\nContent-Length: 5\r\nContent-Type: bar\r\n\r\n56789",
		consts.StatusMultipleChoices, 5, "bar", "56789", nil, consts.HTTP11)
}

func verifyResponseTrailer(t *testing.T, h *protocol.ResponseHeader, expectedTrailers map[string]string) {
	for k, v := range expectedTrailers {
		got := h.Trailer().Peek(k)
		if !bytes.Equal(got, []byte(v)) {
			t.Fatalf("Unexpected trailer %q. Expected %q. Got %q", k, v, got)
		}
	}

	h.Trailer().VisitAll(func(key, value []byte) {
		if v := expectedTrailers[string(key)]; string(value) != v {
			t.Fatalf("Unexpected trailer %q. Expected %q. Got %q", string(key), v, string(value))
		}
	})
}

func testResponseReadLimitBodyError(t *testing.T, s string, maxBodySize int) {
	var resp protocol.Response
	zr := netpoll.NewReader(bytes.NewBufferString(s))
	err := ReadHeaderAndLimitBody(&resp, zr, maxBodySize)
	if err == nil {
		t.Fatalf("expecting error. s=%q, maxBodySize=%d", s, maxBodySize)
	}
	if !errors.Is(err, errs.ErrBodyTooLarge) {
		t.Fatalf("unexpected error: %s. Expecting %s. s=%q, maxBodySize=%d", err, errBodyTooLarge, s, maxBodySize)
	}
}

func testResponseReadLimitBodySuccess(t *testing.T, s string, maxBodySize int) {
	var resp protocol.Response
	mr := mock.NewZeroCopyReader(s)
	if err := ReadHeaderAndLimitBody(&resp, mr, maxBodySize); err != nil {
		t.Fatalf("unexpected error: %s. s=%q, maxBodySize=%d", err, s, maxBodySize)
	}
}

func TestResponseBodyStreamWithTrailer(t *testing.T) {
	t.Parallel()

	testResponseBodyStreamWithTrailer(t, nil, false)

	body := mock.CreateFixedBody(1e5)
	testResponseBodyStreamWithTrailer(t, body, false)
	testResponseBodyStreamWithTrailer(t, body, true)
}

func testResponseBodyStreamWithTrailer(t *testing.T, body []byte, disableNormalizing bool) {
	expectedTrailer := map[string]string{
		"foo": "testfoo",
		"bar": "testbar",
	}
	var resp1 protocol.Response
	if disableNormalizing {
		resp1.Header.DisableNormalizing()
	}
	resp1.SetBodyStream(bytes.NewReader(body), -1)
	for k, v := range expectedTrailer {
		err := resp1.Header.Trailer().Add(k, v)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	if err := Write(&resp1, zw); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var resp2 protocol.Response
	if disableNormalizing {
		resp2.Header.DisableNormalizing()
	}
	br := bufio.NewReader(&w)
	zr := netpoll.NewReader(br)
	if err := Read(&resp2, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	respBody := resp2.Body()
	if !bytes.Equal(respBody, body) {
		t.Fatalf("unexpected body: %q. Expecting %q", respBody, body)
	}

	for k, v := range expectedTrailer {
		kBytes := []byte(k)
		utils.NormalizeHeaderKey(kBytes, disableNormalizing)
		r := resp2.Header.Trailer().Peek(k)
		if string(r) != v {
			t.Fatalf("unexpected trailer header %q: %q. Expecting %s", kBytes, r, v)
		}
	}
}

func TestResponseReadBodyStreamBadReader(t *testing.T) {
	t.Parallel()

	resp := protocol.AcquireResponse()

	errReader := mock.NewErrorReadConn(errors.New("test error"))

	bodyBuf := resp.BodyBuffer()
	bodyBuf.Reset()

	bodyStream := ext.AcquireBodyStream(bodyBuf, errReader, resp.Header.Trailer(), 100)
	resp.ConstructBodyStream(bodyBuf, convertClientRespStream(bodyStream, func(shouldClose bool) error {
		assert.True(t, shouldClose)
		return nil
	}))

	stBody := resp.BodyStream()
	closer, _ := stBody.(io.Closer)
	closer.Close()
}

func TestSetResponseBodyStreamFixedSize(t *testing.T) {
	t.Parallel()

	testSetResponseBodyStream(t, "a")
	testSetResponseBodyStream(t, string(mock.CreateFixedBody(4097)))
	testSetResponseBodyStream(t, string(mock.CreateFixedBody(100500)))
}

func TestSetResponseBodyStreamChunked(t *testing.T) {
	t.Parallel()

	testSetResponseBodyStreamChunked(t, "", map[string]string{"Foo": "bar"})

	body := "foobar baz aaa bbb ccc"
	testSetResponseBodyStreamChunked(t, body, nil)

	body = string(mock.CreateFixedBody(10001))
	testSetResponseBodyStreamChunked(t, body, map[string]string{"Foo": "test", "Bar": "test"})
}

func testSetResponseBodyStream(t *testing.T, body string) {
	var resp protocol.Response
	bodySize := len(body)
	if resp.IsBodyStream() {
		t.Fatalf("IsBodyStream must return false")
	}
	resp.SetBodyStream(bytes.NewBufferString(body), bodySize)
	if !resp.IsBodyStream() {
		t.Fatalf("IsBodyStream must return true")
	}

	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	if err := Write(&resp, zw); err != nil {
		t.Fatalf("unexpected error when writing response: %s. body=%q", err, body)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error when flushing response: %s. body=%q", err, body)
	}

	var resp1 protocol.Response
	br := bufio.NewReader(&w)
	zr := netpoll.NewReader(br)
	if err := Read(&resp1, zr); err != nil {
		t.Fatalf("unexpected error when reading response: %s. body=%q", err, body)
	}
	if string(resp1.Body()) != body {
		t.Fatalf("unexpected body %q. Expecting %q", resp1.Body(), body)
	}
}

func testSetResponseBodyStreamChunked(t *testing.T, body string, trailer map[string]string) {
	var resp protocol.Response
	if resp.IsBodyStream() {
		t.Fatalf("IsBodyStream must return false")
	}
	resp.SetBodyStream(bytes.NewBufferString(body), -1)
	if !resp.IsBodyStream() {
		t.Fatalf("IsBodyStream must return true")
	}

	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	for k, v := range trailer {
		err := resp.Header.Trailer().Add(k, v)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}
	if err := Write(&resp, zw); err != nil {
		t.Fatalf("unexpected error when writing response: %s. body=%q", err, body)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error when flushing response: %s. body=%q", err, body)
	}
	var resp1 protocol.Response
	br := bufio.NewReader(&w)
	zr := netpoll.NewReader(br)
	if err := Read(&resp1, zr); err != nil {
		t.Fatalf("unexpected error when reading response: %s. body=%q", err, body)
	}
	if string(resp1.Body()) != body {
		t.Fatalf("unexpected body %q. Expecting %q", resp1.Body(), body)
	}
	for k, v := range trailer {
		r := resp.Header.Trailer().Peek(k)
		if string(r) != v {
			t.Fatalf("unexpected trailer %s. Expecting %s. Got %q", k, v, r)
		}
	}
}

func testResponseReadBodyStreamSuccess(t *testing.T, resp *protocol.Response, response string, expectedStatusCode, expectedContentLength int,
	expectedContentType, expectedBody string, expectedTrailer map[string]string, expectedProtocol string,
) {
	zr := mock.NewZeroCopyReader(response)
	err := ReadBodyStream(resp, zr, 0, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	assert.True(t, resp.IsBodyStream())

	body, err := ioutil.ReadAll(resp.BodyStream())
	if err != nil && err != io.EOF {
		t.Fatalf("Unexpected error: %s", err)
	}
	verifyResponseHeader(t, &resp.Header, expectedStatusCode, expectedContentLength, expectedContentType, "", expectedProtocol)
	if !bytes.Equal(body, []byte(expectedBody)) {
		t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), []byte(expectedBody))
	}
	verifyResponseTrailer(t, &resp.Header, expectedTrailer)
}

func testResponseReadBodyStreamBadTrailer(t *testing.T, resp *protocol.Response, response string) {
	zr := mock.NewZeroCopyReader(response)
	err := ReadBodyStream(resp, zr, 0, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	assert.True(t, resp.IsBodyStream())

	_, err = ioutil.ReadAll(resp.BodyStream())
	if err == nil || err == io.EOF {
		t.Fatalf("expected error when reading response.")
	}
}

func TestResponseReadBodyStream(t *testing.T) {
	t.Parallel()

	resp := &protocol.Response{}

	// usual response
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Length: 10\r\nContent-Type: foo/bar\r\n\r\n0123456789",
		consts.StatusOK, 10, "foo/bar", "0123456789", nil, consts.HTTP11)

	// zero response
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 500 OK\r\nContent-Length: 0\r\nContent-Type: foo/bar\r\n\r\n",
		consts.StatusInternalServerError, 0, "foo/bar", "", nil, consts.HTTP11)

	// response with trailer
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 300 OK\r\nTransfer-Encoding: chunked\r\nTrailer: Foo\r\nContent-Type: bar\r\n\r\n5\r\n56789\r\n0\r\nfoo: bar\r\n\r\n",
		consts.StatusMultipleChoices, -1, "bar", "56789", map[string]string{"Foo": "bar"}, consts.HTTP11)

	// response with trailer disableNormalizing
	resp.Header.DisableNormalizing()
	resp.Header.Trailer().DisableNormalizing()
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 300 OK\r\nTransfer-Encoding: chunked\r\nTrailer: foo\r\nContent-Type: bar\r\n\r\n5\r\n56789\r\n0\r\nfoo: bar\r\n\r\n",
		consts.StatusMultipleChoices, -1, "bar", "56789", map[string]string{"foo": "bar"}, consts.HTTP11)

	// no content-length ('identity' transfer-encoding)
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: foobar\r\n\r\nzxxxx",
		consts.StatusOK, -2, "foobar", "zxxxx", nil, consts.HTTP11)

	// explicitly stated 'Transfer-Encoding: identity'
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 234 ss\r\nContent-Type: xxx\r\n\r\nxag",
		234, -2, "xxx", "xag", nil, consts.HTTP11)

	// big 'identity' response
	body := string(mock.CreateFixedBody(100500))
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\n\r\n"+body,
		consts.StatusOK, -2, "aa", body, nil, consts.HTTP11)

	// chunked response
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTrailer: Foo2\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nqwer\r\n2\r\nty\r\n0\r\nFoo2: bar2\r\n\r\n",
		200, -1, "text/html", "qwerty", map[string]string{"Foo2": "bar2"}, consts.HTTP11)

	// chunked response with non-chunked Transfer-Encoding.
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 230 OK\r\nContent-Type: text\r\nTrailer: Foo3\r\nTransfer-Encoding: aaabbb\r\n\r\n2\r\ner\r\n2\r\nty\r\n0\r\nFoo3: bar3\r\n\r\n",
		230, -1, "text", "erty", map[string]string{"Foo3": "bar3"}, consts.HTTP11)

	// chunked response with empty body
	testResponseReadBodyStreamSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTrailer: Foo5\r\nTransfer-Encoding: chunked\r\n\r\n0\r\nFoo5: bar5\r\n\r\n",
		consts.StatusOK, -1, "text/html", "", map[string]string{"Foo5": "bar5"}, consts.HTTP11)
}

func TestResponseReadBodyStreamBadTrailer(t *testing.T) {
	t.Parallel()

	resp := &protocol.Response{}

	testResponseReadBodyStreamBadTrailer(t, resp, "HTTP/1.1 300 OK\r\nTransfer-Encoding: chunked\r\nContent-Type: bar\r\n\r\n5\r\n56789\r\n0\r\ncontent-type: bar\r\n\r\n")
	testResponseReadBodyStreamBadTrailer(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nqwer\r\n2\r\nty\r\n0\r\nproxy-connection: bar2\r\n\r\n")
}

func TestResponseString(t *testing.T) {
	resp := protocol.Response{}
	resp.Header.Set("Location", "foo\r\nSet-Cookie: SESSIONID=MaliciousValue\r\n")
	assert.True(t, strings.Contains(GetHTTP1Response(&resp).String(), "Location: foo\r\nSet-Cookie: SESSIONID=MaliciousValue\r\n"))
}
