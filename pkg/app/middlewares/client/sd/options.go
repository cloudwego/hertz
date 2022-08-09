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
	"fmt"
	"net"
	"strings"

	"github.com/cloudwego/hertz/pkg/app/client/discovery"
	"github.com/cloudwego/hertz/pkg/app/client/loadbalance"
	"github.com/cloudwego/hertz/pkg/app/server/registry"
)

// ServiceDiscoveryOptions service discovery option for client
type ServiceDiscoveryOptions struct {
	// Resolver is used to client discovery
	Resolver discovery.Resolver

	// Balancer is used to client load balance
	Balancer loadbalance.Loadbalancer

	// LbOpts LoadBalance option
	LbOpts loadbalance.Options
}

func (o *ServiceDiscoveryOptions) Apply(opts []ServiceDiscoveryOption) {
	for _, op := range opts {
		op.F(o)
	}
}

type ServiceDiscoveryOption struct {
	F func(o *ServiceDiscoveryOptions)
}

// WithCustomizedAddrs specifies the target instance addresses when doing service discovery.
// It overwrites the results from the Resolver
func WithCustomizedAddrs(addrs ...string) ServiceDiscoveryOption {
	return ServiceDiscoveryOption{
		F: func(o *ServiceDiscoveryOptions) {
			var ins []discovery.Instance
			for _, addr := range addrs {
				if _, err := net.ResolveTCPAddr("tcp", addr); err == nil {
					ins = append(ins, discovery.NewInstance("tcp", addr, registry.DefaultWeight, nil))
					continue
				}
				if _, err := net.ResolveUnixAddr("unix", addr); err == nil {
					ins = append(ins, discovery.NewInstance("unix", addr, registry.DefaultWeight, nil))
					continue
				}
				panic(fmt.Errorf("WithCustomizedAddrs: invalid '%s'", addr))
			}
			if len(ins) == 0 {
				panic("WithCustomizedAddrs() requires at least one argument")
			}

			targets := strings.Join(addrs, ",")
			o.Resolver = &discovery.SynthesizedResolver{
				ResolveFunc: func(ctx context.Context, key string) (discovery.Result, error) {
					return discovery.Result{
						CacheKey:  "fixed",
						Instances: ins,
					}, nil
				},
				NameFunc: func() string { return targets },
				TargetFunc: func(ctx context.Context, target *discovery.TargetInfo) string {
					return targets
				},
			}
		},
	}
}

// WithLoadBalanceOptions  sets Loadbalancer and loadbalance options for hertz client
func WithLoadBalanceOptions(lb loadbalance.Loadbalancer, options loadbalance.Options) ServiceDiscoveryOption {
	return ServiceDiscoveryOption{F: func(o *ServiceDiscoveryOptions) {
		o.LbOpts = options
		o.Balancer = lb
	}}
}
