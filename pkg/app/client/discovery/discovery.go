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

package discovery

import (
	"context"
	"net"

	"github.com/cloudwego/hertz/pkg/app/server/registry"
	"github.com/cloudwego/hertz/pkg/common/utils"
)

type TargetInfo struct {
	Host string
	Tags map[string]string
}

type Resolver interface {
	// Target should return a description for the given target that is suitable for being a key for cache.
	Target(ctx context.Context, target *TargetInfo) string

	// Resolve returns a list of instances for the given description of a target.
	Resolve(ctx context.Context, desc string) (Result, error)

	// Name returns the name of the resolver.
	Name() string
}

// SynthesizedResolver synthesizes a Resolver using a resolve function.
type SynthesizedResolver struct {
	TargetFunc  func(ctx context.Context, target *TargetInfo) string
	ResolveFunc func(ctx context.Context, key string) (Result, error)
	NameFunc    func() string
}

func (sr SynthesizedResolver) Target(ctx context.Context, target *TargetInfo) string {
	if sr.TargetFunc == nil {
		return ""
	}
	return sr.TargetFunc(ctx, target)
}

func (sr SynthesizedResolver) Resolve(ctx context.Context, key string) (Result, error) {
	return sr.ResolveFunc(ctx, key)
}

// Name implements the Resolver interface
func (sr SynthesizedResolver) Name() string {
	if sr.NameFunc == nil {
		return ""
	}
	return sr.NameFunc()
}

// Instance contains information of an instance from the target service.
type Instance interface {
	Address() net.Addr
	Weight() int
	Tag(key string) (value string, exist bool)
}

type instance struct {
	addr   net.Addr
	weight int
	tags   map[string]string
}

func (i *instance) Address() net.Addr {
	return i.addr
}

func (i *instance) Weight() int {
	if i.weight > 0 {
		return i.weight
	}
	return registry.DefaultWeight
}

func (i *instance) Tag(key string) (value string, exist bool) {
	value, exist = i.tags[key]
	return
}

// NewInstance creates an Instance using the given network, address and tags
func NewInstance(network, address string, weight int, tags map[string]string) Instance {
	return &instance{
		addr:   utils.NewNetAddr(network, address),
		weight: weight,
		tags:   tags,
	}
}

// Result contains the result of service discovery process.
// the instance list can/should be cached and CacheKey can be used to map the instance list in cache.
type Result struct {
	CacheKey  string
	Instances []Instance
}
