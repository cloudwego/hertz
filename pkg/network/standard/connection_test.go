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
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestRead(t *testing.T) {
	c := mockConn{}
	conn := newConn(&c, 4096)
	// test read small data
	b := make([]byte, 1)
	conn.Read(b)
	if conn.Len() != 4095 {
		t.Errorf("unexpected conn.Len: %v, expected 4095", conn.Len())
	}

	// test read small data again
	conn.Read(b)
	if conn.Len() != 4094 {
		t.Errorf("unexpected conn.Len: %v, expected 4094", conn.Len())
	}

	// test read large data
	b = make([]byte, 10000)
	n, _ := conn.Read(b)
	if n != 4094 {
		t.Errorf("unexpected n: %v, expected 4094", n)
	}

	// test read large data again
	n, _ = conn.Read(b)
	if n != 8192 {
		t.Errorf("unexpected n: %v, expected 4094", n)
	}
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
	c := mockConn{}
	conn := newConn(&c, 4096)
	b, _ := conn.Peek(1)
	if len(b) != 1 {
		t.Errorf("unexpected len(b): %v, expected 1", len(b))
	}

	b, _ = conn.Peek(10000)
	if len(b) != 10000 {
		t.Errorf("unexpected len(b): %v, expected 10000", len(b))
	}

	if conn.Len() != 12288 {
		t.Errorf("unexpected conn.Len: %v, expected 12288", conn.Len())
	}
	err := conn.Skip(12289)
	if err == nil {
		t.Errorf("unexpected no error, expected link buffer skip[12289] not enough")
	}
	conn.Skip(12288)
	if conn.Len() != 0 {
		t.Errorf("unexpected conn.Len: %v, expected 2287", conn.Len())
	}

	// test reuse buffer
	conn.Release()
	b, _ = conn.Peek(1)
	if len(b) != 1 {
		t.Errorf("unexpected len(b): %v, expected 1", len(b))
	}
	if conn.Len() != 8192 {
		t.Errorf("unexpected conn.Len: %v, expected 8192", conn.Len())
	}

	// test cross node
	b, _ = conn.Peek(100000000)
	if len(b) != 100000000 {
		t.Errorf("unexpected len(b): %v, expected 1", len(b))
	}
	conn.Skip(100000000)
	conn.Release()

	// test maxSize
	if conn.(*Conn).maxSize != 524288 {
		t.Errorf("unexpected maxSize: %v, expected 524288", conn.(*Conn).maxSize)
	}
}

func TestReadBytes(t *testing.T) {
	c := mockConn{}
	conn := newConn(&c, 4096)
	b, _ := conn.Peek(1)
	if len(b) != 1 {
		t.Errorf("unexpected len(b): %v, expected 1", len(b))
	}
	b[0] = 'a'
	peekByte, _ := conn.Peek(1)
	if peekByte[0] != 'a' {
		t.Errorf("unexpected bb[0]: %v, expected a", peekByte[0])
	}
	if conn.Len() != 4096 {
		t.Errorf("unexpected conn.Len: %v, expected 4096", conn.Len())
	}

	readBinary, _ := conn.ReadBinary(1)
	if readBinary[0] != 'a' {
		t.Errorf("unexpected readBinary[0]: %v, expected a", readBinary[0])
	}
	b[0] = 'b'
	if readBinary[0] != 'a' {
		t.Errorf("unexpected readBinary[0]: %v, expected a", readBinary[0])
	}
	bbb, _ := conn.ReadByte()
	if bbb != 0 {
		t.Errorf("unexpected bbb: %v, expected nil", bbb)
	}
	if conn.Len() != 4094 {
		t.Errorf("unexpected conn.Len: %v, expected 4094", conn.Len())
	}
}

func TestWriteLogic(t *testing.T) {
	c := mockConn{}
	conn := newConn(&c, 4096)
	conn.Malloc(8190)
	connection := conn.(*Conn)
	// test left buffer
	if connection.outputBuffer.len != 2 {
		t.Errorf("unexpected Len: %v, expected 2", connection.outputBuffer.len)
	}
	// test malloc next node and left buffer
	conn.Malloc(8190)
	if connection.outputBuffer.len != 2 {
		t.Errorf("unexpected Len: %v, expected 2", connection.outputBuffer.len)
	}
	conn.Flush()
	if connection.outputBuffer.head != connection.outputBuffer.write {
		t.Errorf("outputBuffer head != outputBuffer read")
	}
	conn.Malloc(8190)
	if connection.outputBuffer.len != 2 {
		t.Errorf("unexpected Len: %v, expected 2", connection.outputBuffer.len)
	}
	if connection.outputBuffer.head != connection.outputBuffer.write {
		t.Errorf("outputBuffer head != outputBuffer read")
	}
	// test readOnly
	b := make([]byte, 4096)
	conn.WriteBinary(b)
	conn.Flush()
	conn.Malloc(2)
	if connection.outputBuffer.head == connection.outputBuffer.write {
		t.Errorf("outputBuffer head == outputBuffer read")
	}
	// test reuse outputBuffer
	b = make([]byte, 2)
	conn.WriteBinary(b)
	conn.Flush()
	conn.Malloc(2)
	if connection.outputBuffer.head != connection.outputBuffer.write {
		t.Errorf("outputBuffer head != outputBuffer read")
	}
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
	if length > 8192 {
		return 8192, nil
	}
	if len(b) < 1024 {
		return 100, nil
	}
	if len(b) < 5000 {
		return 4096, nil
	}

	return 4099, nil
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
