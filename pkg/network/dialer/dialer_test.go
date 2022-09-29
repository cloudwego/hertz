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
	"errors"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/network"
)

func TestDialer(t *testing.T) {
	SetDialer(&mockDialer{})
	dialer := DefaultDialer()
	assert.DeepEqual(t, &mockDialer{}, dialer)

	_, err := AddTLS(nil, nil)
	assert.NotNil(t, err)

	_, err = DialConnection("", "", 0, nil)
	assert.NotNil(t, err)

	_, err = DialTimeout("", "", 0, nil)
	assert.NotNil(t, err)
}

type mockDialer struct{}

func (m *mockDialer) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	return nil, errors.New("method not implement")
}

func (m *mockDialer) DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error) {
	return nil, errors.New("method not implement")
}

func (m *mockDialer) AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	return nil, errors.New("method not implement")
}
