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

	"github.com/cloudwego/hertz/pkg/app/client/loadbalance"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestWithCustomizedAddrs(t *testing.T) {
	var options []ServiceDiscoveryOption
	options = append(options, WithCustomizedAddrs("127.0.0.1:8080", "/tmp/unix_ss"))
	opts := &ServiceDiscoveryOptions{}
	opts.Apply(options)
	assert.Assert(t, opts.Resolver.Name() == "127.0.0.1:8080,/tmp/unix_ss")
	res, err := opts.Resolver.Resolve(context.Background(), "")
	assert.Assert(t, err == nil)
	assert.Assert(t, res.Instances[0].Address().String() == "127.0.0.1:8080")
	assert.Assert(t, res.Instances[1].Address().String() == "/tmp/unix_ss")
}

func TestWithLoadBalanceOptions(t *testing.T) {
	balance := loadbalance.NewWeightedBalancer()
	var options []ServiceDiscoveryOption
	options = append(options, WithLoadBalanceOptions(balance, loadbalance.DefaultLbOpts))
	opts := &ServiceDiscoveryOptions{}
	opts.Apply(options)
	assert.Assert(t, opts.Balancer.Name() == "weight_random")
}
