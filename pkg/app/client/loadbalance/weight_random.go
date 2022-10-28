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
	"sync"

	"github.com/bytedance/gopkg/lang/fastrand"
	"github.com/cloudwego/hertz/pkg/app/client/discovery"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"golang.org/x/sync/singleflight"
)

type weightedBalancer struct {
	cachedWeightInfo sync.Map
	sfg              singleflight.Group
}

type weightInfo struct {
	instances []discovery.Instance
	entries   []int
	weightSum int
}

// NewWeightedBalancer creates a loadbalancer using weighted-random algorithm.
func NewWeightedBalancer() Loadbalancer {
	lb := &weightedBalancer{}
	return lb
}

func (wb *weightedBalancer) calcWeightInfo(e discovery.Result) *weightInfo {
	w := &weightInfo{
		instances: make([]discovery.Instance, len(e.Instances)),
		weightSum: 0,
		entries:   make([]int, len(e.Instances)),
	}

	var cnt int

	for idx := range e.Instances {
		weight := e.Instances[idx].Weight()
		if weight > 0 {
			w.instances[cnt] = e.Instances[idx]
			w.entries[cnt] = weight
			w.weightSum += weight
			cnt++
		} else {
			hlog.SystemLogger().Warnf("Invalid weight=%d on instance address=%s", weight, e.Instances[idx].Address())
		}
	}

	w.instances = w.instances[:cnt]

	return w
}

// Pick implements the Loadbalancer interface.
func (wb *weightedBalancer) Pick(e discovery.Result) discovery.Instance {
	wi, ok := wb.cachedWeightInfo.Load(e.CacheKey)
	if !ok {
		wi, _, _ = wb.sfg.Do(e.CacheKey, func() (interface{}, error) {
			return wb.calcWeightInfo(e), nil
		})
		wb.cachedWeightInfo.Store(e.CacheKey, wi)
	}

	w := wi.(*weightInfo)
	if w.weightSum <= 0 {
		return nil
	}

	weight := fastrand.Intn(w.weightSum)
	for i := 0; i < len(w.instances); i++ {
		weight -= w.entries[i]
		if weight < 0 {
			return w.instances[i]
		}
	}

	return nil
}

// Rebalance implements the Loadbalancer interface.
func (wb *weightedBalancer) Rebalance(e discovery.Result) {
	wb.cachedWeightInfo.Store(e.CacheKey, wb.calcWeightInfo(e))
}

// Delete implements the Loadbalancer interface.
func (wb *weightedBalancer) Delete(cacheKey string) {
	wb.cachedWeightInfo.Delete(cacheKey)
}

func (wb *weightedBalancer) Name() string {
	return "weight_random"
}
