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
	"io"
	"strings"
	"sync"
)

var silentMode = false

// SetSilentMode is used to mute engine error log,
// for example: error when reading request headers.
// If true, hertz engine will mute it.
func SetSilentMode(s bool) {
	silentMode = s
}

var builderPool = sync.Pool{New: func() interface{} {
	return &strings.Builder{} // nolint:SA6002
}}

type systemLogger struct {
	logger FullLogger
	prefix string
}

func (ll *systemLogger) SetOutput(w io.Writer) {
	ll.logger.SetOutput(w)
}

func (ll *systemLogger) SetLevel(lv Level) {
	ll.logger.SetLevel(lv)
}

func (ll *systemLogger) Fatal(v ...interface{}) {
	v = append([]interface{}{ll.prefix}, v...)
	ll.logger.Fatal(v...)
}

func (ll *systemLogger) Error(v ...interface{}) {
	v = append([]interface{}{ll.prefix}, v...)
	ll.logger.Error(v...)
}

func (ll *systemLogger) Warn(v ...interface{}) {
	v = append([]interface{}{ll.prefix}, v...)
	ll.logger.Warn(v...)
}

func (ll *systemLogger) Notice(v ...interface{}) {
	v = append([]interface{}{ll.prefix}, v...)
	ll.logger.Notice(v...)
}

func (ll *systemLogger) Info(v ...interface{}) {
	v = append([]interface{}{ll.prefix}, v...)
	ll.logger.Info(v...)
}

func (ll *systemLogger) Debug(v ...interface{}) {
	v = append([]interface{}{ll.prefix}, v...)
	ll.logger.Debug(v...)
}

func (ll *systemLogger) Trace(v ...interface{}) {
	v = append([]interface{}{ll.prefix}, v...)
	ll.logger.Trace(v...)
}

func (ll *systemLogger) Fatalf(format string, v ...interface{}) {
	ll.logger.Fatalf(ll.addPrefix(format), v...)
}

func (ll *systemLogger) Errorf(format string, v ...interface{}) {
	if silentMode && format == EngineErrorFormat {
		return
	}
	ll.logger.Errorf(ll.addPrefix(format), v...)
}

func (ll *systemLogger) Warnf(format string, v ...interface{}) {
	ll.logger.Warnf(ll.addPrefix(format), v...)
}

func (ll *systemLogger) Noticef(format string, v ...interface{}) {
	ll.logger.Noticef(ll.addPrefix(format), v...)
}

func (ll *systemLogger) Infof(format string, v ...interface{}) {
	ll.logger.Infof(ll.addPrefix(format), v...)
}

func (ll *systemLogger) Debugf(format string, v ...interface{}) {
	ll.logger.Debugf(ll.addPrefix(format), v...)
}

func (ll *systemLogger) Tracef(format string, v ...interface{}) {
	ll.logger.Tracef(ll.addPrefix(format), v...)
}

func (ll *systemLogger) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	ll.logger.CtxFatalf(ctx, ll.addPrefix(format), v...)
}

func (ll *systemLogger) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	ll.logger.CtxErrorf(ctx, ll.addPrefix(format), v...)
}

func (ll *systemLogger) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	ll.logger.CtxWarnf(ctx, ll.addPrefix(format), v...)
}

func (ll *systemLogger) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	ll.logger.CtxNoticef(ctx, ll.addPrefix(format), v...)
}

func (ll *systemLogger) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	ll.logger.CtxInfof(ctx, ll.addPrefix(format), v...)
}

func (ll *systemLogger) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	ll.logger.CtxDebugf(ctx, ll.addPrefix(format), v...)
}

func (ll *systemLogger) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	ll.logger.CtxTracef(ctx, ll.addPrefix(format), v...)
}

func (ll *systemLogger) addPrefix(format string) string {
	builder := builderPool.Get().(*strings.Builder)
	builder.Grow(len(format) + len(ll.prefix))
	builder.WriteString(ll.prefix)
	builder.WriteString(format)
	s := builder.String()
	builder.Reset()
	builderPool.Put(builder) // nolint:SA6002
	return s
}
