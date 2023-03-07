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
//go:build !tinygo
// +build !tinygo

package server

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// Default implementation for signal waiter.
// SIGTERM triggers immediately close.
// SIGHUP|SIGINT triggers graceful shutdown.
func waitSignal(errCh chan error) error {
	signalToNotify := []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM}
	if signal.Ignored(syscall.SIGHUP) {
		signalToNotify = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, signalToNotify...)

	select {
	case sig := <-signals:
		switch sig {
		case syscall.SIGTERM:
			// force exit
			return errors.New(sig.String()) // nolint
		case syscall.SIGHUP, syscall.SIGINT:
			hlog.SystemLogger().Infof("Received signal: %s\n", sig)
			// graceful shutdown
			return nil
		}
	case err := <-errCh:
		// error occurs, exit immediately
		return err
	}

	return nil
}
