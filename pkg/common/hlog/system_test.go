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

package hlog

import (
	"context"
	"testing"
)

func TestSysLogger(t *testing.T) {
	sysLogger.Trace("trace work")
	sysLogger.Debug("received work order")
	sysLogger.Info("starting work")
	sysLogger.Notice("something happens in work")
	sysLogger.Warn("work may fail")
	sysLogger.Error("work failed")
	// Output:
	// [Trace] trace work
	// [Debug] received work order
	// [Info] starting work
	// [Notice] something happens in work
	// [Warn] work may fail
	// [Error] work failed
}

func TestSysFormatLogger(t *testing.T) {
	work := "work"
	sysLogger.Tracef("trace %s", work)
	sysLogger.Debugf("received %s order", work)
	sysLogger.Infof("starting %s", work)
	sysLogger.Noticef("something happens in %s", work)
	sysLogger.Warnf("%s may fail", work)
	sysLogger.Errorf("%s failed", work)
	// Output:
	// [Trace] trace work
	// [Debug] received work order
	// [Info] starting work
	// [Notice] something happens in work
	// [Warn] work may fail
	// [Error] work failed
}

func TestSysCtxLogger(t *testing.T) {
	ctx := context.Background()
	work := "work"
	sysLogger.CtxTracef(ctx, "trace %s", work)
	sysLogger.CtxDebugf(ctx, "received %s order", work)
	sysLogger.CtxInfof(ctx, "starting %s", work)
	sysLogger.CtxNoticef(ctx, "something happens in %s", work)
	sysLogger.CtxWarnf(ctx, "%s may fail", work)
	sysLogger.CtxErrorf(ctx, "%s failed", work)
	// Output:
	// [Trace] trace work
	// [Debug] received work order
	// [Info] starting work
	// [Notice] something happens in work
	// [Warn] work may fail
	// [Error] work failed
}
