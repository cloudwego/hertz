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
 */

package standard

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/connstate"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/network"
)

// defaultMallocSize is kept for API compatibility with dial.go.
const defaultMallocSize = 4096

type Conn struct {
	c      net.Conn
	br     *bufiox.DefaultReader
	bw     *bufiox.DefaultWriter
	stater connstate.ConnStater
}

func (c *Conn) ToHertzError(err error) error {
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ENOTCONN) {
		return errs.ErrConnectionClosed
	}
	if netErr, ok := err.(*net.OpError); ok && netErr.Timeout() {
		return errs.ErrTimeout
	}

	return err
}

func (c *Conn) SetWriteTimeout(t time.Duration) error {
	if t <= 0 {
		return c.c.SetWriteDeadline(time.Time{})
	}
	return c.c.SetWriteDeadline(time.Now().Add(t))
}

func (c *Conn) SetReadTimeout(t time.Duration) error {
	if t <= 0 {
		return c.c.SetReadDeadline(time.Time{})
	}
	return c.c.SetReadDeadline(time.Now().Add(t))
}

type TLSConn struct {
	Conn
}

// Peek returns the next n bytes without advancing the reader. If Peek returns
// fewer than n bytes, it also returns an error explaining why the read is short.
func (c *Conn) Peek(n int) ([]byte, error) {
	buf, err := c.br.Peek(n)
	if err != nil && buf == nil && n > 0 && c.Len() > 0 {
		buf, _ = c.br.Peek(c.br.Buffered())
		// bufiox readAtLeast converts partial-read+EOF to ErrUnexpectedEOF,
		// but hertz protocol code expects the original io.EOF.
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
	}
	return buf, err
}

// Skip discards the next n bytes.
func (c *Conn) Skip(n int) error {
	if c.Len() < n {
		return errs.NewPrivate("skip[" + strconv.Itoa(n) + "] not enough")
	}
	return c.br.Skip(n)
}

// Release frees internal read buffers.
func (c *Conn) Release() error {
	return c.br.Release(nil)
}

// Len returns the total length of the readable data in the reader.
func (c *Conn) Len() int {
	return c.br.Buffered()
}

// ReadByte is used to read one byte with advancing the read pointer.
func (c *Conn) ReadByte() (byte, error) {
	b, err := c.Peek(1)
	if err != nil {
		return ' ', err
	}
	err = c.Skip(1)
	if err != nil {
		return ' ', err
	}
	return b[0], nil
}

// ReadBinary is used to read next n byte with copy, and the read pointer will be advanced.
func (c *Conn) ReadBinary(n int) ([]byte, error) {
	out := make([]byte, n)
	_, err := c.br.ReadBinary(out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Read implements io.Reader.
func (c *Conn) Read(b []byte) (int, error) {
	return c.br.Read(b)
}

// Write calls Write syscall directly to send data.
// Will flush buffer immediately, for performance considerations use WriteBinary instead.
func (c *Conn) Write(b []byte) (n int, err error) {
	if err = c.Flush(); err != nil {
		return
	}
	return c.c.Write(b)
}

// ReadFrom implements io.ReaderFrom. If the underlying writer
// supports the ReadFrom method, this calls the underlying ReadFrom
// without buffering.
func (c *Conn) ReadFrom(r io.Reader) (n int64, err error) {
	if err = c.Flush(); err != nil {
		return
	}

	if w, ok := c.c.(io.ReaderFrom); ok {
		n, err = w.ReadFrom(r)
		return
	}

	var buf [32 * 1024]byte
	for {
		m, rerr := r.Read(buf[:])
		if m > 0 {
			dst, werr := c.bw.Malloc(m)
			if werr != nil {
				err = werr
				return
			}
			copy(dst, buf[:m])
			n += int64(m)
		}
		if rerr != nil {
			if rerr != io.EOF {
				err = rerr
			}
			return
		}
	}
}

// Close closes the connection
func (c *Conn) Close() error {
	// Close stater first to stop epoll monitoring
	if c.stater != nil {
		c.stater.Close()
		c.stater = nil
	}
	return c.c.Close()
}

// CloseNoResetBuffer closes the connection without reset buffer.
func (c *Conn) CloseNoResetBuffer() error {
	return c.c.Close()
}

// LocalAddr returns the local address of the connection.
func (c *Conn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

// RemoteAddr returns the remote address of the connection.
func (c *Conn) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

// SetDeadline sets the connection deadline.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

// SetReadDeadline sets the read deadline of the connection.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline of the connection.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}

// Malloc will provide a n bytes buffer to send data.
func (c *Conn) Malloc(n int) ([]byte, error) {
	return c.bw.Malloc(n)
}

// WriteBinary will use the user buffer to flush.
// NOTE: Before flush successfully, the buffer b should be valid.
func (c *Conn) WriteBinary(b []byte) (int, error) {
	return c.bw.WriteBinary(b)
}

// Flush will send data to the peer end.
func (c *Conn) Flush() error {
	return c.bw.Flush()
}

func (c *Conn) HandleSpecificError(err error, rip string) (needIgnore bool) {
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		hlog.SystemLogger().Debugf("Go net library error=%s, remoteAddr=%s", err.Error(), rip)
		return true
	}
	return false
}

// SetStater sets the connstate listener for this connection
func (c *Conn) SetStater(stater connstate.ConnStater) {
	c.stater = stater
}

func (c *TLSConn) Handshake() error {
	return c.c.(network.ConnTLSer).Handshake()
}

func (c *TLSConn) ConnectionState() tls.ConnectionState {
	return c.c.(network.ConnTLSer).ConnectionState()
}

func newConn(c net.Conn, _ int) network.Conn {
	return &Conn{
		c:  c,
		br: bufiox.NewDefaultReader(c),
		bw: bufiox.NewDefaultWriter(c),
	}
}

func newTLSConn(c net.Conn, _ int) network.Conn {
	return &TLSConn{
		Conn{
			c:  c,
			br: bufiox.NewDefaultReader(c),
			bw: bufiox.NewDefaultWriter(c),
		},
	}
}
