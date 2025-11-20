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

package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

func TestNewVHostPathRewriter(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.Header.SetHost("foobar.com")
	req.SetRequestURI("/foo/bar/baz")
	req.CopyTo(&ctx.Request)

	f := NewVHostPathRewriter(0)
	path := f(&ctx)
	expectedPath := "/foobar.com/foo/bar/baz"
	if string(path) != expectedPath {
		t.Fatalf("unexpected path %q. Expecting %q", path, expectedPath)
	}

	ctx.Request.Reset()
	ctx.Request.SetRequestURI("https://aaa.bbb.cc/one/two/three/four?asdf=dsf")
	f = NewVHostPathRewriter(2)
	path = f(&ctx)
	expectedPath = "/aaa.bbb.cc/three/four"
	if string(path) != expectedPath {
		t.Fatalf("unexpected path %q. Expecting %q", path, expectedPath)
	}
}

func TestNewVHostPathRewriterMaliciousHost(t *testing.T) {
	var ctx RequestContext
	var req protocol.Request
	req.Header.SetHost("/../../../etc/passwd")
	req.SetRequestURI("/foo/bar/baz")
	req.CopyTo(&ctx.Request)

	f := NewVHostPathRewriter(0)
	path := f(&ctx)
	expectedPath := "/invalid-host/foo/bar/baz"
	if string(path) != expectedPath {
		t.Fatalf("unexpected path %q. Expecting %q", path, expectedPath)
	}
}

func testPathNotFound(t *testing.T, pathNotFoundFunc HandlerFunc) {
	var ctx RequestContext
	var req protocol.Request
	req.SetRequestURI("http//some.url/file")
	req.CopyTo(&ctx.Request)

	fs := &FS{
		Root:         "./",
		PathNotFound: pathNotFoundFunc,
	}
	fs.NewRequestHandler()(context.Background(), &ctx)

	if pathNotFoundFunc == nil {
		// different to ...
		if !bytes.Equal(ctx.Response.Body(),
			[]byte("Cannot open requested path")) {
			t.Fatalf("response defers. Response: %q", ctx.Response.Body())
		}
	} else {
		// Equals to ...
		if bytes.Equal(ctx.Response.Body(),
			[]byte("Cannot open requested path")) {
			t.Fatalf("response defers. Response: %q", ctx.Response.Body())
		}
	}
}

func TestPathNotFound(t *testing.T) {
	t.Parallel()

	testPathNotFound(t, nil)
}

func TestPathNotFoundFunc(t *testing.T) {
	t.Parallel()

	testPathNotFound(t, func(c context.Context, ctx *RequestContext) {
		ctx.WriteString("Not found hehe") //nolint:errcheck
	})
}

func TestServeFileHead(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.Header.SetMethod(consts.MethodHead)
	req.SetRequestURI("http://foobar.com/baz")
	req.CopyTo(&ctx.Request)

	ServeFile(&ctx, "fs.go")

	var r protocol.Response
	r.SkipBody = true
	s := resp.GetHTTP1Response(&ctx.Response).String()
	zr := mock.NewZeroCopyReader(s)
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	ce := r.Header.ContentEncoding()
	if len(ce) > 0 {
		t.Fatalf("Unexpected 'Content-Encoding' %q", ce)
	}

	body := r.Body()
	if len(body) > 0 {
		t.Fatalf("unexpected response body %q. Expecting empty body", body)
	}

	expectedBody, err := getFileContents("/fs.go")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	contentLength := r.Header.ContentLength()
	if contentLength != len(expectedBody) {
		t.Fatalf("unexpected Content-Length: %d. expecting %d", contentLength, len(expectedBody))
	}
}

func TestServeFileSmallNoReadFrom(t *testing.T) {
	t.Parallel()

	teststr := "hello, world!"

	tempdir, err := ioutil.TempDir("", "httpexpect")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)

	if err := ioutil.WriteFile(
		path.Join(tempdir, "hello"), []byte(teststr), 0o666); err != nil {
		t.Fatal(err)
	}

	var ctx RequestContext
	var req protocol.Request
	req.SetRequestURI("http://foobar.com/baz")
	req.CopyTo(&ctx.Request)

	ServeFile(&ctx, path.Join(tempdir, "hello"))

	reader, ok := ctx.Response.BodyStream().(*fsSmallFileReader)
	if !ok {
		t.Fatal("expected fsSmallFileReader")
	}

	buf := bytes.NewBuffer(nil)

	n, err := reader.WriteTo(pureWriter{buf})
	if err != nil {
		t.Fatal(err)
	}

	if n != int64(len(teststr)) {
		t.Fatalf("expected %d bytes, got %d bytes", len(teststr), n)
	}

	body := buf.String()
	if body != teststr {
		t.Fatalf("expected '%s'", teststr)
	}

	data := make([]byte, len([]byte(teststr)))
	nn, err := reader.Read(data)
	assert.DeepEqual(t, len([]byte(teststr)), nn)
	assert.Nil(t, err)
	assert.DeepEqual(t, teststr, string(data))
	assert.DeepEqual(t, reader.startPos, len([]byte(teststr)))

	nn, err = reader.Read(data)
	assert.DeepEqual(t, 0, nn)
	assert.DeepEqual(t, io.EOF, err)

	data1 := make([]byte, 2)
	reader.startPos = len([]byte(teststr)) - 1
	nn, err = reader.Read(data1)
	assert.DeepEqual(t, []byte("!"), []byte{data1[0]})
	assert.DeepEqual(t, 1, nn)
	assert.DeepEqual(t, nil, err)

	reader.startPos = 0
	reader.ff.f = nil
	buf = bytes.NewBuffer(nil)
	reader.ff.dirIndex = make([]byte, len([]byte(teststr)))
	n, err = reader.WriteTo(pureWriter{buf})
	assert.DeepEqual(t, int64(len(teststr)), n)
	assert.Nil(t, err)
}

type pureWriter struct {
	w io.Writer
}

func (pw pureWriter) Write(p []byte) (nn int, err error) {
	return pw.w.Write(p)
}

func TestServeFileCompressed(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.SetRequestURI("http://foobar.com/baz")
	req.Header.Set(consts.HeaderAcceptEncoding, "gzip")
	req.CopyTo(&ctx.Request)

	ServeFile(&ctx, "fs.go")

	var r protocol.Response
	s := resp.GetHTTP1Response(&ctx.Response).String()
	zr := mock.NewZeroCopyReader(s)
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	ce := r.Header.ContentEncoding()
	if string(ce) != "gzip" {
		t.Fatalf("Unexpected 'Content-Encoding' %q. Expecting %q", ce, "gzip")
	}

	body, err := r.BodyGunzip()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedBody, err := getFileContents("/fs.go")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !bytes.Equal(body, expectedBody) {
		t.Fatalf("unexpected body %q. expecting %q", body, expectedBody)
	}
}

func TestServeFileUncompressed(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.SetRequestURI("http://foobar.com/baz")
	req.Header.Set(consts.HeaderAcceptEncoding, "gzip")
	req.CopyTo(&ctx.Request)

	ServeFileUncompressed(&ctx, "fs.go")

	var r protocol.Response
	s := resp.GetHTTP1Response(&ctx.Response).String()
	zr := mock.NewZeroCopyReader(s)
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	ce := r.Header.ContentEncoding()
	if len(ce) > 0 {
		t.Fatalf("Unexpected 'Content-Encoding' %q", ce)
	}

	body := r.Body()
	expectedBody, err := getFileContents("/fs.go")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !bytes.Equal(body, expectedBody) {
		t.Fatalf("unexpected body %q. expecting %q", body, expectedBody)
	}
}

func TestFSByteRangeConcurrent(t *testing.T) {
	t.Parallel()

	fs := &FS{
		Root:            ".",
		AcceptByteRange: true,
	}
	h := fs.NewRequestHandler()

	concurrency := 10
	ch := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				testFSByteRange(t, h, "/fs.go")
			}
			ch <- struct{}{}
		}()
	}

	for i := 0; i < concurrency; i++ {
		select {
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		case <-ch:
		}
	}
}

func TestFSByteRangeSingleThread(t *testing.T) {
	t.Parallel()

	fs := &FS{
		Root:            ".",
		AcceptByteRange: true,
	}
	h := fs.NewRequestHandler()

	testFSByteRange(t, h, "/fs.go")
}

func testFSByteRange(t *testing.T, h HandlerFunc, filePath string) {
	var ctx RequestContext
	req := &protocol.Request{}
	req.CopyTo(&ctx.Request)

	expectedBody, err := getFileContents(filePath)
	if err != nil {
		t.Fatalf("cannot read file %q: %s", filePath, err)
	}

	fileSize := len(expectedBody)
	startPos := rand.Intn(fileSize)
	endPos := rand.Intn(fileSize)
	if endPos < startPos {
		startPos, endPos = endPos, startPos
	}

	ctx.Request.SetRequestURI(filePath)
	ctx.Request.Header.SetByteRange(startPos, endPos)
	h(context.Background(), &ctx)

	var r protocol.Response
	s := resp.GetHTTP1Response(&ctx.Response).String()
	zr := mock.NewZeroCopyReader(s)
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s. filePath=%q", err, filePath)
	}
	if r.StatusCode() != consts.StatusPartialContent {
		t.Fatalf("unexpected status code: %d. Expecting %d. filePath=%q", r.StatusCode(), consts.StatusPartialContent, filePath)
	}
	cr := r.Header.Peek(consts.HeaderContentRange)

	expectedCR := fmt.Sprintf("bytes %d-%d/%d", startPos, endPos, fileSize)
	if string(cr) != expectedCR {
		t.Fatalf("unexpected content-range %q. Expecting %q. filePath=%q", cr, expectedCR, filePath)
	}
	body := r.Body()
	bodySize := endPos - startPos + 1
	if len(body) != bodySize {
		t.Fatalf("unexpected body size %d. Expecting %d. filePath=%q, startPos=%d, endPos=%d",
			len(body), bodySize, filePath, startPos, endPos)
	}

	expectedBody = expectedBody[startPos : endPos+1]
	if !bytes.Equal(body, expectedBody) {
		t.Fatalf("unexpected body %q. Expecting %q. filePath=%q, startPos=%d, endPos=%d",
			body, expectedBody, filePath, startPos, endPos)
	}
}

func getFileContents(path string) ([]byte, error) {
	path = "." + path
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

func TestParseByteRangeSuccess(t *testing.T) {
	t.Parallel()

	testParseByteRangeSuccess(t, "bytes=0-0", 1, 0, 0)
	testParseByteRangeSuccess(t, "bytes=1234-6789", 6790, 1234, 6789)

	testParseByteRangeSuccess(t, "bytes=123-", 456, 123, 455)
	testParseByteRangeSuccess(t, "bytes=-1", 1, 0, 0)
	testParseByteRangeSuccess(t, "bytes=-123", 456, 333, 455)

	// End position exceeding content-length. It should be updated to content-length-1.
	// See https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.35
	testParseByteRangeSuccess(t, "bytes=1-2345", 234, 1, 233)
	testParseByteRangeSuccess(t, "bytes=0-2345", 2345, 0, 2344)

	// Start position overflow. Whole range must be returned.
	// See https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.35
	testParseByteRangeSuccess(t, "bytes=-567", 56, 0, 55)
}

func testParseByteRangeSuccess(t *testing.T, v string, contentLength, startPos, endPos int) {
	startPos1, endPos1, err := ParseByteRange([]byte(v), contentLength)
	if err != nil {
		t.Fatalf("unexpected error: %s. v=%q, contentLength=%d", err, v, contentLength)
	}
	if startPos1 != startPos {
		t.Fatalf("unexpected startPos=%d. Expecting %d. v=%q, contentLength=%d", startPos1, startPos, v, contentLength)
	}
	if endPos1 != endPos {
		t.Fatalf("unexpected endPos=%d. Expectind %d. v=%q, contentLength=%d", endPos1, endPos, v, contentLength)
	}
}

func TestParseByteRangeError(t *testing.T) {
	t.Parallel()

	// invalid value
	testParseByteRangeError(t, "asdfasdfas", 1234)

	// invalid units
	testParseByteRangeError(t, "foobar=1-34", 600)

	// missing '-'
	testParseByteRangeError(t, "bytes=1234", 1235)

	// non-numeric range
	testParseByteRangeError(t, "bytes=foobar", 123)
	testParseByteRangeError(t, "bytes=1-foobar", 123)
	testParseByteRangeError(t, "bytes=df-344", 545)

	// multiple byte ranges
	testParseByteRangeError(t, "bytes=1-2,4-6", 123)

	// byte range exceeding contentLength
	testParseByteRangeError(t, "bytes=123-", 12)

	// startPos exceeding endPos
	testParseByteRangeError(t, "bytes=123-34", 1234)
}

func testParseByteRangeError(t *testing.T, v string, contentLength int) {
	_, _, err := ParseByteRange([]byte(v), contentLength)
	if err == nil {
		t.Fatalf("expecting error when parsing byte range %q", v)
	}
}

func TestFSCompressConcurrent(t *testing.T) {
	// This test can't run parallel as files in / might by changed by other tests.

	fs := &FS{
		Root:               ".",
		GenerateIndexPages: true,
		Compress:           true,
	}
	h := fs.NewRequestHandler()

	concurrency := 4
	ch := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				testFSCompress(t, h, "/fs.go")
				testFSCompress(t, h, "/")
			}
			ch <- struct{}{}
		}()
	}

	for i := 0; i < concurrency; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}
}

func TestFSCompressSingleThread(t *testing.T) {
	// This test can't run parallel as files in / might by changed by other tests.

	fs := &FS{
		Root:               ".",
		GenerateIndexPages: true,
		Compress:           true,
	}
	h := fs.NewRequestHandler()

	testFSCompress(t, h, "/fs.go")
	testFSCompress(t, h, "/")
}

func testFSCompress(t *testing.T, h HandlerFunc, filePath string) {
	var ctx RequestContext
	req := &protocol.Request{}
	req.CopyTo(&ctx.Request)

	// request uncompressed file
	ctx.Request.Reset()
	ctx.Request.SetRequestURI(filePath)
	h(context.Background(), &ctx)

	var r protocol.Response
	s := resp.GetHTTP1Response(&ctx.Response).String()
	zr := mock.NewZeroCopyReader(s)
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s. filePath=%q", err, filePath)
	}
	if r.StatusCode() != consts.StatusOK {
		t.Fatalf("unexpected status code: %d. Expecting %d. filePath=%q", r.StatusCode(), consts.StatusOK, filePath)
	}
	ce := r.Header.ContentEncoding()
	if string(ce) != "" {
		t.Fatalf("unexpected content-encoding %q. Expecting empty string. filePath=%q", ce, filePath)
	}
	body := string(r.Body())

	// request compressed file
	ctx.Request.Reset()
	ctx.Request.SetRequestURI(filePath)
	ctx.Request.Header.Set(consts.HeaderAcceptEncoding, "gzip")
	h(context.Background(), &ctx)
	s = resp.GetHTTP1Response(&ctx.Response).String()
	zr = mock.NewZeroCopyReader(s)
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s. filePath=%q", err, filePath)
	}
	if r.StatusCode() != consts.StatusOK {
		t.Fatalf("unexpected status code: %d. Expecting %d. filePath=%q", r.StatusCode(), consts.StatusOK, filePath)
	}
	ce = r.Header.ContentEncoding()
	if string(ce) != "gzip" {
		t.Fatalf("unexpected content-encoding %q. Expecting %q. filePath=%q", ce, "gzip", filePath)
	}
	zbody, err := r.BodyGunzip()
	if err != nil {
		t.Fatalf("unexpected error when gunzipping response body: %s. filePath=%q", err, filePath)
	}
	if string(zbody) != body {
		t.Fatalf("unexpected body len=%d. Expected len=%d. FilePath=%q", len(zbody), len(body), filePath)
	}
}

func TestFileLock(t *testing.T) {
	t.Parallel()

	for i := 0; i < 10; i++ {
		filePath := fmt.Sprintf("foo/bar/%d.jpg", i)
		lock := getFileLock(filePath)
		lock.Lock()
		time.Sleep(time.Microsecond)
		lock.Unlock() // nolint:staticcheck
	}

	for i := 0; i < 10; i++ {
		filePath := fmt.Sprintf("foo/bar/%d.jpg", i)
		lock := getFileLock(filePath)
		lock.Lock()
		time.Sleep(time.Microsecond)
		lock.Unlock() // nolint:staticcheck
	}
}

func TestStripPathSlashes(t *testing.T) {
	t.Parallel()

	testStripPathSlashes(t, "", 0, "")
	testStripPathSlashes(t, "", 10, "")
	testStripPathSlashes(t, "/", 0, "")
	testStripPathSlashes(t, "/", 1, "")
	testStripPathSlashes(t, "/", 10, "")
	testStripPathSlashes(t, "/foo/bar/baz", 0, "/foo/bar/baz")
	testStripPathSlashes(t, "/foo/bar/baz", 1, "/bar/baz")
	testStripPathSlashes(t, "/foo/bar/baz", 2, "/baz")
	testStripPathSlashes(t, "/foo/bar/baz", 3, "")
	testStripPathSlashes(t, "/foo/bar/baz", 10, "")

	// trailing slash
	testStripPathSlashes(t, "/foo/bar/", 0, "/foo/bar")
	testStripPathSlashes(t, "/foo/bar/", 1, "/bar")
	testStripPathSlashes(t, "/foo/bar/", 2, "")
	testStripPathSlashes(t, "/foo/bar/", 3, "")
}

func testStripPathSlashes(t *testing.T, path string, stripSlashes int, expectedPath string) {
	s := stripLeadingSlashes([]byte(path), stripSlashes)
	s = stripTrailingSlashes(s)
	if string(s) != expectedPath {
		t.Fatalf("unexpected path after stripping %q with stripSlashes=%d: %q. Expecting %q", path, stripSlashes, s, expectedPath)
	}
}

func TestFileExtension(t *testing.T) {
	t.Parallel()

	testFileExtension(t, "foo.bar", false, "zzz", ".bar")
	testFileExtension(t, "foobar", false, "zzz", "")
	testFileExtension(t, "foo.bar.baz", false, "zzz", ".baz")
	testFileExtension(t, "", false, "zzz", "")
	testFileExtension(t, "/a/b/c.d/efg.jpg", false, ".zzz", ".jpg")

	testFileExtension(t, "foo.bar", true, ".zzz", ".bar")
	testFileExtension(t, "foobar.zzz", true, ".zzz", "")
	testFileExtension(t, "foo.bar.baz.hertz.gz", true, ".hertz.gz", ".baz")
	testFileExtension(t, "", true, ".zzz", "")
	testFileExtension(t, "/a/b/c.d/efg.jpg.xxx", true, ".xxx", ".jpg")
}

func testFileExtension(t *testing.T, path string, compressed bool, compressedFileSuffix, expectedExt string) {
	ext := fileExtension(path, compressed, compressedFileSuffix)
	if ext != expectedExt {
		t.Fatalf("unexpected file extension for file %q: %q. Expecting %q", path, ext, expectedExt)
	}
}

func TestServeFileContentType(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.Header.SetMethod(consts.MethodGet)
	req.SetRequestURI("http://foobar.com/baz")
	req.CopyTo(&ctx.Request)

	ServeFile(&ctx, "../common/testdata/test.png")

	var r protocol.Response
	s := resp.GetHTTP1Response(&ctx.Response).String()
	zr := mock.NewZeroCopyReader(s)
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expected := []byte(consts.MIMEImagePNG)
	if !bytes.Equal(r.Header.ContentType(), expected) {
		t.Fatalf("Unexpected Content-Type, expected: %q got %q", expected, r.Header.ContentType())
	}
}

func TestFileSmallUpdateByteRange(t *testing.T) {
	r := &fsSmallFileReader{}
	err := r.UpdateByteRange(1, 1)
	assert.Nil(t, err)
	assert.DeepEqual(t, 1, r.startPos)
	assert.DeepEqual(t, 2, r.endPos)
}
