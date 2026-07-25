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
	"net"
	"sync"
	"syscall"
	"time"
)

// tcpHealthChecker owns the RawConn.Read callback for a native TCP connection.
// The HTTP pool serializes use of an idle connection; the mutex also serializes
// accidental concurrent probes before the result is returned.
type tcpHealthChecker struct {
	mu      sync.Mutex
	tcpConn *net.TCPConn
	rawConn syscall.RawConn
	healthy bool
	probe   func(uintptr) bool
}

type unhealthyTCPHealthChecker struct{}

func (unhealthyTCPHealthChecker) isHealthy() bool { return false }

func newTCPHealthChecker(conn net.Conn) connHealthChecker {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return unhealthyTCPHealthChecker{}
	}
	checker := &tcpHealthChecker{tcpConn: tcpConn, rawConn: rawConn}
	checker.probe = func(fd uintptr) bool {
		checker.healthy = recvHealthyTCP(fd)
		// MSG_DONTWAIT makes the probe decisive at this instant. Returning true
		// prevents RawConn.Read from registering or waiting in the runtime poller.
		return true
	}
	return checker
}

func (c *tcpHealthChecker) isHealthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Keep the timed-Peek health check's deadline-reset contract: an HTTP
	// response may leave an expired read deadline on a pooled connection. Clear
	// it before RawConn.Read so a healthy idle connection is not rejected. This
	// does not install a probe deadline: MSG_DONTWAIT keeps the socket
	// observation non-blocking.
	if err := c.tcpConn.SetReadDeadline(time.Time{}); err != nil {
		return false
	}
	c.healthy = false
	if err := c.rawConn.Read(c.probe); err != nil {
		return false
	}
	return c.healthy
}

func recvHealthyTCP(fd uintptr) bool {
	var buffer [1]byte
	for {
		_, _, err := syscall.Recvfrom(int(fd), buffer[:], syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		switch err {
		case nil:
			// A positive result is unread data; a zero-length result is the
			// peer's FIN. Neither connection is safe to reuse.
			return false
		case syscall.EAGAIN: // EWOULDBLOCK is the same errno on Linux.
			return true
		case syscall.EINTR:
			continue
		default:
			return false
		}
	}
}
