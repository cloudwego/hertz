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
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/test/mock"
)

func TestChunkParseChunkSizeGetCorrect(t *testing.T) {
	// iterate the hexMap, and judge the difference between dec and ParseChunkSize
	hexMap := map[int]string{0: "0", 10: "a", 100: "64", 1000: "3e8"}
	for dec, hex := range hexMap {
		chunkSizeBody := hex + "\r\n"
		zr := mock.NewZeroCopyReader(chunkSizeBody)
		chunkSize, err := ParseChunkSize(zr)
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, chunkSize, dec)
	}
}

func TestChunkParseChunkSizeGetError(t *testing.T) {
	// test err from -----n, err := bytesconv.ReadHexInt(r)-----
	chunkSizeBody := ""
	zr := mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err := ParseChunkSize(zr)
	assert.NotNil(t, err)
	assert.DeepEqual(t, -1, chunkSize)
	// test err from -----c, err := r.ReadByte()-----
	chunkSizeBody = "0"
	zr = mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err = ParseChunkSize(zr)
	assert.NotNil(t, err)
	assert.DeepEqual(t, -1, chunkSize)
	// test err from -----c, err := r.ReadByte()-----
	chunkSizeBody = "0" + "\r"
	zr = mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err = ParseChunkSize(zr)
	assert.NotNil(t, err)
	assert.DeepEqual(t, -1, chunkSize)
	// test err from -----c, err := r.ReadByte()-----
	chunkSizeBody = "0" + "\r" + "\r"
	zr = mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err = ParseChunkSize(zr)
	assert.NotNil(t, err)
	assert.DeepEqual(t, -1, chunkSize)
}

func TestChunkParseChunkSizeCorrectWhiteSpace(t *testing.T) {
	// test the whitespace
	whiteSpace := ""
	for i := 0; i < 10; i++ {
		whiteSpace += " "
		chunkSizeBody := "0" + whiteSpace + "\r\n"
		zr := mock.NewZeroCopyReader(chunkSizeBody)
		chunkSize, err := ParseChunkSize(zr)
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, 0, chunkSize)
	}
}

func TestChunkParseChunkSizeNonCRLF(t *testing.T) {
	// test non-"\r\n"
	chunkSizeBody := "0" + "\n\r"
	zr := mock.NewZeroCopyReader(chunkSizeBody)
	chunkSize, err := ParseChunkSize(zr)
	assert.DeepEqual(t, true, err != nil)
	assert.DeepEqual(t, -1, chunkSize)
}

func TestChunkReadTrueCRLF(t *testing.T) {
	CRLF := "\r\n"
	zr := mock.NewZeroCopyReader(CRLF)
	err := SkipCRLF(zr)
	assert.DeepEqual(t, nil, err)
}

func TestChunkReadFalseCRLF(t *testing.T) {
	CRLF := "\n\r"
	zr := mock.NewZeroCopyReader(CRLF)
	err := SkipCRLF(zr)
	assert.DeepEqual(t, errBrokenChunk, err)
}
