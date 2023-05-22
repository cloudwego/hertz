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
 *
 * Copyright 2016 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"time"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/internal/bytestr"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	reqI "github.com/cloudwego/hertz/pkg/protocol/http1/req"
	respI "github.com/cloudwego/hertz/pkg/protocol/http1/resp"
)

func SetupProxy(conn network.Conn, addr string, proxyURI *protocol.URI, tlsConfig *tls.Config, isTLS bool, dialer network.Dialer) (network.Conn, error) {
	var err error
	if bytes.Equal(proxyURI.Scheme(), bytestr.StrHTTPS) {
		conn, err = dialer.AddTLS(conn, tlsConfig)
		if err != nil {
			return nil, err
		}
	}

	switch {
	case proxyURI == nil:
		// Do nothing. Not using a proxy.
	case isTLS: // target addr is https
		connectReq, connectResp := protocol.AcquireRequest(), protocol.AcquireResponse()
		defer func() {
			protocol.ReleaseRequest(connectReq)
			protocol.ReleaseResponse(connectResp)
		}()

		SetProxyAuthHeader(&connectReq.Header, proxyURI)
		connectReq.SetMethod(consts.MethodConnect)
		connectReq.SetHost(addr)

		// Skip response body when send CONNECT request.
		connectResp.SkipBody = true

		// If there's no done channel (no deadline or cancellation
		// from the caller possible), at least set some (long)
		// timeout here. This will make sure we don't block forever
		// and leak a goroutine if the connection stops replying
		// after the TCP connect.
		connectCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		didReadResponse := make(chan struct{}) // closed after CONNECT write+read is done or fails

		// Write the CONNECT request & read the response.
		go func() {
			defer close(didReadResponse)

			err = reqI.Write(connectReq, conn)

			if err != nil {
				return
			}

			err = conn.Flush()

			if err != nil {
				return
			}

			err = respI.Read(connectResp, conn)
		}()
		select {
		case <-connectCtx.Done():
			conn.Close()
			<-didReadResponse

			return nil, connectCtx.Err()
		case <-didReadResponse:
		}

		if err != nil {
			conn.Close()
			return nil, err
		}

		if connectResp.StatusCode() != consts.StatusOK {
			conn.Close()

			return nil, errors.NewPublic(consts.StatusMessage(connectResp.StatusCode()))
		}
	}

	if proxyURI != nil && isTLS {
		conn, err = dialer.AddTLS(conn, tlsConfig)
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

func SetProxyAuthHeader(h *protocol.RequestHeader, proxyURI *protocol.URI) {
	if username := proxyURI.Username(); username != nil {
		password := proxyURI.Password()
		auth := base64.StdEncoding.EncodeToString(bytesconv.S2b(bytesconv.B2s(username) + ":" + bytesconv.B2s(password)))
		h.Set("Proxy-Authorization", "Basic "+auth)
	}
}
