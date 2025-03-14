/*
 * Copyright 2025 CloudWeGo Authors
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

package testutils

import (
	"net"
	"path"
	"reflect"
	"unsafe"

	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/route"
)

var (
	engineType    = reflect.TypeOf((*route.Engine)(nil)).Elem()
	transportType = reflect.TypeOf((*network.Transporter)(nil)).Elem()
)

func unwrapValue(rv reflect.Value) reflect.Value {
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}
	return rv
}

func getEngine(rv reflect.Value) reflect.Value {
	rv = unwrapValue(rv)
	if rv.Type() == engineType {
		return rv
	}
	for i := 0; i < rv.NumField(); i++ {
		f := unwrapValue(rv.Field(i))
		if f.Type() == engineType {
			return f
		}
	}
	panic("not found *route.Engine")
}

func getNetworkTransporter(rv reflect.Value) reflect.Value {
	rv = unwrapValue(rv)
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if f.Type() == transportType {
			return f
		}
	}
	panic("not found network.Transporter")
}

func getUnexportedField(f reflect.Value) interface{} {
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.Addr().Pointer())).Elem().Interface()
}

// GetListener extracts net.Listener from network.Transporter in route.Engine
func GetListener(v interface{}) net.Listener {
	rv := getEngine(reflect.ValueOf(v)) // *route.Engine
	rv = getNetworkTransporter(rv)      // network.Transporter

	// implemented by network/netpoll & standard
	type ListenerIface interface {
		Listener() net.Listener
	}

	// NOTE: do not access net.Listener directly, it may cause race issue.
	// use `Listener()` method
	trans := getUnexportedField(rv)
	if p, ok := trans.(ListenerIface); ok {
		return p.Listener()
	}
	panic("network.Transporter has no Listener() method")
}

// GetListenerAddr is shortcut of GetListener(e).Addr().String()
func GetListenerAddr(v interface{}) string {
	return GetListener(v).Addr().String()
}

// GetURL ...
func GetURL(v interface{}, p string) string {
	return "http://" + path.Join(GetListenerAddr(v), p)
}
