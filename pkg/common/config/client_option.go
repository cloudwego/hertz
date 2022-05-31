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

	MaxIdleConnDuration       time.Duration
	MaxConnDuration           time.Duration
	MaxConnWaitTimeout        time.Duration
	MaxIdempotentCallAttempts int
	KeepAlive                 bool
	ReadTimeout               time.Duration
	TLSConfig                 *tls.Config
	ResponseBodyStream        bool
	DialFunc                  func(addr string) (network.Conn, error)
}

func NewClientOptions(opts []ClientOption) *ClientOptions {
	options := &ClientOptions{
		DialTimeout:               consts.DefaultDialTimeout,
		MaxConnsPerHost:           consts.DefaultMaxConnsPerHost,
		MaxIdleConnDuration:       consts.DefaultMaxIdleConnDuration,
		MaxIdempotentCallAttempts: consts.DefaultMaxIdempotentCallAttempts,
		KeepAlive:                 true,
	}
	options.Apply(opts)
	return options
}

func (o *ClientOptions) Apply(opts []ClientOption) {
	for _, op := range opts {
		op.F(o)
	}
}
