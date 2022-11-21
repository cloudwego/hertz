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

package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/hertz/cmd/hz/meta"
	"github.com/cloudwego/hertz/cmd/hz/util/logs"
	gv "github.com/hashicorp/go-version"
)

const ThriftgoMiniVersion = "v0.2.0"

// QueryVersion will query the version of the corresponding executable.
func QueryVersion(exe string) (version string, err error) {
	var buf strings.Builder
	cmd := &exec.Cmd{
		Path: exe,
		Args: []string{
			exe, "--version",
		},
		Stdin:  os.Stdin,
		Stdout: &buf,
		Stderr: &buf,
	}
	err = cmd.Run()
	if err == nil {
		version = strings.Split(buf.String(), " ")[1]
		if strings.HasSuffix(version, "\n") {
			version = version[:len(version)-1]
		}
	}
	return
}

// ShouldUpdate will return "true" when current is lower than latest.
func ShouldUpdate(current, latest string) bool {
	cv, err := gv.NewVersion(current)
	if err != nil {
		return false
	}
	lv, err := gv.NewVersion(latest)
	if err != nil {
		return false
	}

	return cv.Compare(lv) < 0
}

// InstallAndCheckThriftgo will automatically install thriftgo and judge whether it is installed successfully.
func InstallAndCheckThriftgo() error {
	exe, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("can not find tool 'go': %v", err)
	}
	var buf strings.Builder
	cmd := &exec.Cmd{
		Path: exe,
		Args: []string{
			exe, "install", "github.com/cloudwego/thriftgo@latest",
		},
		Stdin:  os.Stdin,
		Stdout: &buf,
		Stderr: &buf,
	}

	done := make(chan error)
	logs.Infof("installing thriftgo automatically")
	go func() {
		done <- cmd.Run()
	}()
	select {
	case err = <-done:
		if err != nil {
			return fmt.Errorf("can not install thriftgo, err: %v. Please install it manual, and make sure the version of thriftgo is greater than v0.2.0", cmd.Stderr)
		}
	case <-time.After(time.Second * 30):
		return fmt.Errorf("install thriftgo time out.Please install it manual, and make sure the version of thriftgo is greater than v0.2.0")
	}

	exist, err := CheckCompiler(meta.TpCompilerThrift)
	if err != nil {
		return fmt.Errorf("check %s exist failed, err: %v", meta.TpCompilerThrift, err)
	}
	if !exist {
		return fmt.Errorf("install thriftgo failed. Please install it manual, and make sure the version of thriftgo is greater than v0.2.0")
	}

	return nil
}

// CheckCompiler will check if the tool exists.
func CheckCompiler(tool string) (bool, error) {
	path, err := exec.LookPath(tool)
	if err != nil {
		goPath, err := GetGOPATH()
		if err != nil {
			return false, fmt.Errorf("get 'GOPATH' failed for find %s : %v", tool, path)
		}
		path = filepath.Join(goPath, "bin", tool)
	}

	isExist, err := PathExist(path)
	if err != nil {
		return false, fmt.Errorf("can not check %s exist, err: %v", tool, err)
	}
	if !isExist {
		return false, nil
	}

	return true, nil
}

// CheckAndUpdateThriftgo checks the version of thriftgo and updates the tool to the latest version if its version is less than v0.2.0.
func CheckAndUpdateThriftgo() error {
	path, err := exec.LookPath(meta.TpCompilerThrift)
	if err != nil {
		return fmt.Errorf("can not find %s", meta.TpCompilerThrift)
	}
	curVersion, err := QueryVersion(path)
	logs.Infof("current thriftgo version is %s", curVersion)
	if ShouldUpdate(curVersion, ThriftgoMiniVersion) {
		logs.Infof(" current thriftgo version is less than v0.2.0, so update thriftgo version")
		err = InstallAndCheckThriftgo()
		if err != nil {
			return fmt.Errorf("update thriftgo version failed, err: %v", err)
		}
	}

	return nil
}
