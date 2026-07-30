// Copyright 2022 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package standard

import (
	"crypto/tls"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnIsHealthyTCPFastPath(t *testing.T) {
	t.Run("clears an existing read deadline", func(t *testing.T) {
		conn, peer := newStandardTCPPair(t)
		if err := conn.c.SetReadDeadline(time.Now().Add(-time.Second)); err != nil {
			t.Fatalf("set expired read deadline: %v", err)
		}
		if !conn.IsHealthy(5*time.Millisecond, time.Second) {
			t.Fatal("idle TCP connection should be healthy after clearing its prior deadline")
		}
		if _, err := peer.Write([]byte("x")); err != nil {
			t.Fatalf("write after health check: %v", err)
		}
		data, err := conn.Peek(1)
		if err != nil {
			t.Fatalf("peek after health check: %v", err)
		}
		if string(data) != "x" {
			t.Fatalf("read after health check: got %q, want %q", data, "x")
		}
	})

	t.Run("pending data is rejected without consumption", func(t *testing.T) {
		conn, peer := newStandardTCPPair(t)
		if _, err := peer.Write([]byte("x")); err != nil {
			t.Fatalf("write peer data: %v", err)
		}
		waitForStandardTCPUnhealthy(t, conn)
		data, err := conn.Peek(1)
		if err != nil {
			t.Fatalf("peek after health check: %v", err)
		}
		if string(data) != "x" {
			t.Fatalf("health check consumed or changed data: %q", data)
		}
	})

	t.Run("peer FIN is rejected", func(t *testing.T) {
		conn, peer := newStandardTCPPair(t)
		if err := peer.Close(); err != nil {
			t.Fatalf("close peer: %v", err)
		}
		waitForStandardTCPUnhealthy(t, conn)
	})

	t.Run("peer reset is rejected", func(t *testing.T) {
		conn, peer := newStandardTCPPair(t)
		if err := peer.SetLinger(0); err != nil {
			t.Fatalf("set peer linger: %v", err)
		}
		if err := peer.Close(); err != nil {
			t.Fatalf("reset peer: %v", err)
		}
		waitForStandardTCPUnhealthy(t, conn)
	})

	t.Run("local close is rejected", func(t *testing.T) {
		conn, _ := newStandardTCPPair(t)
		if err := conn.Close(); err != nil {
			t.Fatalf("close connection: %v", err)
		}
		if conn.IsHealthy(5*time.Millisecond, time.Second) {
			t.Fatal("locally closed TCP connection should not be healthy")
		}
	})
}

func TestConnIsHealthyTLSUsesTimedFallback(t *testing.T) {
	certData, keyData, err := generateTestCertificate("")
	if err != nil {
		t.Fatalf("generate certificate: %v", err)
	}
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	serverResult := make(chan *tls.Conn, 1)
	serverErr := make(chan error, 1)
	go func() {
		peer, err := listener.AcceptTCP()
		if err != nil {
			serverErr <- err
			return
		}
		serverTLS := tls.Server(peer, &tls.Config{Certificates: []tls.Certificate{cert}})
		if err := serverTLS.Handshake(); err != nil {
			_ = serverTLS.Close()
			serverErr <- err
			return
		}
		serverResult <- serverTLS
	}()

	clientTCP, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	trackingConn := &readDeadlineTrackingConn{Conn: clientTCP}
	clientTLS := tls.Client(trackingConn, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // loopback test certificate
	if err := clientTLS.Handshake(); err != nil {
		_ = clientTLS.Close()
		t.Fatalf("client handshake: %v", err)
	}

	var serverTLS *tls.Conn
	select {
	case serverTLS = <-serverResult:
	case err := <-serverErr:
		_ = clientTLS.Close()
		t.Fatalf("server handshake: %v", err)
	case <-time.After(time.Second):
		_ = clientTLS.Close()
		t.Fatal("server handshake timed out")
	}
	defer serverTLS.Close()
	defer clientTLS.Close()

	trackingConn.readDeadlineCalls.Store(0)
	conn := newTLSConn(clientTLS, 0).(*TLSConn)
	if !conn.IsHealthy(time.Millisecond, time.Second) {
		t.Fatal("idle TLS connection should remain healthy through fallback")
	}
	if calls := trackingConn.readDeadlineCalls.Load(); calls != 2 {
		t.Fatalf("TLS read deadline calls: got %d, want 2 for set and reset", calls)
	}
	if _, err := serverTLS.Write([]byte("x")); err != nil {
		t.Fatalf("write after TLS health check: %v", err)
	}
	data, err := conn.Peek(1)
	if err != nil {
		t.Fatalf("peek after TLS health check: %v", err)
	}
	if string(data) != "x" {
		t.Fatalf("read after TLS health check: got %q, want %q", data, "x")
	}
}

func TestConnIsHealthyUnsupportedConnUsesTimedFallback(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	trackingConn := &readDeadlineTrackingConn{Conn: clientConn}
	conn := newConn(trackingConn, 0).(*Conn)
	if !conn.IsHealthy(time.Millisecond, time.Second) {
		t.Fatal("idle unsupported connection should remain healthy through fallback")
	}
	if calls := trackingConn.readDeadlineCalls.Load(); calls != 2 {
		t.Fatalf("read deadline calls: got %d, want 2 for set and reset", calls)
	}
	writeDone := make(chan error, 1)
	go func() {
		_, err := serverConn.Write([]byte("x"))
		writeDone <- err
	}()
	data, err := conn.Peek(1)
	if err != nil {
		t.Fatalf("peek after fallback health check: %v", err)
	}
	if string(data) != "x" {
		t.Fatalf("read after fallback health check: got %q, want %q", data, "x")
	}
	if err := <-writeDone; err != nil {
		t.Fatalf("write after fallback health check: %v", err)
	}
}

func BenchmarkConnIsHealthyTCP(b *testing.B) {
	conn, _ := newStandardTCPPair(b)
	if !conn.IsHealthy(5*time.Millisecond, time.Second) {
		b.Fatal("warmup rejected idle TCP connection")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !conn.IsHealthy(5*time.Millisecond, time.Second) {
			b.Fatal("idle TCP connection was rejected")
		}
	}
}

type readDeadlineTrackingConn struct {
	net.Conn
	readDeadlineCalls atomic.Int32
}

func (c *readDeadlineTrackingConn) SetReadDeadline(deadline time.Time) error {
	c.readDeadlineCalls.Add(1)
	return c.Conn.SetReadDeadline(deadline)
}

func newStandardTCPPair(tb testing.TB) (*Conn, *net.TCPConn) {
	tb.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}

	accepted := make(chan *net.TCPConn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		peer, err := listener.AcceptTCP()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- peer
	}()

	clientConn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	if err != nil {
		_ = listener.Close()
		tb.Fatalf("dial: %v", err)
	}

	var peer *net.TCPConn
	select {
	case peer = <-accepted:
	case err := <-acceptErr:
		_ = clientConn.Close()
		_ = listener.Close()
		tb.Fatalf("accept: %v", err)
	case <-time.After(time.Second):
		_ = clientConn.Close()
		_ = listener.Close()
		tb.Fatal("accept timed out")
	}

	conn := newConn(clientConn, 0).(*Conn)
	tb.Cleanup(func() {
		_ = conn.Close()
		_ = peer.Close()
		_ = listener.Close()
	})
	return conn, peer
}

func waitForStandardTCPUnhealthy(tb testing.TB, conn *Conn) {
	tb.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !conn.IsHealthy(5*time.Millisecond, time.Second) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	tb.Fatal("TCP connection remained healthy")
}
