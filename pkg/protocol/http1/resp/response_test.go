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
	"strings"
	"testing"

	"github.com/cloudwego/hertz/internal/bytestr"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
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
		303, 5, "aaa", "HELLO", "aaa")
}

func testResponseReadSuccess(t *testing.T, resp *protocol.Response, response string, expectedStatusCode, expectedContentLength int,
	expectedContentType, expectedBody, expectedTrailer string,
) {
	zr := mock.NewZeroCopyReader(response)
	err := Read(resp, zr)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	verifyResponseHeader(t, &resp.Header, expectedStatusCode, expectedContentLength, expectedContentType)
	if !bytes.Equal(resp.Body(), []byte(expectedBody)) {
		t.Fatalf("Unexpected body %q. Expected %q", resp.Body(), []byte(expectedBody))
	}
	assert.VerifyTrailer(t, zr, expectedTrailer)
}

func TestResponseReadSuccess(t *testing.T) {
	t.Parallel()

	resp := &protocol.Response{}

	// usual response
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Length: 10\r\nContent-Type: foo/bar\r\n\r\n0123456789",
		200, 10, "foo/bar", "0123456789", "")

	// zero response
	testResponseReadSuccess(t, resp, "HTTP/1.1 500 OK\r\nContent-Length: 0\r\nContent-Type: foo/bar\r\n\r\n",
		500, 0, "foo/bar", "", "")

	// response with trailer
	testResponseReadSuccess(t, resp, "HTTP/1.1 300 OK\r\nContent-Length: 5\r\nContent-Type: bar\r\n\r\n56789aaa",
		300, 5, "bar", "56789", "aaa")

	// no content-length ('identity' transfer-encoding)
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: foobar\r\n\r\nzxxc",
		200, 4, "foobar", "zxxc", "")

	// explicitly stated 'Transfer-Encoding: identity'
	testResponseReadSuccess(t, resp, "HTTP/1.1 234 ss\r\nContent-Type: xxx\r\n\r\nxag",
		234, 3, "xxx", "xag", "")

	// big 'identity' response
	body := string(mock.CreateFixedBody(100500))
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\n\r\n"+body,
		200, 100500, "aa", body, "")

	// chunked response
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nqwer\r\n2\r\nty\r\n0\r\n\r\nzzzzz",
		200, 6, "text/html", "qwerty", "zzzzz")

	// chunked response with non-chunked Transfer-Encoding.
	testResponseReadSuccess(t, resp, "HTTP/1.1 230 OK\r\nContent-Type: text\r\nTransfer-Encoding: aaabbb\r\n\r\n2\r\ner\r\n2\r\nty\r\n0\r\n\r\nwe",
		230, 4, "text", "erty", "we")

	// zero chunked response
	testResponseReadSuccess(t, resp, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\nzzz",
		200, 0, "text/html", "", "zzz")
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

	b := []byte{}
	w := bytes.NewBuffer(b)
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

	b := []byte{}
	w := bytes.NewBuffer(b)
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

	b := []byte{}
	w := bytes.NewBuffer(b)
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
	testResponseSuccess(t, 200, "test/plain", "server", "foobar",
		200, "test/plain", "server")

	// response with missing statusCode
	testResponseSuccess(t, 0, "text/plain", "server", "foobar",
		200, "text/plain", "server")

	// response with missing server
	testResponseSuccess(t, 500, "aaa", "", "aaadfsd",
		500, "aaa", "")

	// empty body
	testResponseSuccess(t, 200, "bbb", "qwer", "",
		200, "bbb", "qwer")

	// missing content-type
	testResponseSuccess(t, 200, "", "asdfsd", "asdf",
		200, string(bytestr.DefaultContentType), "asdfsd")
}

func TestResponseReadLimitBody(t *testing.T) {
	t.Parallel()

	// response with content-length
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Length: 10\r\n\r\n9876543210", 10)
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Length: 10\r\n\r\n9876543210", 100)
	testResponseReadLimitBodyError(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nContent-Length: 10\r\n\r\n9876543210", 9)

	// chunked response
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 9)
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 100)
	testResponseReadLimitBodyError(t, "HTTP/1.1 200 OK\r\nContent-Type: aa\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfoobar\r\n3\r\nbaz\r\n0\r\n\r\n", 2)
	//
	//// identity response
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 400 OK\r\nContent-Type: aa\r\n\r\n123456", 6)
	testResponseReadLimitBodySuccess(t, "HTTP/1.1 400 OK\r\nContent-Type: aa\r\n\r\n123456", 106)
	testResponseReadLimitBodyError(t, "HTTP/1.1 400 OK\r\nContent-Type: aa\r\n\r\n123456", 5)
}

func TestResponseReadWithoutBody(t *testing.T) {
	t.Parallel()

	var resp protocol.Response

	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 304 Not Modified\r\nContent-Type: aa\r\nContent-Length: 1235\r\n\r\nfoobar", false,
		304, 1235, "aa", "foobar")

	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 204 Foo Bar\r\nContent-Type: aab\r\nTransfer-Encoding: chunked\r\n\r\n123\r\nss", false,
		204, -1, "aab", "123\r\nss")

	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 123 AAA\r\nContent-Type: xxx\r\nContent-Length: 3434\r\n\r\naaaa", false,
		123, 3434, "xxx", "aaaa")

	testResponseReadWithoutBody(t, &resp, "HTTP 200 OK\r\nContent-Type: text/xml\r\nContent-Length: 123\r\n\r\nxxxx", true,
		200, 123, "text/xml", "xxxx")

	// '100 Continue' must be skipped.
	testResponseReadWithoutBody(t, &resp, "HTTP/1.1 100 Continue\r\nFoo-bar: baz\r\n\r\nHTTP/1.1 329 aaa\r\nContent-Type: qwe\r\nContent-Length: 894\r\n\r\nfoobar", true,
		329, 894, "qwe", "foobar")
}

func verifyResponseHeader(t *testing.T, h *protocol.ResponseHeader, expectedStatusCode, expectedContentLength int, expectedContentType string) {
	if h.StatusCode() != expectedStatusCode {
		t.Fatalf("Unexpected status code %d. Expected %d", h.StatusCode(), expectedStatusCode)
	}
	if h.ContentLength() != expectedContentLength {
		t.Fatalf("Unexpected content length %d. Expected %d", h.ContentLength(), expectedContentLength)
	}
	if string(h.Peek(consts.HeaderContentType)) != expectedContentType {
		t.Fatalf("Unexpected content type %q. Expected %q", h.Peek(consts.HeaderContentType), expectedContentType)
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
	expectedStatusCode, expectedContentLength int, expectedContentType, expectedTrailer string,
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
	verifyResponseHeader(t, &resp.Header, expectedStatusCode, expectedContentLength, expectedContentType)
	assert.VerifyTrailer(t, zr, expectedTrailer)

	// verify that ordinal response is read after null-body response
	resp.SkipBody = false
	testResponseReadSuccess(t, resp, "HTTP/1.1 300 OK\r\nContent-Length: 5\r\nContent-Type: bar\r\n\r\n56789aaa",
		300, 5, "bar", "56789", "aaa")
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
