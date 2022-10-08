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
	"time"

	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Option is the only struct that can be used to set HTTP2 Config.
type Option struct {
	F func(o *Config)
}

// Config All configurations related to retry
type Config struct {
	// MaxHeaderListSize is the http2 SETTINGS_MAX_HEADER_LIST_SIZE to
	// send in the initial settings frame. It is how many bytes
	// of response headers are allowed. Unlike the http2 spec, zero here
	// means to use a default limit (currently 10MB). If you actually
	// want to advertise an unlimited value to the peer, Transport
	// interprets the highest possible value here (0xffffffff or 1<<32-1)
	// to mean no limit.
	MaxHeaderListSize uint32

	// ReadIdleTimeout is the timeout after which a health check using ping
	// frame will be carried out if no frame is received on the connection.
	// Note that a ping response will is considered a received frame, so if
	// there is no other traffic on the connection, the health check will
	// be performed every ReadIdleTimeout interval.
	// If zero, no health check is performed.
	ReadIdleTimeout time.Duration

	// PingTimeout is the timeout after which the connection will be closed
	// if a response to Ping is not received.
	// Defaults to 15s.
	PingTimeout time.Duration

	// WriteByteTimeout is the timeout after which the connection will be
	// closed no data can be written to it. The timeout begins when data is
	// available to write, and is extended whenever any bytes are written.
	WriteByteTimeout time.Duration

	// StrictMaxConcurrentStreams controls whether the server's
	// SETTINGS_MAX_CONCURRENT_STREAMS should be respected
	// globally. If false, new TCP connections are created to the
	// server as needed to keep each under the per-connection
	// SETTINGS_MAX_CONCURRENT_STREAMS limit. If true, the
	// server's SETTINGS_MAX_CONCURRENT_STREAMS is interpreted as
	// a global limit and callers of RoundTrip block when needed,
	// waiting for their turn.
	StrictMaxConcurrentStreams bool
}

func (o *Config) Apply(opts []Option) {
	for _, op := range opts {
		op.F(o)
	}
}

// WithMaxHeaderListSize sets max header list size.
func WithMaxHeaderListSize(maxHeaderListSize uint32) Option {
	return Option{F: func(o *Config) {
		o.MaxHeaderListSize = maxHeaderListSize
	}}
}

// WithReadIdleTimeout is used to set the timeout after which a health check using ping
// frame will be carried out if no frame is received on the connection.
func WithReadIdleTimeout(readIdleTimeout time.Duration) Option {
	return Option{F: func(o *Config) {
		o.ReadIdleTimeout = readIdleTimeout
	}}
}

// WithWriteByteTimeout is used to set the timeout after which the connection will be
// closed no data can be written to it.
func WithWriteByteTimeout(writeByteTimeout time.Duration) Option {
	return Option{F: func(o *Config) {
		o.WriteByteTimeout = writeByteTimeout
	}}
}

// WithStrictMaxConcurrentStreams is used to controls whether the server's
// SETTINGS_MAX_CONCURRENT_STREAMS should be respected globally.
func WithStrictMaxConcurrentStreams(strictMaxConcurrentStreams bool) Option {
	return Option{F: func(o *Config) {
		o.StrictMaxConcurrentStreams = strictMaxConcurrentStreams
	}}
}

// WithPingTimeout is used to set the timeout after which the connection will be closed
// if a response to Ping is not received.
func WithPingTimeout(pt time.Duration) Option {
	return Option{F: func(o *Config) {
		o.PingTimeout = pt
	}}
}

func New() *Config {
	return &Config{
		PingTimeout: consts.DefaultPingTimeout,
	}
}
