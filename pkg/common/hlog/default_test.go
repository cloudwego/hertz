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
	"log"
	"os"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func initTestLogger() {
	logger = &defaultLogger{
		stdlog: log.New(os.Stderr, "", 0),
		depth:  4,
	}
}

type byteSliceWriter struct {
	b []byte
}

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func TestDefaultLogger(t *testing.T) {
	initTestLogger()

	var w byteSliceWriter
	SetOutput(&w)

	Trace("trace work")
	Debug("received work order")
	Info("starting work")
	Notice("something happens in work")
	Warn("work may fail")
	Error("work failed")

	assert.DeepEqual(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Notice] something happens in work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n", string(w.b))
}

func TestDefaultFormatLogger(t *testing.T) {
	initTestLogger()

	var w byteSliceWriter
	SetOutput(&w)

	work := "work"
	Tracef("trace %s", work)
	Debugf("received %s order", work)
	Infof("starting %s", work)
	Noticef("something happens in %s", work)
	Warnf("%s may fail", work)
	Errorf("%s failed", work)

	assert.DeepEqual(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Notice] something happens in work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n", string(w.b))
}

func TestCtxLogger(t *testing.T) {
	initTestLogger()

	var w byteSliceWriter
	SetOutput(&w)

	ctx := context.Background()
	work := "work"
	CtxTracef(ctx, "trace %s", work)
	CtxDebugf(ctx, "received %s order", work)
	CtxInfof(ctx, "starting %s", work)
	CtxNoticef(ctx, "something happens in %s", work)
	CtxWarnf(ctx, "%s may fail", work)
	CtxErrorf(ctx, "%s failed", work)

	assert.DeepEqual(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Notice] something happens in work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n", string(w.b))
}

func TestFormatLoggerWithEscapedCharacters(t *testing.T) {
	initTestLogger()

	var w byteSliceWriter
	SetOutput(&w)

	Infof("http://localhost:8080/ping?f=http://localhost:3000/hello?c=%E5%A4%A7hi%E5%93%A6%E5%95%8A%E8%AF%B4%E5%BE%97%E5%A5%BD")
	assert.DeepEqual(t, "[Info] http://localhost:8080/ping?f=http://localhost:3000/hello?c=%E5%A4%A7hi%E5%93%A6%E5%95%8A%E8%AF%B4%E5%BE%97%E5%A5%BD\n", string(w.b))
}

func TestSetLevel(t *testing.T) {
	setLogger := &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  4,
	}

	setLogger.SetLevel(LevelTrace)
	assert.DeepEqual(t, LevelTrace, setLogger.level)
	assert.DeepEqual(t, LevelTrace.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelDebug)
	assert.DeepEqual(t, LevelDebug, setLogger.level)
	assert.DeepEqual(t, LevelDebug.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelInfo)
	assert.DeepEqual(t, LevelInfo, setLogger.level)
	assert.DeepEqual(t, LevelInfo.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelNotice)
	assert.DeepEqual(t, LevelNotice, setLogger.level)
	assert.DeepEqual(t, LevelNotice.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelWarn)
	assert.DeepEqual(t, LevelWarn, setLogger.level)
	assert.DeepEqual(t, LevelWarn.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelError)
	assert.DeepEqual(t, LevelError, setLogger.level)
	assert.DeepEqual(t, LevelError.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelFatal)
	assert.DeepEqual(t, LevelFatal, setLogger.level)
	assert.DeepEqual(t, LevelFatal.toString(), setLogger.level.toString())

	setLogger.SetLevel(7)
	assert.DeepEqual(t, 7, int(setLogger.level))
	assert.DeepEqual(t, "[?7] ", setLogger.level.toString())
}
