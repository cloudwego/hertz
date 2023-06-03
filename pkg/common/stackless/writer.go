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
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik Dubbelboer, FastHTTP Authors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2022 CloudWeGo Authors.
 */

package stackless

import (
	"fmt"
	"io"

	"github.com/cloudwego/hertz/pkg/common/bytebufferpool"
	"github.com/cloudwego/hertz/pkg/common/errors"
)

// Writer is an interface stackless writer must conform to.
//
// The interface contains common subset for Writers from compress/* packages.
type Writer interface {
	Write(p []byte) (int, error)
	Flush() error
	Close() error
	Reset(w io.Writer)
}

// NewWriterFunc must return new writer that will be wrapped into
// stackless writer.
type NewWriterFunc func(w io.Writer) Writer

// NewWriter creates a stackless writer around a writer returned
// from newWriter.
//
// The returned writer writes data to dstW.
//
// Writers that use a lot of stack space may be wrapped into stackless writer,
// thus saving stack space for high number of concurrently running goroutines.
func NewWriter(dstW io.Writer, newWriter NewWriterFunc) Writer {
	w := &writer{
		dstW: dstW,
	}
	w.zw = newWriter(&w.xw)
	return w
}

type writer struct {
	dstW io.Writer
	zw   Writer
	xw   xWriter

	err error
	n   int

	p  []byte
	op op
}

type op int

const (
	opWrite op = iota
	opFlush
	opClose
	opReset
)

func (w *writer) Write(p []byte) (int, error) {
	w.p = p
	err := w.do(opWrite)
	w.p = nil
	return w.n, err
}

func (w *writer) Flush() error {
	return w.do(opFlush)
}

func (w *writer) Close() error {
	return w.do(opClose)
}

func (w *writer) Reset(dstW io.Writer) {
	w.xw.Reset()
	w.do(opReset) //nolint:errcheck
	w.dstW = dstW
}

func (w *writer) do(op op) error {
	w.op = op
	if !stacklessWriterFunc(w) {
		return errHighLoad
	}
	err := w.err
	if err != nil {
		return err
	}
	if w.xw.bb != nil && len(w.xw.bb.B) > 0 {
		_, err = w.dstW.Write(w.xw.bb.B)
	}
	w.xw.Reset()

	return err
}

var errHighLoad = errors.NewPublic("cannot compress data due to high load")

var stacklessWriterFunc = NewFunc(writerFunc)

func writerFunc(ctx interface{}) {
	w := ctx.(*writer)
	switch w.op {
	case opWrite:
		w.n, w.err = w.zw.Write(w.p)
	case opFlush:
		w.err = w.zw.Flush()
	case opClose:
		w.err = w.zw.Close()
	case opReset:
		w.zw.Reset(&w.xw)
		w.err = nil
	default:
		panic(fmt.Sprintf("BUG: unexpected op: %d", w.op))
	}
}

type xWriter struct {
	bb *bytebufferpool.ByteBuffer
}

func (w *xWriter) Write(p []byte) (int, error) {
	if w.bb == nil {
		w.bb = bufferPool.Get()
	}
	return w.bb.Write(p)
}

func (w *xWriter) Reset() {
	if w.bb != nil {
		bufferPool.Put(w.bb)
		w.bb = nil
	}
}

var bufferPool bytebufferpool.Pool
