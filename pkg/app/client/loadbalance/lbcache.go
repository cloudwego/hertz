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
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client/discovery"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"
	"golang.org/x/sync/singleflight"
)

type cacheResult struct {
	res         atomic.Value // newest and previous discovery result
	expire      int32        // 0 = normal, 1 = expire and collect next ticker
	serviceName string       // service psm
}

var (
	balancerFactories    sync.Map // key: resolver name + load-balancer name
	balancerFactoriesSfg singleflight.Group
)

func cacheKey(resolver, balancer string, opts Options) string {
	return fmt.Sprintf("%s|%s|{%s %s}", resolver, balancer, opts.RefreshInterval, opts.ExpireInterval)
}

type BalancerFactory struct {
	opts     Options
	cache    sync.Map // key -> LoadBalancer
	resolver discovery.Resolver
	balancer Loadbalancer
	sfg      singleflight.Group
}

type Config struct {
	Resolver discovery.Resolver
	Balancer Loadbalancer
	LbOpts   Options
}

// NewBalancerFactory get or create a balancer with given target.
// If it has the same key(resolver.Target(target)), we will cache and reuse the Balance.
func NewBalancerFactory(config Config) *BalancerFactory {
	config.LbOpts.Check()
	uniqueKey := cacheKey(config.Resolver.Name(), config.Balancer.Name(), config.LbOpts)
	val, ok := balancerFactories.Load(uniqueKey)
	if ok {
		return val.(*BalancerFactory)
	}
	val, _, _ = balancerFactoriesSfg.Do(uniqueKey, func() (interface{}, error) {
		b := &BalancerFactory{
			opts:     config.LbOpts,
			resolver: config.Resolver,
			balancer: config.Balancer,
		}
		go b.watcher()
		go b.refresh()
		balancerFactories.Store(uniqueKey, b)
		return b, nil
	})
	return val.(*BalancerFactory)
}

// watch expired balancer
func (b *BalancerFactory) watcher() {
	for range time.Tick(b.opts.ExpireInterval) {
		b.cache.Range(func(key, value interface{}) bool {
			cache := value.(*cacheResult)
			if atomic.CompareAndSwapInt32(&cache.expire, 0, 1) {
				// 1. set expire flag
				// 2. wait next ticker for collect, maybe the balancer is used again
				// (avoid being immediate delete the balancer which had been created recently)
			} else {
				b.cache.Delete(key)
				b.balancer.Delete(key.(string))
			}
			return true
		})
	}
}

// cache key with resolver name prefix avoid conflict for balancer
func renameResultCacheKey(res *discovery.Result, resolverName string) {
	res.CacheKey = resolverName + ":" + res.CacheKey
}

// refresh is used to update service discovery information periodically.
func (b *BalancerFactory) refresh() {
	for range time.Tick(b.opts.RefreshInterval) {
		b.cache.Range(func(key, value interface{}) bool {
			res, err := b.resolver.Resolve(context.Background(), key.(string))
			if err != nil {
				hlog.SystemLogger().Warnf("resolver refresh failed, key=%s error=%s", key, err.Error())
				return true
			}
			renameResultCacheKey(&res, b.resolver.Name())
			cache := value.(*cacheResult)
			cache.res.Store(res)
			atomic.StoreInt32(&cache.expire, 0)
			b.balancer.Rebalance(res)
			return true
		})
	}
}

func (b *BalancerFactory) GetInstance(ctx context.Context, req *protocol.Request) (discovery.Instance, error) {
	cacheRes, err := b.getCacheResult(ctx, req)
	if err != nil {
		return nil, err
	}
	atomic.StoreInt32(&cacheRes.expire, 0)
	ins := b.balancer.Pick(cacheRes.res.Load().(discovery.Result))
	if ins == nil {
		hlog.SystemLogger().Errorf("null instance. serviceName: %s, options: %v", string(req.Host()), req.Options())
		return nil, errors.NewPublic("instance not found")
	}
	return ins, nil
}

func (b *BalancerFactory) getCacheResult(ctx context.Context, req *protocol.Request) (*cacheResult, error) {
	target := b.resolver.Target(ctx, &discovery.TargetInfo{Host: string(req.Host()), Tags: req.Options().Tags()})
	cr, existed := b.cache.Load(target)
	if existed {
		return cr.(*cacheResult), nil
	}
	cr, err, _ := b.sfg.Do(target, func() (interface{}, error) {
		cache := &cacheResult{
			serviceName: string(req.Host()),
		}
		res, err := b.resolver.Resolve(ctx, target)
		if err != nil {
			return cache, err
		}
		renameResultCacheKey(&res, b.resolver.Name())
		cache.res.Store(res)
		atomic.StoreInt32(&cache.expire, 0)
		b.balancer.Rebalance(res)
		b.cache.Store(target, cache)
		return cache, nil
	})
	if err != nil {
		return nil, err
	}
	return cr.(*cacheResult), nil
}
