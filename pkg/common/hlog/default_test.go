package hlog

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestLoggerSetCallerSkip(t *testing.T) {
	// test std logger
	buffer := bytes.NewBuffer(make([]byte, 0, 1024))
	SetOutput(buffer)
	Info("test std logger")
	assert.True(t, strings.Contains(buffer.String(), "default_test.go"))
	assert.True(t, strings.Contains(buffer.String(), "[Info] test std logger"))
	buffer.Reset()

	// test new logger
	l := New()
	l.SetOutput(buffer)

	l.Info("test new logger")
	assert.True(t, strings.Contains(buffer.String(), "default_test.go"))
	assert.True(t, strings.Contains(buffer.String(), "[Info] test new logger"))
	buffer.Reset()

	l.SetCallerSkip(-1)
	l.Info("test new logger")
	assert.True(t, strings.Contains(buffer.String(), "default.go"))
	assert.True(t, strings.Contains(buffer.String(), "[Info] test new logger"))
	buffer.Reset()

	l.SetCallerSkip(1)
	l.Info("test new logger")
	assert.True(t, strings.Contains(buffer.String(), "default_test.go"))
	assert.True(t, strings.Contains(buffer.String(), "[Info] test new logger"))
	buffer.Reset()
}
