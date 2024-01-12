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
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/ext"
)

var errTimeout = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "read response header")

// Read reads response header from r.
//
// io.EOF is returned if r is closed before reading the first header byte.
func ReadHeader(h *protocol.ResponseHeader, r network.Reader) error {
	n := 1
	for {
		err := tryRead(h, r, n)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errs.ErrNeedMore) {
			h.ResetSkipNormalize()
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

// WriteHeader writes response header to w.
func WriteHeader(h *protocol.ResponseHeader, w network.Writer) error {
	header := h.Header()
	h.SetHeaderLength(len(header))
	_, err := w.WriteBinary(header)
	if err != nil {
		return err
	}
	return nil
}

// ConnectionUpgrade returns true if 'Connection: Upgrade' header is set.
func ConnectionUpgrade(h *protocol.ResponseHeader) bool {
	return ext.HasHeaderValue(h.Peek(consts.HeaderConnection), bytestr.StrKeepAlive)
}

func tryRead(h *protocol.ResponseHeader, r network.Reader, n int) error {
	h.ResetSkipNormalize()
	b, err := r.Peek(n)
	if len(b) == 0 {
		// Return ErrTimeout on any timeout.
		if err != nil && strings.Contains(err.Error(), "timeout") {
			return errTimeout
		}
		// treat all other errors on the first byte read as EOF
		if n == 1 || err == io.EOF {
			return io.EOF
		}

		return fmt.Errorf("error when reading response headers: %s", err)
	}
	b = ext.MustPeekBuffered(r)
	headersLen, errParse := parse(h, b)
	if errParse != nil {
		return ext.HeaderError("response", err, errParse, b)
	}
	ext.MustDiscard(r, headersLen)
	return nil
}

func parseHeaders(h *protocol.ResponseHeader, buf []byte) (int, error) {
	// 'identity' content-length by default
	h.InitContentLengthWithValue(-2)

	var s ext.HeaderScanner
	s.B = buf
	s.DisableNormalizing = h.IsDisableNormalizing()
	var err error
	for s.Next() {
		if len(s.Key) > 0 {
			switch s.Key[0] | 0x20 {
			case 'c':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentType) {
					h.SetContentTypeBytes(s.Value)
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentEncoding) {
					h.SetContentEncodingBytes(s.Value)
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentLength) {
					var contentLength int
					if h.ContentLength() != -1 {
						if contentLength, err = protocol.ParseContentLength(s.Value); err != nil {
							h.InitContentLengthWithValue(-2)
						} else {
							h.InitContentLengthWithValue(contentLength)
							h.SetContentLengthBytes(s.Value)
						}
					}
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrConnection) {
					if bytes.Equal(s.Value, bytestr.StrClose) {
						h.SetConnectionClose(true)
					} else {
						h.SetConnectionClose(false)
						h.AddArgBytes(s.Key, s.Value, protocol.ArgsHasValue)
					}
					continue
				}
			case 's':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrServer) {
					h.SetServerBytes(s.Value)
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrSetCookie) {
					h.ParseSetCookie(s.Value)
					continue
				}
			case 't':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrTransferEncoding) {
					if !bytes.Equal(s.Value, bytestr.StrIdentity) {
						h.InitContentLengthWithValue(-1)
						h.SetArgBytes(bytestr.StrTransferEncoding, bytestr.StrChunked, protocol.ArgsHasValue)
					}
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrTrailer) {
					err = h.Trailer().SetTrailers(s.Value)
					continue
				}
			}
			h.AddArgBytes(s.Key, s.Value, protocol.ArgsHasValue)
		}
	}
	if s.Err != nil {
		h.SetConnectionClose(true)
		return 0, s.Err
	}

	if h.ContentLength() < 0 {
		h.SetContentLengthBytes(h.ContentLengthBytes()[:0])
	}
	if h.ContentLength() == -2 && !ConnectionUpgrade(h) && !h.MustSkipContentLength() {
		h.SetArgBytes(bytestr.StrTransferEncoding, bytestr.StrIdentity, protocol.ArgsHasValue)
		h.SetConnectionClose(true)
	}
	if !h.IsHTTP11() && !h.ConnectionClose() {
		// close connection for non-http/1.1 response unless 'Connection: keep-alive' is set.
		v := h.PeekArgBytes(bytestr.StrConnection)
		h.SetConnectionClose(!ext.HasHeaderValue(v, bytestr.StrKeepAlive))
	}

	return len(buf) - len(s.B), err
}

func parse(h *protocol.ResponseHeader, buf []byte) (int, error) {
	m, err := parseFirstLine(h, buf)
	if err != nil {
		return 0, err
	}
	n, err := parseHeaders(h, buf[m:])
	if err != nil {
		return 0, err
	}
	return m + n, nil
}

func parseFirstLine(h *protocol.ResponseHeader, buf []byte) (int, error) {
	bNext := buf
	var b []byte
	var err error
	for len(b) == 0 {
		if b, bNext, err = utils.NextLine(bNext); err != nil {
			return 0, err
		}
	}

	// parse protocol
	n := bytes.IndexByte(b, ' ')
	if n < 0 {
		return 0, fmt.Errorf("cannot find whitespace in the first line of response %q", buf)
	}

	isHTTP11 := bytes.Equal(b[:n], bytestr.StrHTTP11)
	if !isHTTP11 {
		h.SetProtocol(consts.HTTP10)
	} else {
		h.SetProtocol(consts.HTTP11)
	}

	b = b[n+1:]

	// parse status code
	var statusCode int
	statusCode, n, err = bytesconv.ParseUintBuf(b)
	h.SetStatusCode(statusCode)
	if err != nil {
		return 0, fmt.Errorf("cannot parse response status code: %s. Response %q", err, buf)
	}
	if len(b) > n && b[n] != ' ' {
		return 0, fmt.Errorf("unexpected char at the end of status code. Response %q", buf)
	}

	return len(buf) - len(bNext), nil
}
