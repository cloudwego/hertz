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
	"io"
	"log"
	"os"
)

var (
	// Provide default logger for users to use
	logger FullLogger = &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  4,
	}

	// Provide system logger for print system log
	sysLogger FullLogger = &systemLogger{
		&defaultLogger{
			stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
			depth:  4,
		},
		systemLogPrefix,
	}
)

// SetOutput sets the output of default logger and system logger. By default, it is stderr.
func SetOutput(w io.Writer) {
	logger.SetOutput(w)
	sysLogger.SetOutput(w)
}

// SetLevel sets the level of logs below which logs will not be output.
// The default logger and system logger level is LevelTrace.
// Note that this method is not concurrent-safe.
func SetLevel(lv Level) {
	logger.SetLevel(lv)
	sysLogger.SetLevel(lv)
}

// DefaultLogger return the default logger for hertz.
func DefaultLogger() FullLogger {
	return logger
}

// SystemLogger return the system logger for hertz to print system log.
// This function is not recommended for users to use.
func SystemLogger() FullLogger {
	return sysLogger
}

// SetSystemLogger sets the system logger.
// Note that this method is not concurrent-safe and must not be called
// This function is not recommended for users to use.
func SetSystemLogger(v FullLogger) {
	sysLogger = &systemLogger{v, systemLogPrefix}
}

// SetLogger sets the default logger and the system logger.
// Note that this method is not concurrent-safe and must not be called
// after the use of DefaultLogger and global functions in this package.
func SetLogger(v FullLogger) {
	logger = v
	SetSystemLogger(v)
}
