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

package netpoll

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/netpoll"
)

func TestReadBytes(t *testing.T) {
	c := &mockConn{[]byte("a"), nil, 0}
	conn := newConn(c)
	assert.DeepEqual(t, 1, conn.Len())

	b, _ := conn.Peek(1)
	assert.DeepEqual(t, []byte{'a'}, b)

	readByte, _ := conn.ReadByte()
	assert.DeepEqual(t, byte('a'), readByte)

	_, err := conn.ReadByte()
	assert.DeepEqual(t, errors.New("readByte error: index out of range"), err)

	c = &mockConn{[]byte("bcd"), nil, 0}
	conn = newConn(c)

	readBinary, _ := conn.ReadBinary(2)
	assert.DeepEqual(t, []byte{'b', 'c'}, readBinary)

	_, err = conn.ReadBinary(2)
	assert.DeepEqual(t, errors.New("readBinary error: index out of range"), err)
}

func TestPeekRelease(t *testing.T) {
	c := &mockConn{[]byte("abcdefg"), nil, 0}
	conn := newConn(c)

	// release the buf
	conn.Release()
	_, err := conn.Peek(1)
	assert.DeepEqual(t, errors.New("peek error"), err)

	assert.DeepEqual(t, errors.New("skip error"), conn.Skip(2))
}

func TestWriteLogin(t *testing.T) {
	c := &mockConn{nil, []byte("abcdefg"), 0}
	conn := newConn(c)
	buf, _ := conn.Malloc(10)
	assert.DeepEqual(t, 10, len(buf))
	n, _ := conn.WriteBinary([]byte("abcdefg"))
	assert.DeepEqual(t, 7, n)
	assert.DeepEqual(t, errors.New("flush error"), conn.Flush())
}

func TestHandleSpecificError(t *testing.T) {
	conn := &Conn{}
	assert.DeepEqual(t, false, conn.HandleSpecificError(nil, ""))
	assert.DeepEqual(t, true, conn.HandleSpecificError(netpoll.ErrConnClosed, ""))
}

type mockConn struct {
	readBuf  []byte
	writeBuf []byte
	// index for the first readable byte in readBuf
	off int
}

func (m *mockConn) SetWriteTimeout(timeout time.Duration) error {
	//TODO implement me
	panic("implement me")
}

// mockConn's methods is simplified for unit test
// Peek returns the next n bytes without advancing the reader
func (m *mockConn) Peek(n int) (b []byte, err error) {
	if m.off+n-1 < len(m.readBuf) {
		return m.readBuf[m.off : m.off+n], nil
	}
	return nil, errors.New("peek error")
}

// Skip discards the next n bytes
func (m *mockConn) Skip(n int) error {
	if m.off+n < len(m.readBuf) {
		m.off += n
		return nil
	}
	return errors.New("skip error")
}

// Release the memory space occupied by all read slices
func (m *mockConn) Release() error {
	m.readBuf = nil
	m.off = 0
	return nil
}

// Len returns the total length of the readable data in the reader
func (m *mockConn) Len() int {
	return len(m.readBuf) - m.off
}

// ReadByte is used to read one byte with advancing the read pointer
func (m *mockConn) ReadByte() (byte, error) {
	if m.off < len(m.readBuf) {
		m.off++
		return m.readBuf[m.off-1], nil
	}
	return 0, errors.New("readByte error: index out of range")
}

// ReadBinary is used to read next n byte with copy, and the read pointer will be advanced
func (m *mockConn) ReadBinary(n int) (b []byte, err error) {
	if m.off+n < len(m.readBuf) {
		m.off += n
		return m.readBuf[m.off-n : m.off], nil
	}
	return nil, errors.New("readBinary error: index out of range")
}

// Malloc will provide a n bytes buffer to send data
func (m *mockConn) Malloc(n int) (buf []byte, err error) {
	m.writeBuf = make([]byte, n)
	return m.writeBuf, nil
}

// WriteBinary will use the user buffer to flush
func (m *mockConn) WriteBinary(b []byte) (n int, err error) {
	return len(b), nil
}

// Flush will send data to the peer end
func (m *mockConn) Flush() error {
	return errors.New("flush error")
}

func (m *mockConn) HandleSpecificError(err error, rip string) (needIgnore bool) {
	panic("implement me")
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	panic("implement me")
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	panic("implement me")
}

func (m *mockConn) Close() error {
	panic("implement me")
}

func (m *mockConn) LocalAddr() net.Addr {
	panic("implement me")
}

func (m *mockConn) RemoteAddr() net.Addr {
	panic("implement me")
}

func (m *mockConn) SetDeadline(deadline time.Time) error {
	panic("implement me")
}

func (m *mockConn) SetReadDeadline(deadline time.Time) error {
	panic("implement me")
}

func (m *mockConn) SetWriteDeadline(deadline time.Time) error {
	panic("implement me")
}

func (m *mockConn) Reader() netpoll.Reader {
	panic("implement me")
}

func (m *mockConn) Writer() netpoll.Writer {
	panic("implement me")
}

func (m *mockConn) IsActive() bool {
	panic("implement me")
}

func (m *mockConn) SetReadTimeout(timeout time.Duration) error {
	panic("implement me")
}

func (m *mockConn) SetIdleTimeout(timeout time.Duration) error {
	panic("implement me")
}

func (m *mockConn) SetOnRequest(on netpoll.OnRequest) error {
	panic("implement me")
}

func (m *mockConn) AddCloseCallback(callback netpoll.CloseCallback) error {
	panic("implement me")
}
