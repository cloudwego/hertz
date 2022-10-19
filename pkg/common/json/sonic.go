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
