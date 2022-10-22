// Copyright 2022 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

//go:build (linux || windows || darwin) && amd64
// +build linux windows darwin
// +build amd64

package json

import "github.com/bytedance/sonic"

// Name is the name of the effective json package.
const Name = "sonic"

var (
	json = sonic.ConfigStd
	// Marshal is sonic implementation exported by hertz which is used by rendering.
	Marshal = json.Marshal
	// Unmarshal is sonic implementation exported by hertz which is used by binding.
	Unmarshal = json.Unmarshal
	// MarshalIndent is sonic implementation exported by hertz.
	MarshalIndent = json.MarshalIndent
	// NewDecoder is sonic implementation exported by hertz.
	NewDecoder = json.NewDecoder
	// NewEncoder is sonic implementation exported by hertz.
	NewEncoder = json.NewEncoder
)
