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

package config

import (
	"crypto/tls"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client/retry"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type ConnPoolState struct {
	// The conn num of conn pool. These conns are idle connections.
	PoolConnNum int
	// Total conn num.
	TotalConnNum int
	// Number of pending connections
	WaitConnNum int
	// HostClient Addr
	Addr string
}

type HostClientState interface {
	ConnPoolState() ConnPoolState
}

type HostClientStateFunc func(HostClientState)

// ClientOption is the only struct that can be used to set ClientOptions.
type ClientOption struct {
	F func(o *ClientOptions)
}

type ClientOptions struct {
	// Timeout for establishing a connection to server
	DialTimeout time.Duration
	// The max connection nums for each host
	MaxConnsPerHost int

	MaxIdleConnDuration time.Duration
	MaxConnDuration     time.Duration
	MaxConnWaitTimeout  time.Duration
	KeepAlive           bool
	ReadTimeout         time.Duration
	TLSConfig           *tls.Config
	ResponseBodyStream  bool

	// Client name. Used in User-Agent request header.
	//
	// Default client name is used if not set.
	Name string

	// NoDefaultUserAgentHeader when set to true, causes the default
	// User-Agent header to be excluded from the Request.
	NoDefaultUserAgentHeader bool

	// Dialer is the custom dialer used to establish connection.
	// Default Dialer is used if not set.
	Dialer network.Dialer

	// Attempt to connect to both ipv4 and ipv6 addresses if set to true.
	//
	// This option is used only if default TCP dialer is used,
	// i.e. if Dialer is blank.
	//
	// By default client connects only to ipv4 addresses,
	// since unfortunately ipv6 remains broken in many networks worldwide :)
	DialDualStack bool

	// Maximum duration for full request writing (including body).
	//
	// By default request write timeout is unlimited.
	WriteTimeout time.Duration

	// Maximum response body size.
	//
	// The client returns ErrBodyTooLarge if this limit is greater than 0
	// and response body is greater than the limit.
	//
	// By default response body size is unlimited.
	MaxResponseBodySize int

	// Header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// responses to other clients expecting case-sensitive header names.
	//
	// By default request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool

	// Path values are sent as-is without normalization
	//
	// Disabled path normalization may be useful for proxying incoming requests
	// to servers that are expecting paths to be forwarded as-is.
	//
	// By default path values are normalized, i.e.
	// extra slashes are removed, special characters are encoded.
	DisablePathNormalizing bool

	// all configurations related to retry
	RetryConfig *retry.Config

	HostClientStateObserve HostClientStateFunc

	// StateObserve execution interval
	ObservationInterval time.Duration

	// Callback hook for re-configuring host client
	// If an error is returned, the request will be terminated.
	HostClientConfigHook func(hc interface{}) error
}

func NewClientOptions(opts []ClientOption) *ClientOptions {
	options := &ClientOptions{
		DialTimeout:         consts.DefaultDialTimeout,
		MaxConnsPerHost:     consts.DefaultMaxConnsPerHost,
		MaxIdleConnDuration: consts.DefaultMaxIdleConnDuration,
		KeepAlive:           true,
		ObservationInterval: time.Second * 5,
	}
	options.Apply(opts)

	return options
}

func (o *ClientOptions) Apply(opts []ClientOption) {
	for _, op := range opts {
		op.F(o)
	}
}
