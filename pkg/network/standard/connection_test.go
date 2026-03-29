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
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

// --- helpers ---

func mkConn(data []byte) (*bufConn, *Conn) {
	c := &bufConn{r: bytes.NewReader(data), w: &bytes.Buffer{}}
	return c, newConn(c, 4096).(*Conn)
}

type bufConn struct {
	mockConn
	r io.Reader
	w *bytes.Buffer
}

func (c *bufConn) Read(b []byte) (int, error)  { return c.r.Read(b) }
func (c *bufConn) Write(b []byte) (int, error) { return c.w.Write(b) }

type mockConn struct {
	buffer     bytes.Buffer
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (m *mockConn) Handshake() error                     { return errors.New("not supported") }
func (m *mockConn) ConnectionState() tls.ConnectionState { return tls.ConnectionState{} }
func (m mockConn) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 0
	}
	n := len(b)
	if n > 8192 {
		n = 8192
	}
	return n, nil
}
func (m *mockConn) Write(b []byte) (int, error)     { return m.buffer.Write(b) }
func (m *mockConn) Close() error                    { return errors.New("not supported") }
func (m *mockConn) LocalAddr() net.Addr             { return m.localAddr }
func (m *mockConn) RemoteAddr() net.Addr            { return m.remoteAddr }
func (m *mockConn) SetDeadline(t time.Time) error   { return m.SetWriteDeadline(t) }
func (m *mockConn) SetReadDeadline(time.Time) error { return errors.New("read deadline not supported") }
func (m *mockConn) SetWriteDeadline(time.Time) error {
	return errors.New("write deadline not supported")
}

type errReader struct {
	data []byte
	err  error
	done bool
}

func (r *errReader) Read(b []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(b, r.data), nil
}

type readerFromConn struct {
	mockConn
}

func (c *readerFromConn) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(&c.buffer, r)
}

// --- tests ---

func TestRead(t *testing.T) {
	data := bytes.Repeat([]byte{1}, 10000)
	_, conn := mkConn(data)

	b := make([]byte, 5)
	n, err := conn.Read(b)
	assert.Nil(t, err)
	assert.DeepEqual(t, 5, n)

	b = make([]byte, 20000)
	n, err = conn.Read(b)
	assert.Nil(t, err)
	assert.True(t, n > 0)
}

func TestPeekSkipRelease(t *testing.T) {
	data := bytes.Repeat([]byte{0}, 10000)
	_, conn := mkConn(data)

	b, err := conn.Peek(100)
	assert.Nil(t, err)
	assert.DeepEqual(t, 100, len(b))

	// skip more than buffered fails
	assert.NotNil(t, conn.Skip(conn.Len()+1))

	// skip all succeeds
	assert.Nil(t, conn.Skip(conn.Len()))
	assert.DeepEqual(t, 0, conn.Len())

	// release then peek refills
	conn.Release()
	b, err = conn.Peek(1)
	assert.Nil(t, err)
	assert.DeepEqual(t, 1, len(b))
}

func TestReadByteAndReadBinary(t *testing.T) {
	_, conn := mkConn([]byte("abcdef"))

	rb, err := conn.ReadBinary(3)
	assert.Nil(t, err)
	assert.DeepEqual(t, []byte("abc"), rb)

	by, err := conn.ReadByte()
	assert.Nil(t, err)
	assert.DeepEqual(t, byte('d'), by)
}

func TestWriteAndFlush(t *testing.T) {
	c := &mockConn{}
	conn := newConn(c, 4096).(*Conn)

	buf, _ := conn.Malloc(5)
	copy(buf, []byte("hello"))
	conn.WriteBinary([]byte(" world"))
	assert.Nil(t, conn.Flush())
	assert.DeepEqual(t, "hello world", c.buffer.String())
}

func TestReadFrom(t *testing.T) {
	t.Run("fallback loop", func(t *testing.T) {
		c := &mockConn{}
		conn := newConn(c, 4096)
		n, err := conn.(io.ReaderFrom).ReadFrom(strings.NewReader("hello"))
		assert.Nil(t, err)
		assert.DeepEqual(t, int64(5), n)
		conn.Flush()
		assert.DeepEqual(t, "hello", c.buffer.String())
	})

	t.Run("reader error", func(t *testing.T) {
		readErr := errors.New("disk failure")
		c := &mockConn{}
		conn := newConn(c, 4096)
		n, err := conn.(io.ReaderFrom).ReadFrom(&errReader{data: []byte("partial"), err: readErr})
		assert.DeepEqual(t, readErr, err)
		assert.DeepEqual(t, int64(7), n)
	})

	t.Run("underlying ReaderFrom", func(t *testing.T) {
		c := &readerFromConn{}
		conn := newConn(c, 4096)
		n, err := conn.(io.ReaderFrom).ReadFrom(strings.NewReader("hello"))
		assert.Nil(t, err)
		assert.DeepEqual(t, int64(5), n)
		assert.DeepEqual(t, "hello", c.buffer.String())
	})
}

func TestPeekPartialEOF(t *testing.T) {
	_, conn := mkConn(make([]byte, 100))

	b, err := conn.Peek(100)
	assert.Nil(t, err)
	assert.DeepEqual(t, 100, len(b))

	conn.Skip(10)

	b, err = conn.Peek(100)
	assert.DeepEqual(t, io.EOF, err)
	assert.DeepEqual(t, 90, len(b))
}

func TestConnAddrs(t *testing.T) {
	local := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 80}
	remote := &net.TCPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 9000}
	c := &mockConn{localAddr: local, remoteAddr: remote}
	conn := newConn(c, 4096)
	assert.DeepEqual(t, local, conn.LocalAddr())
	assert.DeepEqual(t, remote, conn.RemoteAddr())
}

func TestSetDeadlines(t *testing.T) {
	c := &mockConn{}
	conn := newConn(c, 4096)
	assert.NotNil(t, conn.SetDeadline(time.Time{}))
	assert.NotNil(t, conn.SetReadDeadline(time.Time{}))
	assert.NotNil(t, conn.SetWriteDeadline(time.Time{}))
}

func TestSetTimeouts(t *testing.T) {
	c := &mockConn{}
	conn := newConn(c, 4096)
	assert.NotNil(t, conn.SetReadTimeout(time.Second))
	assert.NotNil(t, conn.SetReadTimeout(-1))
	assert.NotNil(t, conn.SetWriteTimeout(time.Second))
	assert.NotNil(t, conn.SetWriteTimeout(-1))
}

func TestToHertzError(t *testing.T) {
	conn := &Conn{}
	other := errors.New("other")

	assert.DeepEqual(t, errs.ErrConnectionClosed, conn.ToHertzError(syscall.EPIPE))
	assert.DeepEqual(t, errs.ErrConnectionClosed, conn.ToHertzError(syscall.ENOTCONN))
	assert.DeepEqual(t, errs.ErrTimeout, conn.ToHertzError(&net.OpError{Op: "read", Err: &timeoutErr{}}))
	assert.DeepEqual(t, other, conn.ToHertzError(other))
}

func TestHandleSpecificError(t *testing.T) {
	conn := &Conn{}
	assert.DeepEqual(t, false, conn.HandleSpecificError(nil, ""))
	assert.DeepEqual(t, true, conn.HandleSpecificError(syscall.EPIPE, ""))
}

func TestTLSConn(t *testing.T) {
	c := &mockConn{}
	tc := newTLSConn(c, 4096).(*TLSConn)
	assert.NotNil(t, tc.Handshake())
	assert.DeepEqual(t, tls.ConnectionState{}, tc.ConnectionState())
}

type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string { return m.network }
func (m *mockAddr) String() string  { return m.address }

type timeoutErr struct{}

func (e *timeoutErr) Error() string   { return "timeout" }
func (e *timeoutErr) Timeout() bool   { return true }
func (e *timeoutErr) Temporary() bool { return true }
