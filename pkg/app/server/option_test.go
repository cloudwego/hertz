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

package server

import (
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server/registry"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	"github.com/cloudwego/hertz/pkg/common/utils"
)

func TestOptions(t *testing.T) {
	info := &registry.Info{
		ServiceName: "hertz.test.api",
		Weight:      10,
		Addr:        utils.NewNetAddr("tcp", ":8888"),
	}
	opt := config.NewOptions([]config.Option{
		WithReadTimeout(time.Second),
		WithIdleTimeout(time.Second),
		WithKeepAliveTimeout(time.Second),
		WithRedirectTrailingSlash(false),
		WithRedirectFixedPath(true),
		WithHandleMethodNotAllowed(true),
		WithUseRawPath(true),
		WithRemoveExtraSlash(true),
		WithUnescapePathValues(false),
		WithDisablePreParseMultipartForm(true),
		WithStreamBody(false),
		WithHostPorts(":8888"),
		WithMaxRequestBodySize(2),
		WithDisablePrintRoute(true),
		WithNetwork("unix"),
		WithExitWaitTime(time.Second),
		WithMaxKeepBodySize(500),
		WithGetOnly(true),
		WithKeepAlive(false),
		WithTLS(nil),
		WithH2C(true),
		WithReadBufferSize(100),
		WithALPN(true),
		WithTraceLevel(stats.LevelDisabled),
		WithRegistry(nil, info),
		WithAutoReloadRender(true, 5*time.Second),
	})
	assert.DeepEqual(t, opt.ReadTimeout, time.Second)
	assert.DeepEqual(t, opt.IdleTimeout, time.Second)
	assert.DeepEqual(t, opt.KeepAliveTimeout, time.Second)
	assert.DeepEqual(t, opt.RedirectTrailingSlash, false)
	assert.DeepEqual(t, opt.RedirectFixedPath, true)
	assert.DeepEqual(t, opt.HandleMethodNotAllowed, true)
	assert.DeepEqual(t, opt.UseRawPath, true)
	assert.DeepEqual(t, opt.RemoveExtraSlash, true)
	assert.DeepEqual(t, opt.UnescapePathValues, false)
	assert.DeepEqual(t, opt.DisablePreParseMultipartForm, true)
	assert.DeepEqual(t, opt.StreamRequestBody, false)
	assert.DeepEqual(t, opt.Addr, ":8888")
	assert.DeepEqual(t, opt.MaxRequestBodySize, 2)
	assert.DeepEqual(t, opt.DisablePrintRoute, true)
	assert.DeepEqual(t, opt.Network, "unix")
	assert.DeepEqual(t, opt.ExitWaitTimeout, time.Second)
	assert.DeepEqual(t, opt.MaxKeepBodySize, 500)
	assert.DeepEqual(t, opt.GetOnly, true)
	assert.DeepEqual(t, opt.DisableKeepalive, true)
	assert.DeepEqual(t, opt.H2C, true)
	assert.DeepEqual(t, opt.ReadBufferSize, 100)
	assert.DeepEqual(t, opt.ALPN, true)
	assert.DeepEqual(t, opt.TraceLevel, stats.LevelDisabled)
	assert.DeepEqual(t, opt.RegistryInfo, info)
	assert.DeepEqual(t, opt.Registry, nil)
	assert.DeepEqual(t, opt.AutoReloadRender, true)
	assert.DeepEqual(t, opt.AutoReloadInterval, 5*time.Second)
}

func TestDefaultOptions(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	assert.DeepEqual(t, opt.ReadTimeout, time.Minute*3)
	assert.DeepEqual(t, opt.IdleTimeout, time.Minute*3)
	assert.DeepEqual(t, opt.KeepAliveTimeout, time.Minute)
	assert.DeepEqual(t, opt.RedirectTrailingSlash, true)
	assert.DeepEqual(t, opt.RedirectFixedPath, false)
	assert.DeepEqual(t, opt.HandleMethodNotAllowed, false)
	assert.DeepEqual(t, opt.UseRawPath, false)
	assert.DeepEqual(t, opt.RemoveExtraSlash, false)
	assert.DeepEqual(t, opt.UnescapePathValues, true)
	assert.DeepEqual(t, opt.DisablePreParseMultipartForm, false)
	assert.DeepEqual(t, opt.StreamRequestBody, false)
	assert.DeepEqual(t, opt.Addr, ":8888")
	assert.DeepEqual(t, opt.MaxRequestBodySize, 4*1024*1024)
	assert.DeepEqual(t, opt.GetOnly, false)
	assert.DeepEqual(t, opt.DisableKeepalive, false)
	assert.DeepEqual(t, opt.DisablePrintRoute, false)
	assert.DeepEqual(t, opt.Network, "tcp")
	assert.DeepEqual(t, opt.ExitWaitTimeout, time.Second*5)
	assert.DeepEqual(t, opt.MaxKeepBodySize, 4*1024*1024)
	assert.DeepEqual(t, opt.H2C, false)
	assert.DeepEqual(t, opt.ReadBufferSize, 4096)
	assert.DeepEqual(t, opt.ALPN, false)
	assert.DeepEqual(t, opt.Registry, registry.NoopRegistry)
	assert.DeepEqual(t, opt.AutoReloadRender, false)
	assert.Assert(t, opt.RegistryInfo == nil)
	assert.DeepEqual(t, opt.AutoReloadRender, false)
	assert.DeepEqual(t, opt.AutoReloadInterval, time.Duration(0))
}
