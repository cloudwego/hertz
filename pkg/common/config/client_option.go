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

	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

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

	// Callback for establishing new connections to hosts.
	//
	// Default Dial is used if not set.
	Dial func(addr string) (network.Conn, error)

	// Attempt to connect to both ipv4 and ipv6 addresses if set to true.
	//
	// This option is used only if default TCP dialer is used,
	// i.e. if Dial is blank.
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
}

func NewClientOptions(opts []ClientOption) *ClientOptions {
	options := &ClientOptions{
		DialTimeout:         consts.DefaultDialTimeout,
		MaxConnsPerHost:     consts.DefaultMaxConnsPerHost,
		MaxIdleConnDuration: consts.DefaultMaxIdleConnDuration,
		KeepAlive:           true,
	}
	options.Apply(opts)

	return options
}

func (o *ClientOptions) Apply(opts []ClientOption) {
	for _, op := range opts {
		op.F(o)
	}
}
