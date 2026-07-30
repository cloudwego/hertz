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

package netpoll

import (
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/network"
)

type reuseHealthProbe struct {
	network.Conn
	healthy          bool
	calls            int
	ownerWaitTimeout time.Duration
}

func (p *reuseHealthProbe) IsHealthyForReuse(ownerWaitTimeout time.Duration) bool {
	p.calls++
	p.ownerWaitTimeout = ownerWaitTimeout
	return p.healthy
}

type fallbackHealthProbe struct {
	network.Conn
	failTimeoutCall int
	timeoutCalls    int
	peekData        []byte
}

func (p *fallbackHealthProbe) SetReadTimeout(timeout time.Duration) error {
	p.timeoutCalls++
	if p.timeoutCalls == p.failTimeoutCall {
		return errors.New("set read timeout failed")
	}
	return p.Conn.SetReadTimeout(timeout)
}

func (p *fallbackHealthProbe) Peek(int) ([]byte, error) {
	if p.peekData != nil {
		return p.peekData, nil
	}
	return p.Conn.Peek(1)
}

func TestConnIsHealthyDelegatesToOwnerProbe(t *testing.T) {
	for _, tt := range []struct {
		name    string
		healthy bool
	}{
		{name: "healthy", healthy: true},
		{name: "unhealthy", healthy: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			probe := &reuseHealthProbe{Conn: mock.NewConn(""), healthy: tt.healthy}
			conn := &Conn{Conn: probe}
			if got := conn.IsHealthy(50*time.Microsecond, time.Second); got != tt.healthy {
				t.Fatalf("owner probe result: got %v, want %v", got, tt.healthy)
			}
			if probe.calls != 1 {
				t.Fatalf("owner probe calls: got %d, want 1", probe.calls)
			}
			if probe.ownerWaitTimeout != time.Second {
				t.Fatalf("owner wait timeout: got %v, want %v", probe.ownerWaitTimeout, time.Second)
			}
		})
	}
}

func TestConnIsHealthyFallsBackToTimedPeek(t *testing.T) {
	conn := &Conn{Conn: mock.NewConn("")}
	if !conn.IsHealthy(50*time.Microsecond, time.Second) {
		t.Fatal("old netpoll without owner probe should use the timed Peek fallback")
	}
}

func TestConnIsHealthyFallbackRejectsStaleConnection(t *testing.T) {
	conn := &Conn{Conn: mock.NewBrokenConn("")}
	if conn.IsHealthy(50*time.Microsecond, time.Second) {
		t.Fatal("timed Peek fallback must reject a stale connection")
	}
}

func TestConnIsHealthyRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name             string
		conn             *Conn
		probeTimeout     time.Duration
		ownerWaitTimeout time.Duration
	}{
		{name: "nil receiver", probeTimeout: time.Second, ownerWaitTimeout: time.Second},
		{name: "nil connection", conn: &Conn{}, probeTimeout: time.Second, ownerWaitTimeout: time.Second},
		{name: "zero probe timeout", conn: &Conn{Conn: mock.NewConn("")}, ownerWaitTimeout: time.Second},
		{name: "zero owner wait timeout", conn: &Conn{Conn: mock.NewConn("")}, probeTimeout: time.Second},
		{name: "buffered data", conn: &Conn{Conn: mock.NewConn("x")}, probeTimeout: time.Second, ownerWaitTimeout: time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.conn.IsHealthy(tt.probeTimeout, tt.ownerWaitTimeout) {
				t.Fatal("invalid health check input must fail closed")
			}
		})
	}
}

func TestConnIsHealthyFallbackRejectsReadTimeoutFailures(t *testing.T) {
	for _, failCall := range []int{1, 2} {
		probe := &fallbackHealthProbe{Conn: mock.NewConn(""), failTimeoutCall: failCall}
		conn := &Conn{Conn: probe}
		if conn.IsHealthy(50*time.Microsecond, time.Second) {
			t.Fatalf("read timeout failure on call %d must fail closed", failCall)
		}
		if probe.timeoutCalls != failCall {
			t.Fatalf("read timeout calls: got %d, want %d", probe.timeoutCalls, failCall)
		}
	}
}

func TestConnIsHealthyFallbackRejectsUnreadData(t *testing.T) {
	probe := &fallbackHealthProbe{Conn: mock.NewConn(""), peekData: []byte("x")}
	conn := &Conn{Conn: probe}
	if conn.IsHealthy(50*time.Microsecond, time.Second) {
		t.Fatal("timed Peek fallback must reject unread data")
	}
	if probe.timeoutCalls != 2 {
		t.Fatalf("read timeout calls: got %d, want 2", probe.timeoutCalls)
	}
}
