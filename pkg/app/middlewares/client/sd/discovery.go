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

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/app/client/discovery"
	"github.com/cloudwego/hertz/pkg/app/client/loadbalance"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// Discovery will construct a middleware with BalancerFactory.
func Discovery(resolver discovery.Resolver, opts ...ServiceDiscoveryOption) client.Middleware {
	options := &ServiceDiscoveryOptions{
		Balancer: loadbalance.NewWeightedBalancer(),
		LbOpts:   loadbalance.DefaultLbOpts,
		Resolver: resolver,
	}
	options.Apply(opts)

	lbConfig := loadbalance.Config{
		Resolver: options.Resolver,
		Balancer: options.Balancer,
		LbOpts:   options.LbOpts,
	}

	f := loadbalance.NewBalancerFactory(lbConfig)
	return func(next client.Endpoint) client.Endpoint {
		return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
			if req.Options() != nil && req.Options().IsSD() {
				ins, err := f.GetInstance(ctx, req)
				if err != nil {
					return err
				}
				req.SetHost(ins.Address().String())
			}
			return next(ctx, req, resp)
		}
	}
}
