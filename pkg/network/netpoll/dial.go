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
//

package netpoll

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/netpoll"
)

var errNotSupportTLS = errors.NewPublic("not support tls")

type dialer struct {
	netpoll.Dialer
}

func (d dialer) DialConnection(n, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	if tlsConfig != nil {
		// https
		return nil, errNotSupportTLS
	}
	c, err := d.Dialer.DialConnection(n, address, timeout)
	if err != nil {
		return nil, err
	}
	conn = newConn(c)
	return
}

func (d dialer) DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error) {
	if tlsConfig != nil {
		return nil, errNotSupportTLS
	}
	conn, err = d.Dialer.DialTimeout(network, address, timeout)
	return
}

func (d dialer) AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	return nil, errNotSupportTLS
}

func NewDialer() network.Dialer {
	return dialer{Dialer: netpoll.NewDialer()}
}
