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

package standard

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"runtime"
	"strconv"
	"syscall"
	"time"

	errs "github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/network"
)

const (
	block1k                  = 1024
	block4k                  = 4096
	block8k                  = 8192
	mallocMax                = block1k * 512
	defaultMallocSize        = block4k
	maxConsecutiveEmptyReads = 100
)

type Conn struct {
	c            net.Conn
	inputBuffer  *linkBuffer
	outputBuffer *linkBuffer
	caches       [][]byte // buf allocated by Next when cross-package, which should be freed when release
	maxSize      int      // history max malloc size

	err error
}

func (c *Conn) ToHertzError(err error) error {
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ENOTCONN) {
		return errs.ErrConnectionClosed
	}
	if netErr, ok := err.(*net.OpError); ok && netErr.Timeout() {
		return errs.ErrTimeout
	}

	return err
}

func (c *Conn) SetWriteTimeout(t time.Duration) error {
	if t <= 0 {
		return c.c.SetWriteDeadline(time.Time{})
	}
	return c.c.SetWriteDeadline(time.Now().Add(t))
}

func (c *Conn) SetReadTimeout(t time.Duration) error {
	if t <= 0 {
		return c.c.SetReadDeadline(time.Time{})
	}
	return c.c.SetReadDeadline(time.Now().Add(t))
}

type TLSConn struct {
	Conn
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *Conn) Read(b []byte) (l int, err error) {
	l = c.Len()
	// If there is some data in inputBuffer, copy it to b and return.
	if l > 0 {
		l = min(l, len(b))
		return l, c.next(l, b)
	}

	// If left buffer size is less than block4k, first Peek(1) to fill the buffer.
	// Then copy min(c.Len, len(b)) to b.
	if len(b) <= block4k {
		// If c.fill(1) return err, conn.Read must return 0, err. So there is no need
		// to check c.Len
		err = c.fill(1)
		if err != nil {
			return 0, err
		}
		l = min(c.Len(), len(b))
		return l, c.next(l, b)
	}

	// Call Read() directly to fill buffer b
	return c.c.Read(b)
}

// Write calls Write syscall directly to send data.
// Will flush buffer immediately, for performance considerations use WriteBinary instead.
func (c *Conn) Write(b []byte) (n int, err error) {
	if err = c.Flush(); err != nil {
		return
	}
	return c.c.Write(b)
}

// ReadFrom implements io.ReaderFrom. If the underlying writer
// supports the ReadFrom method, and c has no buffered data yet,
// this calls the underlying ReadFrom without buffering.
func (c *Conn) ReadFrom(r io.Reader) (n int64, err error) {
	if err = c.Flush(); err != nil {
		return
	}

	if w, ok := c.c.(io.ReaderFrom); ok {
		n, err = w.ReadFrom(r)
		return
	}

	var m int
	bufNode := c.outputBuffer.write

	// if there is no available buffer, create one.
	if !bufNode.recyclable() || cap(bufNode.buf) == 0 {
		c.Malloc(block4k)
		c.outputBuffer.write.Reset()
		c.outputBuffer.len = cap(c.outputBuffer.write.buf)
		bufNode = c.outputBuffer.write
	}

	for {
		if bufNode.Cap() == 0 {
			if err1 := c.Flush(); err1 != nil {
				return n, err1
			}
		}

		nr := 0
		for nr < maxConsecutiveEmptyReads {
			m, err = r.Read(bufNode.buf[bufNode.malloc:cap(bufNode.buf)])
			if m != 0 || err != nil {
				break
			}
			nr++
		}
		if nr == maxConsecutiveEmptyReads {
			return n, io.ErrNoProgress
		}
		bufNode.malloc += m
		n += int64(m)
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		// If we filled the buffer exactly, flush preemptively.
		if bufNode.Cap() == 0 {
			err = c.Flush()
		} else {
			err = nil
			// Update buffer available length for next Malloc
			c.outputBuffer.len = bufNode.Cap()
		}
	}
	return
}

// Close closes the connection
func (c *Conn) Close() error {
	return c.c.Close()
}

// CloseNoResetBuffer closes the connection without reset buffer.
func (c *Conn) CloseNoResetBuffer() error {
	return c.c.Close()
}

// LocalAddr returns the local address of the connection.
func (c *Conn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

// RemoteAddr returns the remote address of the connection.
func (c *Conn) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

// SetDeadline sets the connection deadline.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

// SetReadDeadline sets the read deadline of the connection.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline of the connection.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}

func (c *Conn) releaseCaches() {
	for i := range c.caches {
		free(c.caches[i])
		c.caches[i] = nil
	}
	c.caches = c.caches[:0]
}

// Release release linkBuffer.
//
// NOTE: This function should only be called in inputBuffer.
func (c *Conn) Release() error {
	// c.Len() is used to check whether the data has been fully read. If there
	// is some data in inputBuffer, we mustn't use head and write to check
	// whether current node can be released. We should use head and read as the
	// judge connection.
	if c.Len() == 0 {
		// Reset buffer so that we can reuse it
		// In this case, the request can be held in one single node. We just need
		// Reset this node to hold next request.
		//
		// NOTE: Each connection will bind a buffer. We need to care about the memory usage.
		if c.inputBuffer.head == c.inputBuffer.write {
			c.inputBuffer.write.Reset()
			return nil
		}

		// Critical condition that the buffer is big enough to hold the whole request
		// In this case, head holds the last request and current request has been held
		// in write node. So we just need to release head and reset write.
		if c.inputBuffer.head.next == c.inputBuffer.write {
			// Recalculate the maxSize
			size := c.inputBuffer.head.malloc
			node := c.inputBuffer.head
			node.Release()
			size += c.inputBuffer.write.malloc
			if size > mallocMax {
				size = mallocMax
			}
			if size > c.maxSize {
				c.maxSize = size
			}
			c.handleTail()
			c.inputBuffer.head, c.inputBuffer.read = c.inputBuffer.write, c.inputBuffer.write
			c.releaseCaches()
			return nil
		}
	}

	// If there is some data in the buffer, it means the request hasn't been fully handled.
	// Or the request is too big to hold in a single node.
	// Cross multi node.
	size := 0
	for c.inputBuffer.head != c.inputBuffer.read {
		node := c.inputBuffer.head
		c.inputBuffer.head = c.inputBuffer.head.next
		size += c.inputBuffer.head.malloc
		node.Release()
	}
	// The readOnly field in readOnly is just used to malloc a new node so that next
	// request can be held in one node.
	// It has nothing to do with release logic.
	c.inputBuffer.write.readOnly = true
	if size > mallocMax {
		size = mallocMax
	}
	if size > c.maxSize {
		c.maxSize = size
	}
	c.releaseCaches()
	return nil
}

// handleTail prevents too large tail node to ensure the memory usage.
func (c *Conn) handleTail() {
	if cap(c.inputBuffer.write.buf) > mallocMax {
		node := c.inputBuffer.write
		c.inputBuffer.write.next = newBufferNode(c.maxSize)
		c.inputBuffer.write = c.inputBuffer.write.next
		node.Release()
		return
	}
	c.inputBuffer.write.Reset()
}

// Peek returns the next n bytes without advancing the reader. The bytes stop
// being valid at the next read call. If Peek returns fewer than n bytes, it
// also returns an error explaining why the read is short.
func (c *Conn) Peek(i int) (p []byte, err error) {
	node := c.inputBuffer.read
	// fill the inputBuffer so that there is enough data
	err = c.fill(i)
	if err != nil {
		return
	}

	if c.Len() < i {
		i = c.Len()
		err = c.readErr()
	}

	l := node.Len()
	// Enough data in a single node, so that just return the slice of the node.
	if l >= i {
		return node.buf[node.off : node.off+i], err
	}

	// not enough data in a signal node
	if block1k < i && i <= mallocMax {
		p = malloc(i, i)
		c.caches = append(c.caches, p)
	} else {
		p = make([]byte, i)
	}
	c.peekBuffer(i, p)
	return p, err
}

// peekBuffer loads the buf with data of size i without moving read pointer.
func (c *Conn) peekBuffer(i int, buf []byte) {
	l, pIdx, node := 0, 0, c.inputBuffer.read
	for ack := i; ack > 0; ack = ack - l {
		l = node.Len()
		if l >= ack {
			copy(buf[pIdx:], node.buf[node.off:node.off+ack])
			break
		} else if l > 0 {
			pIdx += copy(buf[pIdx:], node.buf[node.off:node.off+l])
		}
		node = node.next
	}
}

// next loads the buf with data of size i with moving read pointer.
func (c *Conn) next(length int, b []byte) error {
	c.peekBuffer(length, b)
	err := c.Skip(length)
	if err != nil {
		return err
	}
	return c.Release()
}

// fill loads more data than size i, otherwise it will block read.
// NOTE: fill may fill data less than i and store err in Conn.err
// when last read returns n > 0 and err != nil. So after calling
// fill, it is necessary to check whether c.Len() > i
func (c *Conn) fill(i int) (err error) {
	// Check if there is enough data in inputBuffer.
	if c.Len() >= i {
		return nil
	}
	// check whether conn has returned err before.
	if err = c.readErr(); err != nil {
		if c.Len() > 0 {
			c.err = err
			return nil
		}
		return
	}
	node := c.inputBuffer.write
	node.buf = node.buf[:cap(node.buf)]
	left := cap(node.buf) - node.malloc

	// If left capacity is less than the length of expected data
	// or it is a new request, we malloc an enough node to hold
	// the data
	if left < i-c.Len() || node.readOnly {
		// not enough capacity
		malloc := i
		if i < c.maxSize {
			malloc = c.maxSize
		}
		c.inputBuffer.write.next = newBufferNode(malloc)
		c.inputBuffer.write = c.inputBuffer.write.next
		// Set readOnly flag to false so that current node can be recycled.
		// In inputBuffer, whether readOnly value is, the node need to be recycled.
		node.readOnly = false
	}

	i -= c.Len()
	node = c.inputBuffer.write
	node.buf = node.buf[:cap(node.buf)]

	// Circulate reading data so that the node holds enough data
	for i > 0 {
		n, err := c.c.Read(c.inputBuffer.write.buf[node.malloc:])
		if n > 0 {
			node.malloc += n
			c.inputBuffer.len += n
			i -= n
			if err != nil {
				c.err = err
				return nil
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Skip discards the next n bytes.
func (c *Conn) Skip(n int) error {
	// check whether enough or not.
	if c.Len() < n {
		return errs.NewPrivate("link buffer skip[" + strconv.Itoa(n) + "] not enough")
	}
	c.inputBuffer.len -= n // re-cal length

	var l int
	for ack := n; ack > 0; ack = ack - l {
		l = c.inputBuffer.read.Len()
		if l >= ack {
			c.inputBuffer.read.off += ack
			break
		}
		c.inputBuffer.read = c.inputBuffer.read.next
	}
	return nil
}

// ReadByte is used to read one byte with advancing the read pointer.
func (c *Conn) ReadByte() (p byte, err error) {
	b, err := c.Peek(1)
	if err != nil {
		return ' ', err
	}
	err = c.Skip(1)
	if err != nil {
		return ' ', err
	}
	return b[0], nil
}

// ReadBinary is used to read next n byte with copy, and the read pointer will be advanced.
func (c *Conn) ReadBinary(i int) ([]byte, error) {
	out := make([]byte, i)
	b, err := c.Peek(i)
	if err != nil {
		return nil, err
	}
	copy(out, b)
	err = c.Skip(i)
	return out, err
}

// Len returns the total length of the readable data in the reader.
func (c *Conn) Len() int {
	return c.inputBuffer.len
}

// Malloc will provide a n bytes buffer to send data.
func (c *Conn) Malloc(n int) (buf []byte, err error) {
	if n == 0 {
		return
	}

	// If the capacity of the current buffer is larger than we need,
	// there is no need to malloc new node
	if c.outputBuffer.len > n {
		node := c.outputBuffer.write
		malloc := node.malloc
		node.malloc += n
		node.buf = node.buf[:node.malloc]
		c.outputBuffer.len -= n
		return node.buf[malloc:node.malloc], nil
	}

	mallocSize := n
	if n < defaultMallocSize {
		mallocSize = defaultMallocSize
	}
	node := newBufferNode(mallocSize)
	node.malloc = n
	c.outputBuffer.len = cap(node.buf) - n
	c.outputBuffer.write.next = node
	c.outputBuffer.write = c.outputBuffer.write.next
	return node.buf[:n], nil
}

// WriteBinary will use the user buffer to flush.
//
// NOTE: Before flush successfully, the buffer b should be valid.
func (c *Conn) WriteBinary(b []byte) (n int, err error) {
	// If the data size is less than 4k, then just copy to outputBuffer.
	if len(b) < block4k {
		buf, err := c.Malloc(len(b))
		if err != nil {
			return 0, err
		}
		return copy(buf, b), nil
	}
	// Build a new node with buffer b.
	node := newBufferNode(0)
	node.malloc = len(b)
	node.readOnly = true
	node.buf = b
	c.outputBuffer.write.next = node
	c.outputBuffer.write = c.outputBuffer.write.next
	c.outputBuffer.len = 0
	return len(b), nil
}

// Flush will send data to the peer end.
func (c *Conn) Flush() (err error) {
	// No data to flush
	if c.outputBuffer.head == c.outputBuffer.write && c.outputBuffer.head.Len() == 0 {
		return
	}

	// Current node is the tail node of last request, so move to next node.
	if c.outputBuffer.head.Len() == 0 {
		node := c.outputBuffer.head
		c.outputBuffer.head = c.outputBuffer.head.next
		node.Release()
	}

	for {
		n, err := c.c.Write(c.outputBuffer.head.buf[c.outputBuffer.head.off:c.outputBuffer.head.malloc])
		if err != nil {
			return err
		}
		c.outputBuffer.head.off += n
		if c.outputBuffer.head == c.outputBuffer.write {
			// If the capacity of buffer is less than 8k, then just reset the node
			if c.outputBuffer.head.recyclable() {
				c.outputBuffer.head.Reset()
				c.outputBuffer.len = cap(c.outputBuffer.head.buf)
			}
			break
		}
		// Flush next node
		node := c.outputBuffer.head
		c.outputBuffer.head = c.outputBuffer.head.next
		node.Release()
	}
	return nil
}

func (c *Conn) HandleSpecificError(err error, rip string) (needIgnore bool) {
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		hlog.SystemLogger().Debugf("Go net library error=%s, remoteAddr=%s", err.Error(), rip)
		return true
	}
	return false
}

func (c *Conn) readErr() error {
	err := c.err
	c.err = nil
	return err
}

func (c *TLSConn) Handshake() error {
	return c.c.(network.ConnTLSer).Handshake()
}

func (c *TLSConn) ConnectionState() tls.ConnectionState {
	return c.c.(network.ConnTLSer).ConnectionState()
}

func newConn(c net.Conn, size int) network.Conn {
	maxSize := defaultMallocSize
	if size > maxSize {
		maxSize = size
	}

	node := newBufferNode(maxSize)
	inputBuffer := &linkBuffer{
		head:  node,
		read:  node,
		write: node,
	}
	runtime.SetFinalizer(inputBuffer, (*linkBuffer).release)

	outputNode := newBufferNode(0)
	outputBuffer := &linkBuffer{
		head:  outputNode,
		write: outputNode,
	}
	runtime.SetFinalizer(outputBuffer, (*linkBuffer).release)

	return &Conn{
		c:            c,
		inputBuffer:  inputBuffer,
		outputBuffer: outputBuffer,
		maxSize:      maxSize,
	}
}

func newTLSConn(c net.Conn, size int) network.Conn {
	maxSize := defaultMallocSize
	if size > maxSize {
		maxSize = size
	}

	node := newBufferNode(maxSize)
	inputBuffer := &linkBuffer{
		head:  node,
		read:  node,
		write: node,
	}
	runtime.SetFinalizer(inputBuffer, (*linkBuffer).release)

	outputNode := newBufferNode(0)
	outputBuffer := &linkBuffer{
		head:  outputNode,
		write: outputNode,
	}
	runtime.SetFinalizer(outputBuffer, (*linkBuffer).release)

	return &TLSConn{
		Conn{
			c:            c,
			inputBuffer:  inputBuffer,
			outputBuffer: outputBuffer,
			maxSize:      maxSize,
		},
	}
}
