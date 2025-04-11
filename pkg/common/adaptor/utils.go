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
	"errors"
	"io"

	"github.com/cloudwego/hertz/pkg/network"
)

// methodstr tries to return consts instead of allocating mem and converting to string
func methodstr(m []byte) string {
	if len(m) == 0 {
		return "GET"
	}
	switch m[0] {
	case 'G':
		return "GET"
	case 'P':
		switch string(m) {
		case "POST":
			return "POST"
		case "PUT":
			return "PUT"
		case "PATCH":
			return "PATCH"
		}
	case 'H':
		if string(m) == "HEAD" {
			return "HEAD"
		}
	case 'D':
		if string(m) == "DELETE" {
			return "DELETE"
		}
	}
	return string(m)
}

// ooriginally from bytes.Buffer
// added Close for io.ReadCloser without io.NopCloser alloc
type bytesRWCloser struct {
	b []byte
	i int64
}

func newBytesRWCloser(b []byte) *bytesRWCloser {
	return &bytesRWCloser{b: b, i: 0}
}

func (r *bytesRWCloser) Len() int {
	if r.i >= int64(len(r.b)) {
		return 0
	}
	return int(int64(len(r.b)) - r.i)
}

func (r *bytesRWCloser) Size() int64 { return int64(len(r.b)) }

// Read implements the [io.Reader] interface.
func (r *bytesRWCloser) Read(b []byte) (n int, err error) {
	if r.i >= int64(len(r.b)) {
		return 0, io.EOF
	}
	n = copy(b, r.b[r.i:])
	r.i += int64(n)
	return
}

// ReadAt implements the [io.ReaderAt] interface.
func (r *bytesRWCloser) ReadAt(b []byte, off int64) (n int, err error) {
	// cannot modify state - see io.ReaderAt
	if off < 0 {
		return 0, errors.New("bytesRWCloser.ReadAt: negative offset")
	}
	if off >= int64(len(r.b)) {
		return 0, io.EOF
	}
	n = copy(b, r.b[off:])
	if n < len(b) {
		err = io.EOF
	}
	return
}

// Seek implements the [io.Seeker] interface.
func (r *bytesRWCloser) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.i + offset
	case io.SeekEnd:
		abs = int64(len(r.b)) + offset
	default:
		return 0, errors.New("bytesRWCloser.Seek: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("bytesRWCloser.Seek: negative position")
	}
	r.i = abs
	return abs, nil
}

// Read implements the [io.Writer] interface.
func (r *bytesRWCloser) Write(b []byte) (int, error) {
	r.b = append(r.b, b...)
	return len(b), nil
}

// WriteTo implements the [io.WriterTo] interface.
func (r *bytesRWCloser) WriteTo(w io.Writer) (n int64, err error) {
	if r.i >= int64(len(r.b)) {
		return 0, nil
	}
	b := r.b[r.i:]
	m, err := w.Write(b)
	if m > len(b) {
		panic("bytesRWCloser.WriteTo: invalid Write count")
	}
	r.i += int64(m)
	n = int64(m)
	if m != len(b) && err == nil {
		err = io.ErrShortWrite
	}
	return
}

// Reset resets the [bytesRWCloser] to be reading from b.
func (r *bytesRWCloser) Reset(b []byte) { *r = bytesRWCloser{b, 0} }

// Close implements the [io.Closer] interface.
func (r *bytesRWCloser) Close() error { return nil }

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
