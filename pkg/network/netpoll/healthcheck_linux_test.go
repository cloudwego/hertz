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
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/mock"
	"github.com/cloudwego/hertz/pkg/network"
)

type reuseHealthProbe struct {
	network.Conn
	healthy bool
	calls   int
}

func (p *reuseHealthProbe) IsHealthyForReuse() bool {
	p.calls++
	return p.healthy
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
			if got := conn.IsHealthy(50 * time.Microsecond); got != tt.healthy {
				t.Fatalf("owner probe result: got %v, want %v", got, tt.healthy)
			}
			if probe.calls != 1 {
				t.Fatalf("owner probe calls: got %d, want 1", probe.calls)
			}
		})
	}
}

func TestConnIsHealthyFallsBackToTimedPeek(t *testing.T) {
	conn := &Conn{Conn: mock.NewConn("")}
	if !conn.IsHealthy(50 * time.Microsecond) {
		t.Fatal("old netpoll without owner probe should use the timed Peek fallback")
	}
}

func TestConnIsHealthyFallbackRejectsStaleConnection(t *testing.T) {
	conn := &Conn{Conn: mock.NewBrokenConn("")}
	if conn.IsHealthy(50 * time.Microsecond) {
		t.Fatal("timed Peek fallback must reject a stale connection")
	}
}
