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

package registry

import "net"

const (
	DefaultWeight = 10
)

// Registry is extension interface of service registry.
type Registry interface {
	Register(info *Info) error
	Deregister(info *Info) error
}

// Info is used for registry.
// The fields are just suggested, which is used depends on design.
type Info struct {
	// ServiceName will be set in hertz by default
	ServiceName string
	// Addr will be set in hertz by default
	Addr net.Addr
	// Weight will be set in hertz by default
	Weight int
	// extend other infos with Tags.
	Tags map[string]string
}

// NoopRegistry is an empty implement of Registry
var NoopRegistry Registry = &noopRegistry{}

// NoopRegistry
type noopRegistry struct{}

func (e noopRegistry) Register(*Info) error {
	return nil
}

func (e noopRegistry) Deregister(*Info) error {
	return nil
}
