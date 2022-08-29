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

package dialer

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/cloudwego/hertz/pkg/network"
)

var defaultDialer network.Dialer

// SetDialer is used to set the global default dialer.
// Deprecated: use WithDialer instead.
func SetDialer(dialer network.Dialer) {
	defaultDialer = dialer
}

func DefaultDialer() network.Dialer {
	return defaultDialer
}

func DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	return defaultDialer.DialConnection(network, address, timeout, tlsConfig)
}

func DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error) {
	return defaultDialer.DialTimeout(network, address, timeout, tlsConfig)
}

// AddTLS is used to add tls to a persistent connection, i.e. negotiate a TLS session. If conn is already a TLS
// tunnel, this function establishes a nested TLS session inside the encrypted channel.
func AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	return defaultDialer.AddTLS(conn, tlsConfig)
}
