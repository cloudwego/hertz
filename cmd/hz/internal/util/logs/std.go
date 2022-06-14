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

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
)

type StdLogger struct {
	level      int
	outLogger  *log.Logger
	warnLogger *log.Logger
	errLogger  *log.Logger
	out        *bytes.Buffer
	warn       *bytes.Buffer
	err        *bytes.Buffer
	Defer      bool
	ErrOnly    bool
}

func NewStdLogger(level int) *StdLogger {
	out := bytes.NewBuffer(nil)
	warn := bytes.NewBuffer(nil)
	err := bytes.NewBuffer(nil)
	return &StdLogger{
		level:      level,
		outLogger:  log.New(out, "[INFO]", log.Llongfile),
		warnLogger: log.New(warn, "[WARN]", log.Llongfile),
		errLogger:  log.New(err, "[ERROR]", log.Llongfile),
		out:        out,
		err:        err,
	}
}

func (stdLogger *StdLogger) Debugf(format string, v ...interface{}) {
	if stdLogger.level > LevelDebug {
		return
	}
	stdLogger.outLogger.Output(3, fmt.Sprintf(format, v...))
	if !stdLogger.Defer {
		stdLogger.FlushOut()
	}
}

func (stdLogger *StdLogger) Infof(format string, v ...interface{}) {
	if stdLogger.level > LevelInfo {
		return
	}
	stdLogger.outLogger.Output(3, fmt.Sprintf(format, v...))
	if !stdLogger.Defer {
		stdLogger.FlushOut()
	}
}

func (stdLogger *StdLogger) Warnf(format string, v ...interface{}) {
	if stdLogger.level > LevelWarn {
		return
	}
	stdLogger.warnLogger.Output(3, fmt.Sprintf(format, v...))
	if !stdLogger.Defer {
		stdLogger.FlushErr()
	}
}

func (stdLogger *StdLogger) Errorf(format string, v ...interface{}) {
	if stdLogger.level > LevelError {
		return
	}
	stdLogger.errLogger.Output(3, fmt.Sprintf(format, v...))
	if !stdLogger.Defer {
		stdLogger.FlushErr()
	}
}

func (stdLogger *StdLogger) Flush() {
	stdLogger.FlushErr()
	if !stdLogger.ErrOnly {
		stdLogger.FlushOut()
	}
}

func (stdLogger *StdLogger) FlushOut() {
	os.Stderr.Write(stdLogger.out.Bytes())
	stdLogger.out.Reset()
}

func (stdLogger *StdLogger) Err() string {
	return string(stdLogger.err.Bytes())
}

func (stdLogger *StdLogger) ErrLines() []string {
	lines := bytes.Split(stdLogger.err.Bytes(), []byte("[WARN]"))
	if len(lines) <= 1 {
		return nil
	}
	rets := make([]string, len(lines)-1)
	for i := range rets {
		rets[i] = string(lines[i+1])
	}
	return rets
}

func (stdLogger *StdLogger) FlushErr() {
	os.Stderr.Write(stdLogger.err.Bytes())
	stdLogger.err.Reset()
}

func (stdLogger *StdLogger) OutLines() []string {
	lines := bytes.Split(stdLogger.out.Bytes(), []byte("[INFO]"))
	rets := make([]string, len(lines))
	for i, ret := range lines {
		rets[i] = string(ret)
	}
	return rets
}

func (stdLogger *StdLogger) Out() []byte {
	return stdLogger.out.Bytes()
}

func (stdLogger *StdLogger) SetLevel(level int) error {
	switch level {
	case LevelDebug, LevelInfo, LevelWarn, LevelError:
		break
	default:
		return errors.New("invalid log level")
	}
	stdLogger.level = level
	return nil
}
