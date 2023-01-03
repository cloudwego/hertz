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

package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"
)

// Reader is for buffered Reader
type Reader interface {
	// Peek returns the next n bytes without advancing the reader.
	Peek(n int) ([]byte, error)

	// Skip discards the next n bytes.
	Skip(n int) error

	// Release the memory space occupied by all read slices. This method needs to be executed actively to
	// recycle the memory after confirming that the previously read data is no longer in use.
	// After invoking Release, the slices obtained by the method such as Peek will
	// become an invalid address and cannot be used anymore.
	Release() error

	// Len returns the total length of the readable data in the reader.
	Len() int

	// ReadByte is used to read one byte with advancing the read pointer.
	ReadByte() (byte, error)

	// ReadBinary is used to read next n byte with copy, and the read pointer will be advanced.
	ReadBinary(n int) (p []byte, err error)
}

type Writer interface {
	// Malloc will provide a n bytes buffer to send data.
	Malloc(n int) (buf []byte, err error)

	// WriteBinary will use the user buffer to flush.
	// NOTE: Before flush successfully, the buffer b should be valid.
	WriteBinary(b []byte) (n int, err error)

	// Flush will send data to the peer end.
	Flush() error
}

type ReadWriter interface {
	Reader
	Writer
}

type Conn interface {
	net.Conn
	Reader
	Writer

	// SetReadTimeout should work for every Read process
	SetReadTimeout(t time.Duration) error
	SetWriteTimeout(t time.Duration) error
}

type ConnTLSer interface {
	Handshake() error
	ConnectionState() tls.ConnectionState
}

type HandleSpecificError interface {
	HandleSpecificError(err error, rip string) (needIgnore bool)
}

type ErrorNormalization interface {
	ToHertzError(err error) error
}

type DialFunc func(addr string) (Conn, error)

/****************** Stream-based connection *******************/

// StreamConn is interface for stream-based connection abstraction.
type StreamConn interface {
	GetRawConnection() interface{}
	// HandshakeComplete blocks until the handshake completes (or fails).
	HandshakeComplete() context.Context
	// GetVersion returns the version of the protocol used by the connection.
	GetVersion() uint32
	// CloseWithError closes the connection with an error.
	// The error string will be sent to the peer.
	CloseWithError(err ApplicationError, errMsg string) error
	// LocalAddr returns the local address.
	LocalAddr() net.Addr
	// RemoteAddr returns the address of the peer.
	RemoteAddr() net.Addr
	// The context is cancelled when the connection is closed.
	Context() context.Context
	// Streamer is the interface for stream operations.
	Streamer
}

type Streamer interface {
	// AcceptStream returns the next stream opened by the peer, blocking until one is available.
	// If the connection was closed due to a timeout, the error satisfies
	// the net.Error interface, and Timeout() will be true.
	AcceptStream(context.Context) (Stream, error)
	// AcceptUniStream returns the next unidirectional stream opened by the peer, blocking until one is available.
	// If the connection was closed due to a timeout, the error satisfies
	// the net.Error interface, and Timeout() will be true.
	AcceptUniStream(context.Context) (ReceiveStream, error)
	// OpenStream opens a new bidirectional QUIC stream.
	// There is no signaling to the peer about new streams:
	// The peer can only accept the stream after data has been sent on the stream.
	// If the error is non-nil, it satisfies the net.Error interface.
	// When reaching the peer's stream limit, err.Temporary() will be true.
	// If the connection was closed due to a timeout, Timeout() will be true.
	OpenStream() (Stream, error)
	// OpenStreamSync opens a new bidirectional QUIC stream.
	// It blocks until a new stream can be opened.
	// If the error is non-nil, it satisfies the net.Error interface.
	// If the connection was closed due to a timeout, Timeout() will be true.
	OpenStreamSync(context.Context) (Stream, error)
	// OpenUniStream opens a new outgoing unidirectional QUIC stream.
	// If the error is non-nil, it satisfies the net.Error interface.
	// When reaching the peer's stream limit, Temporary() will be true.
	// If the connection was closed due to a timeout, Timeout() will be true.
	OpenUniStream() (SendStream, error)
	// OpenUniStreamSync opens a new outgoing unidirectional QUIC stream.
	// It blocks until a new stream can be opened.
	// If the error is non-nil, it satisfies the net.Error interface.
	// If the connection was closed due to a timeout, Timeout() will be true.
	OpenUniStreamSync(context.Context) (SendStream, error)
}

type Stream interface {
	ReceiveStream
	SendStream
}

// ReceiveStream is the interface for receiving data on a stream.
type ReceiveStream interface {
	StreamID() int64
	io.Reader

	// CancelRead aborts receiving on this stream.
	// It will ask the peer to stop transmitting stream data.
	// Read will unblock immediately, and future Read calls will fail.
	// When called multiple times or after reading the io.EOF it is a no-op.
	CancelRead(err ApplicationError)

	// SetReadDeadline sets the deadline for future Read calls and
	// any currently-blocked Read call.
	// A zero value for t means Read will not time out.
	SetReadDeadline(t time.Time) error
}

// SendStream is the interface for sending data on a stream.
type SendStream interface {
	StreamID() int64
	// Writer writes data to the stream.
	// Write can be made to time out and return a net.Error with Timeout() == true
	// after a fixed time limit; see SetDeadline and SetWriteDeadline.
	// If the stream was canceled by the peer, the error implements the StreamError
	// interface, and Canceled() == true.
	// If the connection was closed due to a timeout, the error satisfies
	// the net.Error interface, and Timeout() will be true.
	io.Writer
	// CancelWrite aborts sending on this stream.
	// Data already written, but not yet delivered to the peer is not guaranteed to be delivered reliably.
	// Write will unblock immediately, and future calls to Write will fail.
	// When called multiple times or after closing the stream it is a no-op.
	CancelWrite(err ApplicationError)
	// Closer closes the write-direction of the stream.
	// Future calls to Write are not permitted after calling Close.
	// It must not be called concurrently with Write.
	// It must not be called after calling CancelWrite.
	io.Closer

	// The Context is canceled as soon as the write-side of the stream is closed.
	// This happens when Close() or CancelWrite() is called, or when the peer
	// cancels the read-side of their stream.
	Context() context.Context
	// SetWriteDeadline sets the deadline for future Write calls
	// and any currently-blocked Write call.
	// Even if write times out, it may return n > 0, indicating that
	// some data was successfully written.
	// A zero value for t means Write will not time out.
	SetWriteDeadline(t time.Time) error
}

type ApplicationError interface {
	ErrCode() uint64
	fmt.Stringer
}
