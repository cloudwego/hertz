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
