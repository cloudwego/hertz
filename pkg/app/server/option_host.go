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
//go:build !tinygo
// +build !tinygo

package server

import (
	"crypto/tls"
	"net"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

// WithTLS sets TLS config to start a tls server.
//
// NOTE: If a tls server is started, it won't accept non-tls request.
func WithTLS(cfg *tls.Config) config.Option {
	return config.Option{F: func(o *config.Options) {
		// If there is no explicit transporter, change it to standard one. Netpoll do not support tls yet.
		if o.TransporterNewer == nil {
			o.TransporterNewer = standard.NewTransporter
		}
		o.TLS = cfg
	}}
}

// WithListenConfig sets listener config.
func WithListenConfig(l *net.ListenConfig) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ListenConfig = l
	}}
}
