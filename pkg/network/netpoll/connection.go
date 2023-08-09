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

package netpoll

import (
	"errors"
	"io"
	"strings"
	"syscall"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/netpoll"
)

type Conn struct {
	network.Conn
}

func (c *Conn) ToHertzError(err error) error {
	if errors.Is(err, netpoll.ErrConnClosed) || errors.Is(err, syscall.EPIPE) {
		return errs.ErrConnectionClosed
	}

	// only unify read timeout for now
	if errors.Is(err, netpoll.ErrReadTimeout) {
		return errs.ErrTimeout
	}
	return err
}

func (c *Conn) Peek(n int) (b []byte, err error) {
	b, err = c.Conn.Peek(n)
	err = normalizeErr(err)
	return
}

func (c *Conn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	err = normalizeErr(err)
	return n, err
}

func (c *Conn) Skip(n int) error {
	return c.Conn.Skip(n)
}

func (c *Conn) Release() error {
	return c.Conn.Release()
}

func (c *Conn) Len() int {
	return c.Conn.Len()
}

func (c *Conn) ReadByte() (b byte, err error) {
	b, err = c.Conn.ReadByte()
	err = normalizeErr(err)
	return
}

func (c *Conn) ReadBinary(n int) (b []byte, err error) {
	b, err = c.Conn.ReadBinary(n)
	err = normalizeErr(err)
	return
}

func (c *Conn) Malloc(n int) (buf []byte, err error) {
	return c.Conn.Malloc(n)
}

func (c *Conn) WriteBinary(b []byte) (n int, err error) {
	return c.Conn.WriteBinary(b)
}

func (c *Conn) Flush() error {
	return c.Conn.Flush()
}

func (c *Conn) HandleSpecificError(err error, rip string) (needIgnore bool) {
	if errors.Is(err, netpoll.ErrConnClosed) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		// ignore flushing error when connection is closed or reset
		if strings.Contains(err.Error(), "when flush") {
			return true
		}
		hlog.SystemLogger().Debugf("Netpoll error=%s, remoteAddr=%s", err.Error(), rip)
		return true
	}
	return false
}

func normalizeErr(err error) error {
	if errors.Is(err, netpoll.ErrEOF) {
		return io.EOF
	}

	return err
}

func newConn(c netpoll.Connection) network.Conn {
	return &Conn{Conn: c.(network.Conn)}
}
