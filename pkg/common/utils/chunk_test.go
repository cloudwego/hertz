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

// TestParseChunkSize tests the chunk size parsing functionality with various input formats
func TestParseChunkSize(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectSize  int
		expectError bool
	}{
		{
			name:        "valid hex with CRLF",
			input:       "a\r\n",
			expectSize:  10,
			expectError: false,
		},
		{
			name:        "valid hex with spaces and CRLF",
			input:       "10   \r\n",
			expectSize:  16,
			expectError: false,
		},
		{
			name:        "empty input",
			input:       "",
			expectSize:  -1,
			expectError: true,
		},
		{
			name:        "missing CR",
			input:       "a\n",
			expectSize:  -1,
			expectError: true,
		},
		{
			name:        "missing LF",
			input:       "a\r",
			expectSize:  -1,
			expectError: true,
		},
		{
			name:        "invalid hex",
			input:       "xyz\r\n",
			expectSize:  -1,
			expectError: true,
		},
		{
			name:        "invalid char after size",
			input:       "a#\r\n",
			expectSize:  -1,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := mock.NewZeroCopyReader(tc.input)
			size, err := ParseChunkSize(r)

			if tc.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.DeepEqual(t, tc.expectSize, size)
		})
	}
}

// TestSkipCRLF tests the CRLF skipping functionality with different input sequences
func TestSkipCRLF(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "valid CRLF",
			input:       "\r\n",
			expectError: false,
		},
		{
			name:        "only CR",
			input:       "\r",
			expectError: true,
		},
		{
			name:        "only LF",
			input:       "\n",
			expectError: true,
		},
		{
			name:        "empty input",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid sequence",
			input:       "ab",
			expectError: true,
		},
		{
			name:        "CRLF with extra data",
			input:       "\r\ndata",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := mock.NewZeroCopyReader(tc.input)
			err := SkipCRLF(r)

			if tc.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				if len(tc.input) > 2 {
					// Verify that CRLF was correctly skipped by checking remaining data
					remaining, _ := r.Peek(len(tc.input) - 2)
					assert.DeepEqual(t, tc.input[2:], string(remaining))
				}
			}
		})
	}
}
