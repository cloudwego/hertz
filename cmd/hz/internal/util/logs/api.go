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

package logs

func init() {
	defaultLogger = NewStdLogger(LevelInfo)
}

func SetLogger(logger Logger) {
	defaultLogger = logger
}

const (
	LevelDebug = 1 + iota
	LevelInfo
	LevelWarn
	LevelError
)

// TODO: merge with hertz logger package
type Logger interface {
	Debugf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Flush()
	SetLevel(level int) error
}

var defaultLogger Logger

func Errorf(format string, v ...interface{}) {
	defaultLogger.Errorf(format, v...)
}

func Warnf(format string, v ...interface{}) {
	defaultLogger.Warnf(format, v...)
}

func Infof(format string, v ...interface{}) {
	defaultLogger.Infof(format, v...)
}

func Debugf(format string, v ...interface{}) {
	defaultLogger.Debugf(format, v...)
}

func Error(format string, v ...interface{}) {
	defaultLogger.Errorf(format, v...)
}

func Warn(format string, v ...interface{}) {
	defaultLogger.Warnf(format, v...)
}

func Info(format string, v ...interface{}) {
	defaultLogger.Infof(format, v...)
}

func Debug(format string, v ...interface{}) {
	defaultLogger.Debugf(format, v...)
}

func Flush() {
	defaultLogger.Flush()
}

func SetLevel(level int) {
	defaultLogger.SetLevel(level)
}
