// Copyright 2026 CloudWeGo Authors
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
//
//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package standard

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/connstate"

	"github.com/cloudwego/hertz/pkg/common/config"
)

// TestConnCloseWithStater tests that Close properly closes the stater
func TestConnCloseWithStater(t *testing.T) {
	c := &mockConn{
		localAddr: &mockAddr{
			network: "tcp",
			address: "127.0.0.1:8080",
		},
		remoteAddr: &mockAddr{
			network: "tcp",
			address: "127.0.0.1:12345",
		},
	}
	conn := newConn(c, 4096).(*Conn)

	// Create a mock stater
	mockStater := &mockConnStater{}
	conn.SetStater(mockStater)

	// Close the connection - mockConn.Close() returns error, but stater should still be closed
	_ = conn.Close()

	// After Close, stater should be nil
	if conn.stater != nil {
		t.Errorf("Stater should be nil after Close, got %v", conn.stater)
	}

	// MockStater's Close method should have been called
	if !mockStater.closed {
		t.Error("MockStater's Close method was not called")
	}
}

// TestConnstateConnectionCloseDetection tests the connstate-based connection close detection
func TestConnstateConnectionCloseDetection(t *testing.T) {
	// Use net.Pipe to create a pair of connected connections
	cliConn, svrConn := net.Pipe()
	defer cliConn.Close()
	defer svrConn.Close()

	// Create a context that can be canceled
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	// Create server-side Conn
	svrStdConn := newConn(svrConn, 4096).(*Conn)

	// Register connstate callback
	stater, err := connstate.ListenConnState(svrConn,
		connstate.WithOnRemoteClosed(func() {
			cancelCtx()
		}),
	)
	if err != nil {
		// If connstate is not supported on this platform, skip the test
		t.Skipf("connstate.ListenConnState not supported: %v", err)
		return
	}
	defer stater.Close()

	// Set stater to Conn
	svrStdConn.SetStater(stater)

	// Test 1: Normal operation - context should not be canceled
	select {
	case <-ctx.Done():
		t.Fatal("Context should not be canceled in normal operation")
	case <-time.After(100 * time.Millisecond):
		// Expected - context not canceled
	}

	// Test 2: Close client connection - context should be canceled
	cliConn.Close()

	// Wait for context to be canceled (with timeout)
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()

	select {
	case <-ctx.Done():
		// Expected - context canceled after client closes connection
	case <-timeout.C:
		t.Fatal("Context was not canceled after client closed connection")
	}

	// Close should properly clean up stater
	err = svrStdConn.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestSenseClientDisconnectionContextCancel tests that the context is canceled
// when the client disconnects and SenseClientDisconnection is enabled
func TestSenseClientDisconnectionContextCancel(t *testing.T) {
	handlerRunning := make(chan struct{})
	handlerExited := make(chan struct{})

	trans := NewTransporter(&config.Options{
		Network:                  "tcp",
		Addr:                     "127.0.0.1:0",
		SenseClientDisconnection: true,
	}).(*transport)

	go trans.ListenAndServe(func(ctx context.Context, conn interface{}) error {
		close(handlerRunning)

		select {
		case <-ctx.Done():
			// Context was canceled as expected
		case <-time.After(3 * time.Second):
			panic("Context was not canceled after client disconnected")
		}

		close(handlerExited)
		return nil
	})

	for trans.Listener() == nil {
		time.Sleep(2 * time.Millisecond)
	}

	clientConn, err := net.Dial("tcp", trans.Listener().Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// Wait for handler to start running
	<-handlerRunning

	// Close client connection to trigger disconnection detection
	clientConn.Close()

	// Wait for handler to exit (context should be canceled)
	select {
	case <-handlerExited:
		// Handler exited as expected
	case <-time.After(4 * time.Second):
		t.Fatal("Handler did not exit after client disconnected")
	}
}

// TestSenseClientDisconnectionDisabled tests that context cancel behavior
// when SenseClientDisconnection is disabled
func TestSenseClientDisconnectionDisabled(t *testing.T) {
	handlerRunning := make(chan struct{})
	handlerExited := make(chan struct{})

	trans := NewTransporter(&config.Options{
		Network:                  "tcp",
		Addr:                     "127.0.0.1:0",
		SenseClientDisconnection: false,
	}).(*transport)

	go trans.ListenAndServe(func(ctx context.Context, conn interface{}) error {
		close(handlerRunning)

		select {
		case <-ctx.Done():
			panic("Context was not canceled after client disconnected")
		case <-time.After(2 * time.Second):
		}

		close(handlerExited)
		return nil
	})

	for trans.Listener() == nil {
		time.Sleep(1 * time.Millisecond)
	}

	clientConn, err := net.Dial("tcp", trans.Listener().Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// Wait for handler to start running
	<-handlerRunning

	// Close client connection to trigger disconnection detection
	clientConn.Close()

	// Wait for handler to exit (context should be canceled)
	select {
	case <-handlerExited:
		// Handler exited as expected
	case <-time.After(4 * time.Second):
		t.Fatal("Handler did not exit after client disconnected")
	}
}

// mockConnStater is a mock implementation of connstate.ConnStater for testing
type mockConnStater struct {
	closed bool
	state  connstate.ConnState
}

func (m *mockConnStater) Close() error {
	m.closed = true
	return nil
}

func (m *mockConnStater) State() connstate.ConnState {
	return m.state
}
