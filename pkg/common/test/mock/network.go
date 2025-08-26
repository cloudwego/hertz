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

package mock

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"strings"
	"time"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/netpoll"
)

var (
	ErrReadTimeout  = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "read timeout")
	ErrWriteTimeout = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "write timeout")
)

type Conn struct {
	zr       network.Reader
	zw       network.ReadWriter
	wroteLen int

	rtimeout time.Duration
	wtimeout time.Duration
}

type Recorder interface {
	network.Reader
	WroteLen() int
}

func (m *Conn) SetWriteTimeout(t time.Duration) error {
	// TODO implement me
	return nil
}

type SlowReadConn struct {
	*Conn
}

func (m *SlowReadConn) SetWriteTimeout(t time.Duration) error {
	return nil
}

func (m *SlowReadConn) SetReadTimeout(t time.Duration) error {
	m.Conn.rtimeout = t
	return nil
}

func SlowReadDialer(addr string) (network.Conn, error) {
	return NewSlowReadConn(""), nil
}

func SlowWriteDialer(addr string) (network.Conn, error) {
	return NewSlowWriteConn(""), nil
}

func (m *Conn) ReadBinary(n int) (p []byte, err error) {
	return m.zr.(netpoll.Reader).ReadBinary(n)
}

func (m *Conn) Read(b []byte) (n int, err error) {
	return netpoll.NewIOReader(m.zr.(netpoll.Reader)).Read(b)
}

func (m *Conn) Write(b []byte) (n int, err error) {
	return netpoll.NewIOWriter(m.zw.(netpoll.ReadWriter)).Write(b)
}

func (m *Conn) Release() error {
	return nil
}

func (m *Conn) Peek(i int) ([]byte, error) {
	b, err := m.zr.Peek(i)
	if err != nil || len(b) != i {
		if m.rtimeout <= 0 {
			// simulate timeout forever
			select {}
		}
		time.Sleep(m.rtimeout)
		return nil, errs.ErrTimeout
	}
	return b, err
}

func (m *Conn) Skip(n int) error {
	return m.zr.Skip(n)
}

func (m *Conn) ReadByte() (byte, error) {
	return m.zr.ReadByte()
}

func (m *Conn) Len() int {
	return m.zr.Len()
}

func (m *Conn) Malloc(n int) (buf []byte, err error) {
	m.wroteLen += n
	return m.zw.Malloc(n)
}

func (m *Conn) WriteBinary(b []byte) (n int, err error) {
	n, err = m.zw.WriteBinary(b)
	m.wroteLen += n
	return n, err
}

func (m *Conn) Flush() error {
	return m.zw.Flush()
}

func (m *Conn) WriterRecorder() Recorder {
	return &recorder{c: m, Reader: m.zw}
}

func (m *Conn) GetReadTimeout() time.Duration {
	return m.rtimeout
}

func (m *Conn) GetWriteTimeout() time.Duration {
	return m.wtimeout
}

type recorder struct {
	c *Conn
	network.Reader
}

func (r *recorder) WroteLen() int {
	return r.c.wroteLen
}

func (m *SlowReadConn) Peek(i int) ([]byte, error) {
	b, err := m.zr.Peek(i)
	if m.rtimeout > 0 {
		time.Sleep(m.rtimeout)
	} else {
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || len(b) != i {
		return nil, ErrReadTimeout
	}
	return b, err
}

func NewConn(source string) *Conn {
	zr := netpoll.NewReader(strings.NewReader(source))
	zw := netpoll.NewReadWriter(&bytes.Buffer{})

	return &Conn{
		zr: zr,
		zw: zw,
	}
}

type BrokenConn struct {
	*Conn
}

func (o *BrokenConn) Peek(i int) ([]byte, error) {
	return nil, io.ErrUnexpectedEOF
}

func (o *BrokenConn) Read(b []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func (o *BrokenConn) Flush() error {
	return errs.ErrConnectionClosed
}

func NewBrokenConn(source string) *BrokenConn {
	return &BrokenConn{Conn: NewConn(source)}
}

type OneTimeConn struct {
	isRead        bool
	isFlushed     bool
	contentLength int
	*Conn
}

func (o *OneTimeConn) Peek(n int) ([]byte, error) {
	if o.isRead {
		return nil, io.EOF
	}
	return o.Conn.Peek(n)
}

func (o *OneTimeConn) Skip(n int) error {
	if o.isRead {
		return io.EOF
	}
	o.contentLength -= n

	if o.contentLength == 0 {
		o.isRead = true
	}

	return o.Conn.Skip(n)
}

func (o *OneTimeConn) Flush() error {
	if o.isFlushed {
		return errs.ErrConnectionClosed
	}
	o.isFlushed = true
	return o.Conn.Flush()
}

func NewOneTimeConn(source string) *OneTimeConn {
	return &OneTimeConn{isRead: false, isFlushed: false, Conn: NewConn(source), contentLength: len(source)}
}

func NewSlowReadConn(source string) *SlowReadConn {
	return &SlowReadConn{Conn: NewConn(source)}
}

type ErrorReadConn struct {
	*Conn
	errorToReturn error
}

func NewErrorReadConn(err error) *ErrorReadConn {
	return &ErrorReadConn{
		Conn:          NewConn(""),
		errorToReturn: err,
	}
}

func (er *ErrorReadConn) Peek(n int) ([]byte, error) {
	return nil, er.errorToReturn
}

type SlowWriteConn struct {
	*Conn
	writeTimeout time.Duration
}

func (m *SlowWriteConn) SetWriteTimeout(t time.Duration) error {
	m.writeTimeout = t
	return nil
}

func NewSlowWriteConn(source string) *SlowWriteConn {
	return &SlowWriteConn{NewConn(source), 0}
}

func (m *SlowWriteConn) Flush() error {
	err := m.zw.Flush()
	if err == nil {
		time.Sleep(m.writeTimeout)
		return ErrWriteTimeout
	}
	return err
}

func (m *Conn) Close() error {
	return nil
}

func (m *Conn) LocalAddr() net.Addr {
	return nil
}

func (m *Conn) RemoteAddr() net.Addr {
	return nil
}

func (m *Conn) SetDeadline(t time.Time) error {
	m.rtimeout = -time.Since(t)
	m.wtimeout = m.rtimeout
	return nil
}

func (m *Conn) SetReadDeadline(t time.Time) error {
	m.rtimeout = -time.Since(t)
	return nil
}

func (m *Conn) SetWriteDeadline(t time.Time) error {
	panic("implement me")
}

func (m *Conn) Reader() network.Reader {
	return m.zr
}

func (m *Conn) Writer() network.Writer {
	return m.zw
}

func (m *Conn) IsActive() bool {
	panic("implement me")
}

func (m *Conn) SetIdleTimeout(timeout time.Duration) error {
	return nil
}

func (m *Conn) SetReadTimeout(t time.Duration) error {
	m.rtimeout = t
	return nil
}

func (m *Conn) SetOnRequest(on netpoll.OnRequest) error {
	panic("implement me")
}

func (m *Conn) AddCloseCallback(callback netpoll.CloseCallback) error {
	panic("implement me")
}

type StreamConn struct {
	HasReleased bool
	Data        []byte
}

func NewStreamConn() *StreamConn {
	return &StreamConn{
		Data: make([]byte, 1<<15, 1<<16),
	}
}

func (m *StreamConn) Peek(n int) ([]byte, error) {
	if len(m.Data) >= n {
		return m.Data[:n], nil
	}
	if n == 1 {
		m.Data = m.Data[:cap(m.Data)]
		return m.Data[:1], nil
	}
	return nil, errs.NewPublic("not enough data")
}

func (m *StreamConn) Skip(n int) error {
	if len(m.Data) >= n {
		m.Data = m.Data[n:]
		return nil
	}
	return errs.NewPublic("not enough data")
}

func (m *StreamConn) Release() error {
	m.HasReleased = true
	return nil
}

func (m *StreamConn) Len() int {
	return len(m.Data)
}

func (m *StreamConn) ReadByte() (byte, error) {
	panic("implement me")
}

func (m *StreamConn) ReadBinary(n int) (p []byte, err error) {
	panic("implement me")
}

func DialerFun(addr string) (network.Conn, error) {
	return NewConn(""), nil
}

type MockWriter struct {
	w network.Writer

	MockMalloc      func(n int) (buf []byte, err error)
	MockWriteBinary func(b []byte) (n int, err error)
	MockFlush       func() error
}

func NewMockWriter(w network.Writer) *MockWriter {
	return &MockWriter{w: w}
}

func (m *MockWriter) Malloc(n int) (buf []byte, err error) {
	if m.MockMalloc != nil {
		return m.MockMalloc(n)
	}
	return m.w.Malloc(n)
}

func (m *MockWriter) WriteBinary(b []byte) (n int, err error) {
	if m.MockWriteBinary != nil {
		return m.MockWriteBinary(b)
	}
	return m.w.WriteBinary(b)
}

func (m *MockWriter) Flush() error {
	if m.MockFlush != nil {
		return m.MockFlush()
	}
	return m.w.Flush()
}

type TLSConn struct {
	network.Conn

	HandshakeErr error
}

var _ network.ConnTLSer = (*TLSConn)(nil)

func (c *TLSConn) Handshake() error {
	return c.HandshakeErr
}

func (c *TLSConn) ConnectionState() tls.ConnectionState {
	return tls.ConnectionState{}
}

func NewTLSConn(conn network.Conn) *TLSConn {
	return &TLSConn{Conn: conn}
}
