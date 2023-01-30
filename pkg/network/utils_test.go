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

package network

import (
	"os"
	"runtime"
	"testing"
)

func TestUnlinkUdsFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tmp := "tmpFile"
	var err error

	err = UnlinkUdsFile("unix", tmp)

	if err == nil {
		t.Errorf("should have error when unlinking a nonexistent file")
	}

	os.Create(tmp)

	err = UnlinkUdsFile("unix", tmp)
	if err != nil {
		t.Errorf("unlink file failed: %s", err.Error())
	}

	isExist, _ := pathExists(tmp)

	if isExist {
		t.Errorf("unlink file failed, file still exist")
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
