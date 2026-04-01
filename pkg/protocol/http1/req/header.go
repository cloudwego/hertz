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

package req

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/ext"
)

var errEOFReadHeader = errs.NewPublic("error when reading request headers: EOF")

// Write writes request header to w.
func WriteHeader(h *protocol.RequestHeader, w network.Writer) error {
	header := h.Header()
	_, err := w.WriteBinary(header)
	return err
}

func ReadHeader(h *protocol.RequestHeader, r network.Reader) error {
	return ReadHeaderWithLimit(h, r, 0)
}

func ReadHeaderWithLimit(h *protocol.RequestHeader, r network.Reader, maxHeaderBytes int) error {
	n := 1
	for {
		err := tryReadWithLimit(h, r, n, maxHeaderBytes)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errs.ErrNeedMore) {
			h.ResetSkipNormalize()
			return err
		}

		// No more data available on the wire, try block peek
		if n == r.Len() {
			n++
			continue
		}
		n = r.Len()
	}
}

func tryReadWithLimit(h *protocol.RequestHeader, r network.Reader, n, maxHeaderBytes int) error {
	h.ResetSkipNormalize()
	b, err := r.Peek(n)
	if len(b) == 0 {
		if err != io.EOF {
			return err
		}

		// n == 1 on the first read for the request.
		if n == 1 {
			// We didn't read a single byte.
			return errs.New(errs.ErrNothingRead, errs.ErrorTypePrivate, err)
		}

		return errEOFReadHeader
	}
	b = ext.MustPeekBuffered(r)
	if maxHeaderBytes > 0 && len(b) > maxHeaderBytes {
		b = b[:maxHeaderBytes]
	}
	headersLen, errParse := parse(h, b)
	if errParse != nil {
		if maxHeaderBytes > 0 && len(b) >= maxHeaderBytes && errors.Is(errParse, errs.ErrNeedMore) {
			return errHeaderTooLarge
		}
		return ext.HeaderError("request", err, errParse, b)
	}
	ext.MustDiscard(r, headersLen)
	return nil
}

func parse(h *protocol.RequestHeader, buf []byte) (int, error) {
	m, err := parseFirstLine(h, buf)
	if err != nil {
		return 0, err
	}
	rawHeaders, _, err := ext.ReadRawHeaders(h.RawHeaders()[:0], buf[m:])
	h.SetRawHeaders(rawHeaders)
	if err != nil {
		return 0, err
	}
	n, err := parseHeaders(h, buf[m:])
	if err != nil {
		return 0, err
	}
	return m + n, nil
}

const (
	maxCheckMethodLen = 10

	// reuse ValidHeaderFieldNameTable for Method, both are `token`
	// see:
	//	https://www.rfc-editor.org/rfc/rfc9110.html#name-methods
	//	https://www.rfc-editor.org/rfc/rfc9110.html#name-field-names
	validMethodCharTable = bytesconv.ValidHeaderFieldNameTable
)

var errMalformedHTTPRequest = errors.New("malformed HTTP request")

// errBothTEAndCL is returned when a request contains both Transfer-Encoding and Content-Length headers.
// This is a potential HTTP request smuggling attack vector.
// See RFC 9112 Section 6.3 Rule 3.
var errBothTEAndCL = errors.New("both Transfer-Encoding and Content-Length headers are present in request: potential HTTP request smuggling")

// errDuplicateCL is returned when a request contains multiple Content-Length headers with different values.
// RFC 9112 Section 6.3 Rule 5 treats this as an unrecoverable error (normalization is no longer permitted).
var errDuplicateCL = errors.New("duplicate Content-Length header with conflicting values")

// errNonChunkedTE is returned when Transfer-Encoding is present but "chunked" is not the final
// transfer coding. Per RFC 9112 Section 6.3 Rule 4, the server MUST respond with 400 Bad Request.
var errNonChunkedTE = errors.New("Transfer-Encoding present but chunked is not the final transfer coding")

// request-line = method SP request-target SP HTTP-version CRLF
func parseFirstLine(h *protocol.RequestHeader, buf []byte) (int, error) {
	b, leftb, err := utils.NextLine(buf)
	if err != nil {
		// errs.ErrNeedMore?
		// check malformed HTTP request before reading more data
		// NOTE:
		//  only check method bytes if errs.ErrNeedMore for closing malformed connections.
		//  for performance concern, it won't be checked in the hot path.
		for i, c := range buf {
			if c == ' ' || i > maxCheckMethodLen {
				break // skip if SP or reach maxCheckMethodLen
			}
			if validMethodCharTable[c] == 0 {
				return 0, errMalformedHTTPRequest
			}
		}
		return 0, err
	}

	// parse method
	n := bytes.IndexByte(b, ' ')
	if n <= 0 {
		return 0, errMalformedHTTPRequest
	}
	h.SetMethodBytes(b[:n])
	b = b[n+1:]

	// parse request-target (uri)
	n = bytes.IndexByte(b, ' ')
	if n <= 0 {
		return 0, errMalformedHTTPRequest
	}
	h.SetRequestURIBytes(b[:n])
	b = b[n+1:]

	// parse http protocol
	switch string(b) {
	case consts.HTTP11: // likely HTTP/1.1
		h.SetProtocol(consts.HTTP11)
	case consts.HTTP10:
		h.SetProtocol(consts.HTTP10)
	default:
		if len(b) < 5 || string(b[:5]) != "HTTP/" {
			return 0, errMalformedHTTPRequest
		}
		// XXX: all other cases are considered to be HTTP/1.0 for safe
		h.SetProtocol(consts.HTTP10)
	}
	return len(buf) - len(leftb), nil
}

// validHeaderFieldValue is equal to httpguts.ValidHeaderFieldValue（shares the same context）
func validHeaderFieldValue(val []byte) bool {
	for _, v := range val {
		if bytesconv.ValidHeaderFieldValueTable[v] == 0 {
			return false
		}
	}
	return true
}

func parseHeaders(h *protocol.RequestHeader, buf []byte) (int, error) {
	h.InitContentLengthWithValue(-2)

	// teSeen tracks whether any Transfer-Encoding header has been seen.
	// Each TE line is validated individually rather than merged — multiple TE lines
	// (e.g. "TE: gzip" then "TE: chunked") are strictly rejected. This is intentional:
	// different proxies merge multi-line headers inconsistently, making it a smuggling vector.
	var teSeen bool

	var s ext.HeaderScanner
	s.B = buf
	s.DisableNormalizing = h.IsDisableNormalizing()
	var err error
	for s.Next() {
		if len(s.Key) > 0 {
			// Spaces between the header key and colon are not allowed.
			// See RFC 7230, Section 3.2.4.
			if bytes.IndexByte(s.Key, ' ') != -1 || bytes.IndexByte(s.Key, '\t') != -1 {
				err = fmt.Errorf("invalid header key %q", s.Key)
				return 0, err
			}

			// Check the invalid chars in header value
			if !validHeaderFieldValue(s.Value) {
				err = fmt.Errorf("invalid header value %q", s.Value)
				return 0, err
			}

			switch s.Key[0] | 0x20 {
			case 'h':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrHost) {
					h.SetHostBytes(s.Value)
					continue
				}
			case 'u':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrUserAgent) {
					h.SetUserAgentBytes(s.Value)
					continue
				}
			case 'c':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentType) {
					h.SetContentTypeBytes(s.Value)
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentLength) {
					if !teSeen {
						var nerr error
						var contentLength int
						if contentLength, nerr = protocol.ParseContentLength(s.Value); nerr != nil {
							if err == nil {
								err = nerr
							}
							h.InitContentLengthWithValue(-2)
						} else {
							// Reject duplicate Content-Length with conflicting values.
							// RFC 9112 Section 6.3 Rule 5 treats mismatched values as an unrecoverable error.
							if h.ContentLength() >= 0 && h.ContentLength() != contentLength {
								return 0, errDuplicateCL
							}
							h.InitContentLengthWithValue(contentLength)
							h.SetContentLengthBytes(s.Value)
						}
					} else {
						// Transfer-Encoding already seen; reject per RFC 9112 Section 6.3 Rule 3.
						return 0, errBothTEAndCL
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
			case 't':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrTransferEncoding) {
					// Any TE + CL combination is a potential HTTP request smuggling vector.
					// Reject per RFC 9112 Section 6.3 Rule 3.
					if h.ContentLength() >= 0 {
						return 0, errBothTEAndCL
					}
					teSeen = true
					// identity means no encoding; ignore it for backward compatibility
					// (removed in RFC 9112 Section 11.2, but still sent by some clients).
					if !bytes.EqualFold(s.Value, bytestr.StrIdentity) {
						// Per RFC 9112 Section 6.3 Rule 4, chunked MUST be the final transfer coding.
						if !isFinalChunked(s.Value) {
							return 0, errNonChunkedTE
						}
						h.InitContentLengthWithValue(-1)
						h.SetArgBytes(bytestr.StrTransferEncoding, bytestr.StrChunked, protocol.ArgsHasValue)
					}
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrTrailer) {
					if nerr := h.Trailer().SetTrailers(s.Value); nerr != nil {
						if err == nil {
							err = nerr
						}
					}
					continue
				}
			}
		}
		h.AddArgBytes(s.Key, s.Value, protocol.ArgsHasValue)
	}

	if s.Err != nil && err == nil {
		err = s.Err
	}
	if err != nil {
		h.SetConnectionClose(true)
		return 0, err
	}

	if h.ContentLength() < 0 {
		h.SetContentLengthBytes(h.ContentLengthBytes()[:0])
	}
	if !h.IsHTTP11() && !h.ConnectionClose() {
		// close connection for non-http/1.1 request unless 'Connection: keep-alive' is set.
		v := h.PeekArgBytes(bytestr.StrConnection)
		h.SetConnectionClose(!ext.HasHeaderValue(v, bytestr.StrKeepAlive))
	}
	return s.HLen, nil
}

// isFinalChunked reports whether "chunked" is the last transfer-coding token in a
// (possibly comma-separated) Transfer-Encoding field value.
// Per RFC 9112 Section 6.1 and Section 6.3 Rule 4, chunked MUST be the final coding.
func isFinalChunked(te []byte) bool {
	te = bytes.TrimRight(te, " \t")
	if i := bytes.LastIndexByte(te, ','); i >= 0 {
		te = bytes.TrimLeft(te[i+1:], " \t")
	}
	return bytes.EqualFold(te, bytestr.StrChunked)
}
