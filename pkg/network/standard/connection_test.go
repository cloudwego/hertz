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

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

// bufConn wraps an io.Reader and *bytes.Buffer as a net.Conn for deterministic tests.
type bufConn struct {
	mockConn
	r io.Reader
	w *bytes.Buffer
}

func newBufConn(data []byte) *bufConn {
	return &bufConn{r: bytes.NewReader(data), w: &bytes.Buffer{}}
}

func (c *bufConn) Read(b []byte) (int, error)  { return c.r.Read(b) }
func (c *bufConn) Write(b []byte) (int, error) { return c.w.Write(b) }

func TestRead(t *testing.T) {
	data := bytes.Repeat([]byte{1}, 10000)
	c := newBufConn(data)
	conn := newConn(c, 0)

	// small read
	b := make([]byte, 5)
	n, err := conn.Read(b)
	assert.Nil(t, err)
	assert.DeepEqual(t, 5, n)
	assert.DeepEqual(t, bytes.Repeat([]byte{1}, 5), b)

	// another small read
	b = make([]byte, 10)
	n, err = conn.Read(b)
	assert.Nil(t, err)
	assert.DeepEqual(t, 10, n)

	// large read returns available data
	b = make([]byte, 20000)
	n, err = conn.Read(b)
	assert.Nil(t, err)
	assert.True(t, n > 0)
	assert.True(t, n <= 9985) // at most 10000 - 15 already read
}

func TestReadFromHasBufferAvailable(t *testing.T) {
	preData := []byte("head data")
	rawData := strings.Repeat("helloworld", 1)
	tailData := []byte("tail data")
	data := strings.NewReader(rawData)
	c := &mockConn{}
	conn := newConn(c, 4096)

	// WriteBinary will malloc a buffer if no buffer available.
	_, err0 := conn.WriteBinary(preData)
	assert.Nil(t, err0)

	reader, ok := conn.(io.ReaderFrom)
	assert.True(t, ok)

	l, err := reader.ReadFrom(data)
	assert.Nil(t, err)
	assert.DeepEqual(t, len(rawData), int(l))

	_, err1 := conn.WriteBinary(tailData)
	assert.Nil(t, err1)

	err2 := conn.Flush()
	assert.Nil(t, err2)
	assert.DeepEqual(t, string(preData)+rawData+string(tailData), c.buffer.String())
}

func TestReadFromNoBufferAvailable(t *testing.T) {
	rawData := strings.Repeat("helloworld", 1)
	tailData := []byte("tail data")
	data := strings.NewReader(rawData)
	c := &mockConn{}
	conn := newConn(c, 4096)
	reader, ok := conn.(io.ReaderFrom)
	assert.True(t, ok)

	l, err := reader.ReadFrom(data)
	assert.Nil(t, err)
	assert.DeepEqual(t, len(rawData), int(l))

	_, err1 := conn.WriteBinary(tailData)
	assert.Nil(t, err1)

	err2 := conn.Flush()
	assert.Nil(t, err2)

	assert.DeepEqual(t, rawData+string(tailData), c.buffer.String())
}

func TestPeekRelease(t *testing.T) {
	data := bytes.Repeat([]byte{0}, 100000)
	c := newBufConn(data)
	conn := newConn(c, 0)

	// peek small
	b, err := conn.Peek(1)
	assert.Nil(t, err)
	assert.DeepEqual(t, 1, len(b))

	// peek large
	b, err = conn.Peek(10000)
	assert.Nil(t, err)
	assert.DeepEqual(t, 10000, len(b))

	// skip more than available should error
	l := conn.Len()
	assert.True(t, l >= 10000)
	err = conn.Skip(l + 1)
	assert.NotNil(t, err)

	// skip all
	err = conn.Skip(l)
	assert.Nil(t, err)
	assert.DeepEqual(t, 0, conn.Len())

	// release and peek again
	conn.Release()
	b, err = conn.Peek(1)
	assert.Nil(t, err)
	assert.DeepEqual(t, 1, len(b))
	assert.True(t, conn.Len() > 0)
}

func TestReadBytes(t *testing.T) {
	data := make([]byte, 1000)
	data[0] = 'a'
	data[1] = 'b'
	c := newBufConn(data)
	conn := newConn(c, 0)

	// Peek returns view into buffer
	b, _ := conn.Peek(1)
	assert.DeepEqual(t, byte('a'), b[0])

	// ReadBinary returns independent copy
	rb, _ := conn.ReadBinary(1)
	assert.DeepEqual(t, byte('a'), rb[0])

	// mutating peek slice doesn't affect the copy
	b[0] = 'z'
	assert.DeepEqual(t, byte('a'), rb[0])

	// ReadByte reads next unconsumed byte
	by, err := conn.ReadByte()
	assert.Nil(t, err)
	assert.DeepEqual(t, byte('b'), by)
}

func TestWriteLogic(t *testing.T) {
	c := &mockConn{}
	conn := newConn(c, 0)

	// Malloc and fill
	buf, err := conn.Malloc(5)
	assert.Nil(t, err)
	copy(buf, []byte("hello"))

	// WriteBinary
	_, err = conn.WriteBinary([]byte(" world"))
	assert.Nil(t, err)

	// Flush sends all data
	err = conn.Flush()
	assert.Nil(t, err)
	assert.DeepEqual(t, "hello world", c.buffer.String())

	// Multiple Malloc + Flush
	c.buffer.Reset()
	buf, _ = conn.Malloc(3)
	copy(buf, []byte("foo"))
	buf, _ = conn.Malloc(3)
	copy(buf, []byte("bar"))
	conn.Flush()
	assert.DeepEqual(t, "foobar", c.buffer.String())
}

func TestInitializeConn(t *testing.T) {
	c := mockConn{
		localAddr: &mockAddr{
			network: "tcp",
			address: "192.168.0.10:80",
		},
		remoteAddr: &mockAddr{
			network: "tcp",
			address: "192.168.0.20:80",
		},
	}
	conn := newConn(&c, 8192)
	// check the assignment
	assert.DeepEqual(t, errors.New("conn: write deadline not supported"), conn.SetDeadline(time.Time{}))
	assert.DeepEqual(t, errors.New("conn: read deadline not supported"), conn.SetReadDeadline(time.Time{}))
	assert.DeepEqual(t, errors.New("conn: write deadline not supported"), conn.SetWriteDeadline(time.Time{}))
	assert.DeepEqual(t, errors.New("conn: read deadline not supported"), conn.SetReadTimeout(time.Duration(1)*time.Second))
	assert.DeepEqual(t, errors.New("conn: read deadline not supported"), conn.SetReadTimeout(time.Duration(-1)*time.Second))
	assert.DeepEqual(t, errors.New("conn: method not supported"), conn.Close())
	assert.DeepEqual(t, &mockAddr{network: "tcp", address: "192.168.0.10:80"}, conn.LocalAddr())
	assert.DeepEqual(t, &mockAddr{network: "tcp", address: "192.168.0.20:80"}, conn.RemoteAddr())
}

func TestInitializeTLSConn(t *testing.T) {
	c := mockConn{}
	tlsConn := newTLSConn(&c, 8192).(*TLSConn)
	assert.DeepEqual(t, errors.New("conn: method not supported"), tlsConn.Handshake())
	assert.DeepEqual(t, tls.ConnectionState{}, tlsConn.ConnectionState())
}

func TestHandleSpecificError(t *testing.T) {
	conn := &Conn{}
	assert.DeepEqual(t, false, conn.HandleSpecificError(nil, ""))
	assert.DeepEqual(t, true, conn.HandleSpecificError(syscall.EPIPE, ""))
}

func TestFillReturnErrAndN(t *testing.T) {
	data := make([]byte, 4099)
	c := newBufConn(data)
	conn := newConn(c, 0)

	b, err := conn.Peek(4099)
	assert.Nil(t, err)
	assert.DeepEqual(t, 4099, len(b))

	conn.Skip(10)
	b, err = conn.Peek(4099)
	assert.DeepEqual(t, io.EOF, err)
	assert.DeepEqual(t, 4089, len(b))
}

type mockConn struct {
	buffer     bytes.Buffer
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (m *mockConn) Handshake() error {
	return errors.New("conn: method not supported")
}

func (m *mockConn) ConnectionState() tls.ConnectionState {
	return tls.ConnectionState{}
}

func (m mockConn) Read(b []byte) (n int, err error) {
	length := len(b)
	for i := 0; i < length; i++ {
		b[i] = 0
	}

	if length > 8192 {
		return 8192, err
	}
	if len(b) < 1024 {
		return 100, err
	}
	if len(b) < 5000 {
		return 4096, err
	}

	return 4099, err
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return m.buffer.Write(b)
}

func (m *mockConn) Close() error {
	return errors.New("conn: method not supported")
}

func (m *mockConn) LocalAddr() net.Addr {
	return m.localAddr
}

func (m *mockConn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

func (m *mockConn) SetDeadline(deadline time.Time) error {
	if err := m.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return m.SetWriteDeadline(deadline)
}

func (m *mockConn) SetReadDeadline(deadline time.Time) error {
	return errors.New("conn: read deadline not supported")
}

func (m *mockConn) SetWriteDeadline(deadline time.Time) error {
	return errors.New("conn: write deadline not supported")
}

type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string {
	return m.network
}

func (m *mockAddr) String() string {
	return m.address
}
