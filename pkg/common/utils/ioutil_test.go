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

package utils

import (
	"bytes"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/network"
)

func TestIoutilCopyBuffer(t *testing.T) {
	var writeBuffer bytes.Buffer
	src := bytes.NewBufferString("hertz is very good!!!")
	dst := network.NewWriter(&writeBuffer)
	var buf []byte
	// src.Len() will change, when use src.read(p []byte)
	srcLen := int64(src.Len())
	written, err := CopyBuffer(dst, src, buf)

	assert.DeepEqual(t, written, srcLen)
	assert.DeepEqual(t, err, nil)
}

func TestIoutilCopyBufferWithNilBuffer(t *testing.T) {
	var writeBuffer bytes.Buffer
	src := bytes.NewBufferString("hertz is very good!!!")
	dst := network.NewWriter(&writeBuffer)
	// src.Len() will change, when use src.read(p []byte)
	srcLen := int64(src.Len())
	written, err := CopyBuffer(dst, src, nil)

	assert.DeepEqual(t, written, srcLen)
	assert.DeepEqual(t, err, nil)
}

func TestIoutilCopyZeroAlloc(t *testing.T) {
	var writeBuffer bytes.Buffer
	src := bytes.NewBufferString("hertz is very good!!!")
	dst := network.NewWriter(&writeBuffer)
	srcLen := int64(src.Len())
	written, err := CopyZeroAlloc(dst, src)
	if written != srcLen {
		t.Fatalf("Unexpected written: %d. Expecting: %d", written, srcLen)
	}
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}
