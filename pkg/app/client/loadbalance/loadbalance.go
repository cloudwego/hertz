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

package loadbalance

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/client/discovery"
)

// Loadbalancer picks instance for the given service discovery result.
type Loadbalancer interface {
	// Pick is used to select an instance according to discovery result
	Pick(discovery.Result) discovery.Instance

	// Rebalance is used to refresh the cache of load balance's information
	Rebalance(discovery.Result)

	// Delete is used to delete the cache of load balance's information when it is expired
	Delete(string)

	// Name returns the name of the Loadbalancer.
	Name() string
}

const (
	DefaultRefreshInterval = 5 * time.Second
	DefaultExpireInterval  = 15 * time.Second
)

var DefaultLbOpts = Options{
	RefreshInterval: DefaultRefreshInterval,
	ExpireInterval:  DefaultExpireInterval,
}

// Options for LoadBalance option
type Options struct {
	// refresh discovery result timely
	RefreshInterval time.Duration

	// Balancer expire check interval
	// we need remove idle Balancers for resource saving
	ExpireInterval time.Duration
}

// Check checks option's param
func (v *Options) Check() {
	if v.RefreshInterval <= 0 {
		v.RefreshInterval = DefaultRefreshInterval
	}
	if v.ExpireInterval <= 0 {
		v.ExpireInterval = DefaultExpireInterval
	}
}
