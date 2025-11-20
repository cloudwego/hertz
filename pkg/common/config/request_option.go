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

import "time"

var preDefinedOpts []RequestOption

type RequestOptions struct {
	tags map[string]string
	isSD bool

	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	// Request timeout. Usually set by DoDeadline or DoTimeout
	// if <= 0, means not set
	requestTimeout time.Duration
	start          time.Time
}

// RequestOption is the only struct to set request-level options.
type RequestOption struct {
	F func(o *RequestOptions)
}

// NewRequestOptions create a *RequestOptions according to the given opts.
func NewRequestOptions(opts []RequestOption) *RequestOptions {
	options := &RequestOptions{
		tags: make(map[string]string),
		isSD: false,
	}
	if preDefinedOpts != nil {
		options.Apply(preDefinedOpts)
	}
	options.Apply(opts)
	return options
}

// WithTag set tag in RequestOptions.
func WithTag(k, v string) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.tags[k] = v
	}}
}

// WithSD set isSD in RequestOptions.
func WithSD(b bool) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.isSD = b
	}}
}

// WithDialTimeout sets dial timeout.
//
// This is the request level configuration. It has a higher
// priority than the client level configuration
// Note: it won't take effect in the case of the number of
// connections in the connection pool exceeds the maximum
// number of connections and needs to establish a connection
// while waiting.
func WithDialTimeout(t time.Duration) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.dialTimeout = t
	}}
}

// WithReadTimeout sets read timeout.
//
// This is the request level configuration. It has a higher
// priority than the client level configuration
func WithReadTimeout(t time.Duration) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.readTimeout = t
	}}
}

// WithWriteTimeout sets write timeout.
//
// This is the request level configuration. It has a higher
// priority than the client level configuration
func WithWriteTimeout(t time.Duration) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.writeTimeout = t
	}}
}

// WithRequestTimeout sets whole request timeout. If it reaches timeout,
// the client will return.
//
// This is the request level configuration.
func WithRequestTimeout(t time.Duration) RequestOption {
	return RequestOption{F: func(o *RequestOptions) {
		o.requestTimeout = t
	}}
}

func (o *RequestOptions) Apply(opts []RequestOption) {
	for _, op := range opts {
		op.F(o)
	}
}

func (o *RequestOptions) Tag(k string) string {
	return o.tags[k]
}

func (o *RequestOptions) Tags() map[string]string {
	return o.tags
}

func (o *RequestOptions) IsSD() bool {
	return o.isSD
}

func (o *RequestOptions) DialTimeout() time.Duration {
	return o.dialTimeout
}

func (o *RequestOptions) ReadTimeout() time.Duration {
	return o.readTimeout
}

func (o *RequestOptions) WriteTimeout() time.Duration {
	return o.writeTimeout
}

func (o *RequestOptions) RequestTimeout() time.Duration {
	return o.requestTimeout
}

// StartRequest records the start time of the request.
//
// Note: Users should not call this method.
func (o *RequestOptions) StartRequest() {
	if o.requestTimeout > 0 {
		o.start = time.Now()
	}
}

func (o *RequestOptions) StartTime() time.Time {
	return o.start
}

func (o *RequestOptions) CopyTo(dst *RequestOptions) {
	if dst.tags == nil {
		dst.tags = make(map[string]string)
	}

	for k, v := range o.tags {
		dst.tags[k] = v
	}

	dst.isSD = o.isSD
	dst.readTimeout = o.readTimeout
	dst.writeTimeout = o.writeTimeout
	dst.dialTimeout = o.dialTimeout
	dst.requestTimeout = o.requestTimeout
	dst.start = o.start
}

// SetPreDefinedOpts Pre define some RequestOption here
func SetPreDefinedOpts(opts ...RequestOption) {
	preDefinedOpts = nil
	preDefinedOpts = append(preDefinedOpts, opts...)
}
