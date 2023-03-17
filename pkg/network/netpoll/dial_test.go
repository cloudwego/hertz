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

//go:build !windows
// +build !windows

package netpoll

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
)

func TestDial(t *testing.T) {
	t.Run("NetpollDial", func(t *testing.T) {
		const nw = "tcp"
		const addr = ":1234"
		transporter := NewTransporter(&config.Options{
			Addr:    addr,
			Network: nw,
		})
		go transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
			return nil
		})
		defer transporter.Close()

		dial := NewDialer()
		// DialConnection
		_, err := dial.DialConnection("tcp", ":4567", time.Second, nil) // wrong addr
		assert.NotNil(t, err)

		nwConn, err := dial.DialConnection(nw, addr, time.Second, nil)
		assert.Nil(t, err)
		defer nwConn.Close()
		_, err = nwConn.Write([]byte("abcdef"))
		assert.Nil(t, err)
		// DialTimeout
		nConn, err := dial.DialTimeout(nw, addr, time.Second, nil)
		assert.Nil(t, err)
		defer nConn.Close()
	})

	t.Run("NotSupportTLS", func(t *testing.T) {
		dial := NewDialer()
		_, err := dial.AddTLS(mock.NewConn(""), nil)
		assert.DeepEqual(t, errNotSupportTLS, err)
		_, err = dial.DialConnection("tcp", ":1234", time.Microsecond, &tls.Config{})
		assert.DeepEqual(t, errNotSupportTLS, err)
	})
}
