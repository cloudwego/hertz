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
	"strconv"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/client/discovery"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestRoundRobinBalancer(t *testing.T) {
	balancer := NewRoundRobinBalancer()
	// nil
	ins := balancer.Pick(discovery.Result{})
	assert.DeepEqual(t, ins, nil)
	// test Name()
	assert.DeepEqual(t, "round_robin", balancer.Name())

	// empty instance
	e := discovery.Result{
		Instances: make([]discovery.Instance, 0),
		CacheKey:  "a",
	}
	balancer.Rebalance(e)
	ins = balancer.Pick(e)
	assert.DeepEqual(t, ins, nil)

	// one instance
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "127.0.0.1:8888", 0, nil),
	}
	e = discovery.Result{
		Instances: insList,
		CacheKey:  "b",
	}
	balancer.Rebalance(e)
	for i := 0; i < 100; i++ {
		ins = balancer.Pick(e)
		assert.DeepEqual(t, "127.0.0.1:8888", ins.Address().String())
	}

	// multi instances, weightSum > 0
	insList = []discovery.Instance{
		discovery.NewInstance("tcp", "127.0.0.1:8880", 0, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8881", 0, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8882", 0, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8883", 0, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8884", 0, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8885", 0, nil),
	}

	n := 10000
	e = discovery.Result{
		Instances: insList,
		CacheKey:  "c",
	}
	balancer.Rebalance(e)
	for i := 0; i < n; i++ {
		ins = balancer.Pick(e)
		addr := ins.Address()
		expectedAddr := "127.0.0.1:888" + strconv.Itoa(i%6)
		assert.DeepEqual(t, expectedAddr, addr.String())
	}
}
