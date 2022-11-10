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
	"log"
	"os"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestDefaultAndSysLogger(t *testing.T) {
	defaultLog := DefaultLogger()
	systemLog := SystemLogger()

	assert.DeepEqual(t, logger, defaultLog)
	assert.DeepEqual(t, sysLogger, systemLog)
	assert.NotEqual(t, logger, systemLog)
	assert.NotEqual(t, sysLogger, defaultLog)
}

func TestSetLogger(t *testing.T) {
	setLog := &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  6,
	}
	setSysLog := &systemLogger{
		setLog,
		systemLogPrefix,
	}

	assert.NotEqual(t, logger, setLog)
	assert.NotEqual(t, sysLogger, setSysLog)
	SetLogger(setLog)
	assert.DeepEqual(t, logger, setLog)
	assert.DeepEqual(t, sysLogger, setSysLog)
}
