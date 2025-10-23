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
	"sync/atomic"
	"testing"
	"time"
)

func TestNewTestListener(t *testing.T) {
	ln := NewTestListener(t)
	defer ln.Close()
	t.Log(ln.Addr())
}

type routeEngine struct {
	Running atomic.Bool
}

func (e *routeEngine) IsRunning() bool {
	return e.Running.Load()
}

func TestWaitEngineRunning(t *testing.T) {
	e := &routeEngine{}
	go func() {
		time.Sleep(30 * time.Millisecond)
		e.Running.Store(true)
	}()
	WaitEngineRunning(e)
}
