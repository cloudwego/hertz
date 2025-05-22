/*
 * Copyright 2025 CloudWeGo Authors
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
	"context"
	"embed"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
	"testing"
	"time"
)

//go:embed fs.go fs_fs_test.go
var fsTestFilesystem embed.FS

func TestFSServeFileHead(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.Header.SetMethod(consts.MethodGet)
	req.SetRequestURI("http://foobar.com/baz")
	req.CopyTo(&ctx.Request)

	ServeFS(context.Background(), &ctx, fsTestFilesystem, "fs.go")

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

func TestServeFSCompressed(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.SetRequestURI("http://foobar.com/baz")
	req.Header.Set(consts.HeaderAcceptEncoding, "gzip")
	req.CopyTo(&ctx.Request)

	ServeFS(context.Background(), &ctx, fsTestFilesystem, "fs.go")

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

func TestFSServeFileUncompressed(t *testing.T) {
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

func TestFSFSByteRangeConcurrent(t *testing.T) {
	t.Parallel()

	stop := make(chan struct{})
	defer close(stop)

	fs := &FS{
		FS:              fsTestFilesystem,
		Root:            "",
		AcceptByteRange: true,
	}
	h := fs.NewRequestHandler()

	concurrency := 10
	ch := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				testFSByteRange(t, h, "/fs.go")
				testFSByteRange(t, h, "/fs_fs_test.go")
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

func TestFSFSByteRangeSingleThread(t *testing.T) {
	t.Parallel()

	stop := make(chan struct{})
	defer close(stop)

	fs := &FS{
		FS:              fsTestFilesystem,
		Root:            ".",
		AcceptByteRange: true,
	}
	h := fs.NewRequestHandler()

	testFSByteRange(t, h, "/fs.go")
	testFSByteRange(t, h, "/fs_fs_test.go")
}
