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

package ext

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const maxContentLengthInStream = 8 * 1024

var errBrokenChunk = errs.NewPublic("cannot find crlf at the end of chunk").SetMeta("when read body chunk")

func MustPeekBuffered(r network.Reader) []byte {
	l := r.Len()
	buf, err := r.Peek(l)
	if len(buf) == 0 || err != nil {
		panic(fmt.Sprintf("bufio.Reader.Peek() returned unexpected data (%q, %v)", buf, err))
	}

	return buf
}

func MustDiscard(r network.Reader, n int) {
	if err := r.Skip(n); err != nil {
		panic(fmt.Sprintf("bufio.Reader.Discard(%d) failed: %s", n, err))
	}
}

func ReadRawHeaders(dst, buf []byte) ([]byte, int, error) {
	n := bytes.IndexByte(buf, '\n')
	if n < 0 {
		return dst[:0], 0, errNeedMore
	}
	if (n == 1 && buf[0] == '\r') || n == 0 {
		// empty headers
		return dst, n + 1, nil
	}

	n++
	b := buf
	m := n
	for {
		b = b[m:]
		m = bytes.IndexByte(b, '\n')
		if m < 0 {
			return dst, 0, errNeedMore
		}
		m++
		n += m
		if (m == 2 && b[0] == '\r') || m == 1 {
			dst = append(dst, buf[:n]...)
			return dst, n, nil
		}
	}
}

func WriteBodyChunked(w network.Writer, r io.Reader) error {
	vbuf := utils.CopyBufPool.Get()
	buf := vbuf.([]byte)

	var err error
	var n int
	for {
		n, err = r.Read(buf)
		if n == 0 {
			if err == nil {
				panic("BUG: io.Reader returned 0, nil")
			}

			if !errors.Is(err, io.EOF) {
				hlog.SystemLogger().Warnf("writing chunked response body encountered an error from the reader, "+
					"this may cause the short of the content in response body, error: %s", err.Error())
			}

			if err = WriteChunk(w, buf[:0], true); err != nil {
				break
			}

			err = nil
			break
		}
		if err = WriteChunk(w, buf[:n], true); err != nil {
			break
		}
	}

	utils.CopyBufPool.Put(vbuf)
	return err
}

func WriteBodyFixedSize(w network.Writer, r io.Reader, size int64) error {
	if size == 0 {
		return nil
	}
	if size > consts.MaxSmallFileSize {
		if err := w.Flush(); err != nil {
			return err
		}
	}

	if size > 0 {
		r = io.LimitReader(r, size)
	}

	n, err := utils.CopyZeroAlloc(w, r)

	if n != size && err == nil {
		err = fmt.Errorf("copied %d bytes from body stream instead of %d bytes", n, size)
	}
	return err
}

func appendBodyFixedSize(r network.Reader, dst []byte, n int) ([]byte, error) {
	if n == 0 {
		return dst, nil
	}

	offset := len(dst)
	dstLen := offset + n
	if cap(dst) < dstLen {
		b := make([]byte, round2(dstLen))
		copy(b, dst)
		dst = b
	}
	dst = dst[:dstLen]

	// Peek can get all data, otherwise it will through error
	buf, err := r.Peek(n)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return dst[:offset], err
	}
	copy(dst[offset:], buf)
	r.Skip(len(buf)) // nolint: errcheck
	return dst, nil
}

func readBodyIdentity(r network.Reader, maxBodySize int, dst []byte) ([]byte, error) {
	dst = dst[:cap(dst)]
	if len(dst) == 0 {
		dst = make([]byte, 1024)
	}
	offset := 0
	for {
		nn := r.Len()

		if nn == 0 {
			_, err := r.Peek(1)
			if err != nil {
				return dst[:offset], nil
			}
			nn = r.Len()
		}
		if nn >= (len(dst) - offset) {
			nn = len(dst) - offset
		}

		buf, err := r.Peek(nn)
		if err != nil {
			return dst[:offset], err
		}
		copy(dst[offset:], buf)
		r.Skip(nn) // nolint: errcheck

		offset += nn
		if maxBodySize > 0 && offset > maxBodySize {
			return dst[:offset], errBodyTooLarge
		}
		if len(dst) == offset {
			n := round2(2 * offset)
			if maxBodySize > 0 && n > maxBodySize {
				n = maxBodySize + 1
			}
			b := make([]byte, n)
			copy(b, dst)
			dst = b
		}
	}
}

func ReadBody(r network.Reader, contentLength, maxBodySize int, dst []byte) ([]byte, error) {
	dst = dst[:0]
	if contentLength >= 0 {
		if maxBodySize > 0 && contentLength > maxBodySize {
			return dst, errBodyTooLarge
		}
		return appendBodyFixedSize(r, dst, contentLength)
	}
	if contentLength == -1 {
		return readBodyChunked(r, maxBodySize, dst)
	}
	return readBodyIdentity(r, maxBodySize, dst)
}

func LimitedReaderSize(r io.Reader) int64 {
	lr, ok := r.(*io.LimitedReader)
	if !ok {
		return -1
	}
	return lr.N
}

func readBodyChunked(r network.Reader, maxBodySize int, dst []byte) ([]byte, error) {
	if len(dst) > 0 {
		panic("BUG: expected zero-length buffer")
	}

	strCRLFLen := len(bytestr.StrCRLF)
	for {
		chunkSize, err := utils.ParseChunkSize(r)
		if err != nil {
			return dst, err
		}
		// If it is the end of chunk, Read CRLF after reading trailer
		if chunkSize == 0 {
			return dst, nil
		}
		if maxBodySize > 0 && len(dst)+chunkSize > maxBodySize {
			return dst, errBodyTooLarge
		}
		dst, err = appendBodyFixedSize(r, dst, chunkSize+strCRLFLen)
		if err != nil {
			return dst, err
		}
		if !bytes.Equal(dst[len(dst)-strCRLFLen:], bytestr.StrCRLF) {
			return dst, errBrokenChunk
		}
		dst = dst[:len(dst)-strCRLFLen]
	}
}

func round2(n int) int {
	if n <= 0 {
		return 0
	}
	n--
	x := uint(0)
	for n > 0 {
		n >>= 1
		x++
	}
	return 1 << x
}

func WriteChunk(w network.Writer, b []byte, withFlush bool) (err error) {
	n := len(b)
	if err = bytesconv.WriteHexInt(w, n); err != nil {
		return err
	}

	w.WriteBinary(bytestr.StrCRLF) //nolint:errcheck
	if _, err = w.WriteBinary(b); err != nil {
		return err
	}

	// If it is the end of chunk, write CRLF after writing trailer
	if n > 0 {
		w.WriteBinary(bytestr.StrCRLF) //nolint:errcheck
	}

	if !withFlush {
		return nil
	}
	err = w.Flush()
	return
}

func isOnlyCRLF(b []byte) bool {
	for _, ch := range b {
		if ch != '\r' && ch != '\n' {
			return false
		}
	}
	return true
}

func BufferSnippet(b []byte) string {
	n := len(b)
	start := 20
	end := n - start
	if start >= end {
		start = n
		end = n
	}
	bStart, bEnd := b[:start], b[end:]
	if len(bEnd) == 0 {
		return fmt.Sprintf("%q", b)
	}
	return fmt.Sprintf("%q...%q", bStart, bEnd)
}

func normalizeHeaderValue(ov, ob []byte, headerLength int) (nv, nb []byte, nhl int) {
	nv = ov
	length := len(ov)
	if length <= 0 {
		return
	}
	write := 0
	shrunk := 0
	lineStart := false
	for read := 0; read < length; read++ {
		c := ov[read]
		if c == '\r' || c == '\n' {
			shrunk++
			if c == '\n' {
				lineStart = true
			}
			continue
		} else if lineStart && c == '\t' {
			c = ' '
		} else {
			lineStart = false
		}
		nv[write] = c
		write++
	}

	nv = nv[:write]
	copy(ob[write:], ob[write+shrunk:])

	// Check if we need to skip \r\n or just \n
	skip := 0
	if ob[write] == '\r' {
		if ob[write+1] == '\n' {
			skip += 2
		} else {
			skip++
		}
	} else if ob[write] == '\n' {
		skip++
	}

	nb = ob[write+skip : len(ob)-shrunk]
	nhl = headerLength - shrunk
	return
}

func stripSpace(b []byte) []byte {
	for len(b) > 0 && b[0] == ' ' {
		b = b[1:]
	}
	for len(b) > 0 && b[len(b)-1] == ' ' {
		b = b[:len(b)-1]
	}
	return b
}

func SkipTrailer(r network.Reader) error {
	n := 1
	for {
		err := trySkipTrailer(r, n)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errs.ErrNeedMore) {
			return err
		}
		// No more data available on the wire, try block peek(by netpoll)
		if n == r.Len() {
			n++

			continue
		}
		n = r.Len()
	}
}

func trySkipTrailer(r network.Reader, n int) error {
	b, err := r.Peek(n)
	if len(b) == 0 {
		// Return ErrTimeout on any timeout.
		if err != nil && strings.Contains(err.Error(), "timeout") {
			return errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "read response header")
		}

		if n == 1 || err == io.EOF {
			return io.EOF
		}

		return errs.NewPublicf("error when reading request trailer: %w", err)
	}
	b = MustPeekBuffered(r)
	headersLen, errParse := skipTrailer(b)
	if errParse != nil {
		if err == io.EOF {
			return err
		}
		return HeaderError("response", err, errParse, b)
	}
	MustDiscard(r, headersLen)
	return nil
}

func skipTrailer(buf []byte) (int, error) {
	skip := 0
	strCRLFLen := len(bytestr.StrCRLF)
	for {
		index := bytes.Index(buf, bytestr.StrCRLF)
		if index == -1 {
			return 0, errs.ErrNeedMore
		}

		buf = buf[index+strCRLFLen:]
		skip += index + strCRLFLen

		if index == 0 {
			return skip, nil
		}
	}
}

func ReadTrailer(t *protocol.Trailer, r network.Reader) error {
	n := 1
	for {
		err := tryReadTrailer(t, r, n)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errs.ErrNeedMore) {
			t.ResetSkipNormalize()
			return err
		}
		// No more data available on the wire, try block peek(by netpoll)
		if n == r.Len() {
			n++

			continue
		}
		n = r.Len()
	}
}

func tryReadTrailer(t *protocol.Trailer, r network.Reader, n int) error {
	b, err := r.Peek(n)
	if len(b) == 0 {
		// Return ErrTimeout on any timeout.
		if err != nil && strings.Contains(err.Error(), "timeout") {
			return errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "read response header")
		}

		if n == 1 || err == io.EOF {
			return io.EOF
		}

		return errs.NewPublicf("error when reading request trailer: %w", err)
	}
	b = MustPeekBuffered(r)
	headersLen, errParse := parseTrailer(t, b)
	if errParse != nil {
		if err == io.EOF {
			return err
		}
		return HeaderError("response", err, errParse, b)
	}
	MustDiscard(r, headersLen)
	return nil
}

func parseTrailer(t *protocol.Trailer, buf []byte) (int, error) {
	// Skip any 0 length chunk.
	if buf[0] == '0' {
		skip := len(bytestr.StrCRLF) + 1
		if len(buf) < skip {
			return 0, io.EOF
		}
		buf = buf[skip:]
	}

	var s HeaderScanner
	s.B = buf
	s.DisableNormalizing = t.IsDisableNormalizing()
	var err error
	for s.Next() {
		if len(s.Key) > 0 {
			if bytes.IndexByte(s.Key, ' ') != -1 || bytes.IndexByte(s.Key, '\t') != -1 {
				err = fmt.Errorf("invalid trailer key %q", s.Key)
				continue
			}
			err = t.UpdateArgBytes(s.Key, s.Value)
		}
	}
	if s.Err != nil {
		return 0, s.Err
	}
	if err != nil {
		return 0, err
	}
	return s.HLen, nil
}

// writeTrailer writes response trailer to w
func WriteTrailer(t *protocol.Trailer, w network.Writer) error {
	_, err := w.WriteBinary(t.Header())
	return err
}
