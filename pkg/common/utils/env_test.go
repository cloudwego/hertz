/*
 * Copyright 2024 CloudWeGo Authors
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
	"os"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

// TestGetBoolFromEnv tests the functionality of getting boolean values from environment variables
func TestGetBoolFromEnv(t *testing.T) {
	// Test case: environment variable does not exist
	nonExistKey := "HERTZ_TEST_NON_EXIST_KEY"
	os.Unsetenv(nonExistKey) // Ensure the environment variable doesn't exist
	val, err := GetBoolFromEnv(nonExistKey)
	assert.NotNil(t, err)
	assert.DeepEqual(t, false, val)
	assert.DeepEqual(t, "env not exist", err.Error())

	// Test case: environment variable value is "true"
	trueKey := "HERTZ_TEST_TRUE_KEY"
	os.Setenv(trueKey, "true")
	val, err = GetBoolFromEnv(trueKey)
	assert.Nil(t, err)
	assert.DeepEqual(t, true, val)

	// Test case: environment variable value is "false"
	falseKey := "HERTZ_TEST_FALSE_KEY"
	os.Setenv(falseKey, "false")
	val, err = GetBoolFromEnv(falseKey)
	assert.Nil(t, err)
	assert.DeepEqual(t, false, val)

	// Test case: environment variable value is "1" (should parse as true)
	oneKey := "HERTZ_TEST_ONE_KEY"
	os.Setenv(oneKey, "1")
	val, err = GetBoolFromEnv(oneKey)
	assert.Nil(t, err)
	assert.DeepEqual(t, true, val)

	// Test case: environment variable value is "0" (should parse as false)
	zeroKey := "HERTZ_TEST_ZERO_KEY"
	os.Setenv(zeroKey, "0")
	val, err = GetBoolFromEnv(zeroKey)
	assert.Nil(t, err)
	assert.DeepEqual(t, false, val)

	// Test case: environment variable value contains whitespace
	spaceKey := "HERTZ_TEST_SPACE_KEY"
	os.Setenv(spaceKey, "  true  ")
	val, err = GetBoolFromEnv(spaceKey)
	assert.Nil(t, err)
	assert.DeepEqual(t, true, val)

	// Test case: environment variable value is an invalid boolean
	invalidKey := "HERTZ_TEST_INVALID_KEY"
	os.Setenv(invalidKey, "not-a-bool")
	val, err = GetBoolFromEnv(invalidKey)
	assert.NotNil(t, err)
	assert.DeepEqual(t, false, val)

	// Cleanup test environment variables
	os.Unsetenv(trueKey)
	os.Unsetenv(falseKey)
	os.Unsetenv(oneKey)
	os.Unsetenv(zeroKey)
	os.Unsetenv(spaceKey)
	os.Unsetenv(invalidKey)
}