// Copyright 2023 CloudWeGo Authors
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

//go:build windows
// +build windows

package client

import (
	"crypto/tls"
	"time"

	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

func newMockDialerWithCustomFunc(network, address string, timeout time.Duration, f func(network, address string, timeout time.Duration, tlsConfig *tls.Config)) network.Dialer {
	dialer := standard.NewDialer()
	return &mockDialer{
		Dialer:           dialer,
		customDialerFunc: f,
		network:          network,
		address:          address,
		timeout:          timeout,
	}
}
