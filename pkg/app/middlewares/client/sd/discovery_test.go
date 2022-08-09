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

package sd

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/client/discovery"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func TestDiscovery(t *testing.T) {
	inss := []discovery.Instance{
		discovery.NewInstance("tcp", "127.0.0.1:8888", 10, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8889", 10, nil),
	}
	r := &discovery.SynthesizedResolver{
		TargetFunc: func(ctx context.Context, target *discovery.TargetInfo) string {
			return target.Host
		},
		ResolveFunc: func(ctx context.Context, key string) (discovery.Result, error) {
			return discovery.Result{CacheKey: "svc1", Instances: inss}, nil
		},
		NameFunc: func() string { return t.Name() },
	}

	mw := Discovery(r)
	checkMdw := func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
		t.Log(string(req.Host()))
		assert.Assert(t, string(req.Host()) == "127.0.0.1:8888" || string(req.Host()) == "127.0.0.1:8889")
		return nil
	}
	for i := 0; i < 10; i++ {
		req := &protocol.Request{}
		resp := &protocol.Response{}
		req.Options().Apply([]config.RequestOption{config.WithSD(true)})
		req.SetRequestURI("http://service_name")
		_ = mw(checkMdw)(context.Background(), req, resp)
	}
}
