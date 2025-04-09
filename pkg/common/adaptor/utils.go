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

package adaptor

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unsafe"

	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// methodstr tries to return consts without allocation
func methodstr(m []byte) string {
	if len(m) == 0 {
		return "GET"
	}
	switch m[0] {
	case 'G':
		if string(m) == consts.MethodGet {
			return consts.MethodGet
		}
	case 'P':
		switch string(m) {
		case consts.MethodPost:
			return consts.MethodPost

		case consts.MethodPut:
			return consts.MethodPut

		case consts.MethodPatch:
			return consts.MethodPatch
		}
	case 'H':
		if string(m) == consts.MethodHead {
			return consts.MethodHead
		}
	case 'D':
		if string(m) == consts.MethodDelete {
			return consts.MethodDelete
		}
	}
	return string(m)
}

type bytesRWCloser struct {
	bytes.Reader
}

// newBytesRWCloser adds noop Close method to bytes.Reader without allocation
func newBytesRWCloser(b []byte) io.ReadCloser {
	rd := bytes.NewReader(b)
	return (*bytesRWCloser)(unsafe.Pointer(rd))
}

// Close implements the [io.Closer] interface.
func (bytesRWCloser) Close() error { return nil }

func writer2writerExt(w network.Writer) network.ExtWriter {
	return extWriter{w}
}

type extWriter struct {
	network.Writer
}

func (w extWriter) Write(b []byte) (int, error) {
	buf, err := w.Writer.Malloc(len(b))
	if err != nil {
		return 0, err
	}
	return copy(buf, b), nil
}

func (w extWriter) Finalize() error {
	return w.Flush()
}

func reader2closer(r io.Reader) io.ReadCloser {
	rc, ok := r.(io.ReadCloser)
	if ok {
		return rc
	}
	return io.NopCloser(r)
}

func parseHTTPVersion(s string) (major, minor int, _ error) {
	v := strings.TrimPrefix(s, "HTTP/")
	if len(v) == len(s) {
		return 1, 1, fmt.Errorf("invalid http version: %q", s)
	}
	switch v {
	case "1.0":
		return 1, 0, nil

	case "1.1":
		return 1, 1, nil

	default:
		a, b, ok := strings.Cut(v, ".")
		if ok {
			major, err1 := strconv.Atoi(a)
			minor, err2 := strconv.Atoi(b)
			if err1 == nil && err2 == nil {
				return major, minor, nil
			}
		}
	}
	return 1, 1, fmt.Errorf("invalid http version: %q", s)
}
