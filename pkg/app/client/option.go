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

package client

import (
	"crypto/tls"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client/retry"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/dialer"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// WithDialTimeout sets dial timeout.
func WithDialTimeout(dialTimeout time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.DialTimeout = dialTimeout
	}}
}

// WithMaxConnsPerHost sets maximum number of connections per host which may be established.
func WithMaxConnsPerHost(mc int) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.MaxConnsPerHost = mc
	}}
}

// WithMaxIdleConnDuration sets max idle connection duration, idle keep-alive connections are closed after this duration.
func WithMaxIdleConnDuration(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.MaxIdleConnDuration = t
	}}
}

// WithMaxConnDuration sets max connection duration, keep-alive connections are closed after this duration.
func WithMaxConnDuration(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.MaxConnDuration = t
	}}
}

// WithMaxConnWaitTimeout sets maximum duration for waiting for a free connection.
func WithMaxConnWaitTimeout(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.MaxConnWaitTimeout = t
	}}
}

// WithKeepAlive determines whether use keep-alive connection.
func WithKeepAlive(b bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.KeepAlive = b
	}}
}

// WithClientReadTimeout sets maximum duration for full response reading (including body).
func WithClientReadTimeout(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.ReadTimeout = t
	}}
}

// WithTLSConfig sets tlsConfig to create a tls connection.
func WithTLSConfig(cfg *tls.Config) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.TLSConfig = cfg
		o.Dialer = standard.NewDialer()
	}}
}

// WithDialer sets the specific dialer.
func WithDialer(d network.Dialer) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.Dialer = d
	}}
}

// WithResponseBodyStream is used to determine whether read body in stream or not.
func WithResponseBodyStream(b bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.ResponseBodyStream = b
	}}
}

// WithDisableHeaderNamesNormalizing is used to set whether disable header names normalizing.
func WithDisableHeaderNamesNormalizing(disable bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.DisableHeaderNamesNormalizing = disable
	}}
}

// WithName sets client name which used in User-Agent Header.
func WithName(name string) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.Name = name
	}}
}

// WithNoDefaultUserAgentHeader sets whether no default User-Agent header.
func WithNoDefaultUserAgentHeader(isNoDefaultUserAgentHeader bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.NoDefaultUserAgentHeader = isNoDefaultUserAgentHeader
	}}
}

// WithDisablePathNormalizing sets whether disable path normalizing.
func WithDisablePathNormalizing(isDisablePathNormalizing bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.DisablePathNormalizing = isDisablePathNormalizing
	}}
}

func WithRetryConfig(opts ...retry.Option) config.ClientOption {
	retryCfg := &retry.Config{
		MaxAttemptTimes: consts.DefaultMaxRetryTimes,
		Delay:           1 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		MaxJitter:       20 * time.Millisecond,
		DelayPolicy:     retry.CombineDelay(retry.DefaultDelayPolicy),
	}
	retryCfg.Apply(opts)

	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.RetryConfig = retryCfg
	}}
}

// WithWriteTimeout sets write timeout.
func WithWriteTimeout(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.WriteTimeout = t
	}}
}

// WithConnStateObserve sets the connection state observation function.
// The first param is used to set hostclient state func.
// The second param is used to set observation interval, default value is 5 seconds.
// Warn: Do not start go routine in HostClientStateFunc.
func WithConnStateObserve(hs config.HostClientStateFunc, interval ...time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.HostClientStateObserve = hs
		if len(interval) > 0 {
			o.ObservationInterval = interval[0]
		}
	}}
}

// WithDialFunc is used to set dialer function.
// Note: WithDialFunc will overwrite custom dialer.
func WithDialFunc(f network.DialFunc, dialers ...network.Dialer) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		d := dialer.DefaultDialer()
		if len(dialers) != 0 {
			d = dialers[0]
		}
		o.Dialer = newCustomDialerWithDialFunc(d, f)
	}}
}

// customDialer set customDialerFunc and params to set dailFunc
type customDialer struct {
	network.Dialer
	dialFunc network.DialFunc
}

func (m *customDialer) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	if m.dialFunc != nil {
		return m.dialFunc(address)
	}
	return m.Dialer.DialConnection(network, address, timeout, tlsConfig)
}

func newCustomDialerWithDialFunc(dialer network.Dialer, dialFunc network.DialFunc) network.Dialer {
	return &customDialer{
		Dialer:   dialer,
		dialFunc: dialFunc,
	}
}
