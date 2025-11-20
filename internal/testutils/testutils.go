/*
 * Copyright 2025 CloudWeGo Authors
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

package testutils

import (
	"net"
	"testing"
	"time"
)

type RouteEngine interface {
	IsRunning() bool
}

func WaitEngineRunning(e RouteEngine) {
	for i := 0; i < 100; i++ {
		if e.IsRunning() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	panic("not running")
}

// NewTestListener creates a TCP listener on a random available port.
// It calls tb.Fatal if the listener cannot be created.
// The caller is responsible for closing the listener (usually via defer).
func NewTestListener(tb testing.TB) net.Listener {
	tb.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("failed to create test listener: %s", err)
	}
	return ln
}
