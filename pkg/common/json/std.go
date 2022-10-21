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

//go:build stdjson || !(amd64 && (linux || windows || darwin))
// +build stdjson !amd64 !linux,!windows,!darwin

package json

import "encoding/json"

// Name is the name of the effective json package.
const Name = "encoding/json"

var (
	// Marshal is standard implementation exported by hertz which is used by rendering.
	Marshal = json.Marshal
	// Unmarshal is standard implementation exported by hertz which is used by binding.
	Unmarshal = json.Unmarshal
	// MarshalIndent is standard implementation exported by hertz.
	MarshalIndent = json.MarshalIndent
	// NewDecoder is standard implementation exported by hertz.
	NewDecoder = json.NewDecoder
	// NewEncoder is standard implementation exported by hertz.
	NewEncoder = json.NewEncoder
)
