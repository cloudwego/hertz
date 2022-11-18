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
	"crypto/tls"
	"net"
	"time"
)

type Dialer interface {
	// DialConnection is used to dial the peer end.
	DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn Conn, err error)

	// DialTimeout is used to dial the peer end with a timeout.
	//
	// NOTE: Not recommended to use this function. Just for compatibility.
	DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error)

	// AddTLS will transfer a common connection to a tls connection.
	AddTLS(conn Conn, tlsConfig *tls.Config) (Conn, error)
}

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
