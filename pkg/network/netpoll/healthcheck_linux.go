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
	"net"
	"time"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
)

type reusableConnHealthChecker interface {
	IsHealthyForReuse(ownerWaitTimeout time.Duration) bool
}

// IsHealthy checks whether an idle pooled connection is safe to reuse without
// consuming application data. probeTimeout bounds the fallback timed Peek;
// ownerWaitTimeout bounds waiting for the netpoll operator. The owner-side
// probe itself is non-blocking once acquired.
func (c *Conn) IsHealthy(probeTimeout, ownerWaitTimeout time.Duration) bool {
	if c == nil || c.Conn == nil || probeTimeout <= 0 || ownerWaitTimeout <= 0 || c.Len() > 0 {
		return false
	}

	// Newer netpoll versions synchronize this check with their receive owner.
	// Keep the narrow assertion here so Hertz remains compatible with older
	// netpoll versions and third-party network.Conn implementations.
	if checker, ok := c.Conn.(reusableConnHealthChecker); ok {
		return checker.IsHealthyForReuse(ownerWaitTimeout)
	}

	// Older netpoll versions have no owner-side probe. Preserve the bounded
	// Peek fallback for rolling upgrades instead of inspecting their raw fd.
	if err := c.SetReadTimeout(probeTimeout); err != nil {
		return false
	}
	p, err := c.Peek(1)
	if resetErr := c.SetReadTimeout(0); resetErr != nil {
		return false
	}
	if len(p) != 0 {
		return false
	}
	if errors.Is(err, errs.ErrTimeout) {
		return true
	}
	var timeoutErr net.Error
	return errors.As(err, &timeoutErr) && timeoutErr.Timeout()
}
