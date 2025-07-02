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

package mock

import (
	"context"
	"io"
	"testing"
	"time"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/netpoll"
)

func TestConn(t *testing.T) {
	t.Run("TestReader", func(t *testing.T) {
		s1 := "abcdef4343"
		conn1 := NewConn(s1)
		assert.Nil(t, conn1.SetWriteTimeout(1))
		err := conn1.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		assert.DeepEqual(t, nil, err)
		err = conn1.SetReadTimeout(time.Millisecond * 100)
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, time.Millisecond*100, conn1.GetReadTimeout())

		// Peek Skip Read
		b, _ := conn1.Peek(1)
		assert.DeepEqual(t, []byte{'a'}, b)
		conn1.Skip(1)
		readByte, _ := conn1.ReadByte()
		assert.DeepEqual(t, byte('b'), readByte)

		p := make([]byte, 100)
		n, err := conn1.Read(p)
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, s1[2:], string(p[:n]))

		_, err = conn1.Peek(1)
		assert.DeepEqual(t, errs.ErrTimeout, err)

		conn2 := NewConn(s1)
		p, _ = conn2.ReadBinary(len(s1))
		assert.DeepEqual(t, s1, string(p))
		assert.DeepEqual(t, 0, conn2.Len())
		// Reader
		assert.DeepEqual(t, conn2.zr, conn2.Reader())
	})

	t.Run("TestReadWriter", func(t *testing.T) {
		s1 := "abcdef4343"
		conn := NewConn(s1)
		p, err := conn.ReadBinary(len(s1))
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, s1, string(p))

		wr := conn.WriterRecorder()
		s2 := "efghljk"
		// WriteBinary
		n, err := conn.WriteBinary([]byte(s2))
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, len(s2), n)
		assert.DeepEqual(t, len(s2), wr.WroteLen())

		// Flush
		p, _ = wr.ReadBinary(len(s2))
		assert.DeepEqual(t, len(p), 0)

		conn.Flush()
		p, _ = wr.ReadBinary(len(s2))
		assert.DeepEqual(t, s2, string(p))

		// Write
		s3 := "foobarbaz"
		n, err = conn.Write([]byte(s3))
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, len(s3), n)
		p, _ = wr.ReadBinary(len(s3))
		assert.DeepEqual(t, s3, string(p))

		// Malloc
		buf, _ := conn.Malloc(10)
		assert.DeepEqual(t, 10, len(buf))
		// Writer
		assert.DeepEqual(t, conn.zw, conn.Writer())

		_, err = DialerFun("")
		assert.DeepEqual(t, nil, err)
	})

	t.Run("TestNotImplement", func(t *testing.T) {
		conn := NewConn("")
		t1 := time.Now().Add(time.Millisecond)
		du1 := time.Second
		assert.DeepEqual(t, nil, conn.Release())
		assert.DeepEqual(t, nil, conn.Close())
		assert.DeepEqual(t, nil, conn.LocalAddr())
		assert.DeepEqual(t, nil, conn.RemoteAddr())
		assert.DeepEqual(t, nil, conn.SetIdleTimeout(du1))
		assert.Panic(t, func() {
			conn.SetDeadline(t1)
		})
		assert.Panic(t, func() {
			conn.SetWriteDeadline(t1)
		})
		assert.Panic(t, func() {
			conn.IsActive()
		})
		assert.Panic(t, func() {
			conn.SetOnRequest(func(ctx context.Context, connection netpoll.Connection) error {
				return nil
			})
		})
		assert.Panic(t, func() {
			conn.AddCloseCallback(func(connection netpoll.Connection) error {
				return nil
			})
		})
	})
}

func TestSlowConn(t *testing.T) {
	t.Run("TestSlowReadConn", func(t *testing.T) {
		s1 := "abcdefg"
		conn := NewSlowReadConn(s1)
		assert.Nil(t, conn.SetWriteTimeout(1))
		assert.Nil(t, conn.SetReadTimeout(1))
		assert.DeepEqual(t, time.Duration(1), conn.readTimeout)

		b, err := conn.Peek(4)
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, s1[:4], string(b))
		conn.Skip(len(s1))
		_, err = conn.Peek(1)
		assert.DeepEqual(t, ErrReadTimeout, err)
		_, err = SlowReadDialer("")
		assert.DeepEqual(t, nil, err)
	})

	t.Run("TestSlowWriteConn", func(t *testing.T) {
		conn, err := SlowWriteDialer("")
		assert.DeepEqual(t, nil, err)
		conn.SetWriteTimeout(time.Millisecond * 100)
		err = conn.Flush()
		assert.DeepEqual(t, ErrWriteTimeout, err)
	})
}

func TestStreamConn(t *testing.T) {
	t.Run("TestStreamConn", func(t *testing.T) {
		conn := NewStreamConn()
		_, err := conn.Peek(10)
		assert.DeepEqual(t, nil, err)
		conn.Skip(conn.Len())
		assert.DeepEqual(t, 0, conn.Len())
		_, err = conn.Peek(10)
		assert.DeepEqual(t, "not enough data", err.Error())
		_, err = conn.Peek(1)
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, cap(conn.Data), conn.Len())
		err = conn.Skip(conn.Len() + 1)
		assert.DeepEqual(t, "not enough data", err.Error())
		err = conn.Release()
		assert.DeepEqual(t, nil, err)
		assert.DeepEqual(t, true, conn.HasReleased)
	})

	t.Run("TestNotImplement", func(t *testing.T) {
		conn := NewStreamConn()
		assert.Panic(t, func() {
			conn.ReadByte()
		})
		assert.Panic(t, func() {
			conn.ReadBinary(10)
		})
	})
}

func TestBrokenConn_Flush(t *testing.T) {
	conn := NewBrokenConn("")
	n, err := conn.Writer().WriteBinary([]byte("Foo"))
	assert.DeepEqual(t, 3, n)
	assert.Nil(t, err)
	assert.DeepEqual(t, errs.ErrConnectionClosed, conn.Flush())
}

func TestBrokenConn_Peek(t *testing.T) {
	conn := NewBrokenConn("Foo")
	buf, err := conn.Peek(3)
	assert.Nil(t, buf)
	assert.DeepEqual(t, io.ErrUnexpectedEOF, err)
}

func TestOneTimeConn_Flush(t *testing.T) {
	conn := NewOneTimeConn("")
	n, err := conn.Writer().WriteBinary([]byte("Foo"))
	assert.DeepEqual(t, 3, n)
	assert.Nil(t, err)
	assert.Nil(t, conn.Flush())
	n, err = conn.Writer().WriteBinary([]byte("Bar"))
	assert.DeepEqual(t, 3, n)
	assert.Nil(t, err)
	assert.DeepEqual(t, errs.ErrConnectionClosed, conn.Flush())
}

func TestOneTimeConn_Skip(t *testing.T) {
	conn := NewOneTimeConn("FooBar")
	buf, err := conn.Peek(3)
	assert.DeepEqual(t, "Foo", string(buf))
	assert.Nil(t, err)
	assert.Nil(t, conn.Skip(3))
	assert.DeepEqual(t, 3, conn.contentLength)

	buf, err = conn.Peek(3)
	assert.DeepEqual(t, "Bar", string(buf))
	assert.Nil(t, err)
	assert.Nil(t, conn.Skip(3))
	assert.DeepEqual(t, 0, conn.contentLength)

	buf, err = conn.Peek(3)
	assert.DeepEqual(t, 0, len(buf))
	assert.DeepEqual(t, io.EOF, err)
	assert.DeepEqual(t, io.EOF, conn.Skip(3))
	assert.DeepEqual(t, 0, conn.contentLength)
}
