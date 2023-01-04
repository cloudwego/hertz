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
	"runtime"
	"sync"
)

// NewFunc returns stackless wrapper for the function f.
//
// Unlike f, the returned stackless wrapper doesn't use stack space
// on the goroutine that calls it.
// The wrapper may save a lot of stack space if the following conditions
// are met:
//
//   - f doesn't contain blocking calls on network, I/O or channels;
//   - f uses a lot of stack space;
//   - the wrapper is called from high number of concurrent goroutines.
//
// The stackless wrapper returns false if the call cannot be processed
// at the moment due to high load.
func NewFunc(f func(ctx interface{})) func(ctx interface{}) bool {
	if f == nil {
		panic("BUG: f cannot be nil")
	}

	funcWorkCh := make(chan *funcWork, runtime.GOMAXPROCS(-1)*2048)
	onceInit := func() {
		n := runtime.GOMAXPROCS(-1)
		for i := 0; i < n; i++ {
			go funcWorker(funcWorkCh, f)
		}
	}
	var once sync.Once

	return func(ctx interface{}) bool {
		once.Do(onceInit)
		fw := getFuncWork()
		fw.ctx = ctx

		select {
		case funcWorkCh <- fw:
		default:
			putFuncWork(fw)
			return false
		}
		<-fw.done
		putFuncWork(fw)
		return true
	}
}

func funcWorker(funcWorkCh <-chan *funcWork, f func(ctx interface{})) {
	for fw := range funcWorkCh {
		f(fw.ctx)
		fw.done <- struct{}{}
	}
}

func getFuncWork() *funcWork {
	v := funcWorkPool.Get()
	if v == nil {
		v = &funcWork{
			done: make(chan struct{}, 1),
		}
	}
	return v.(*funcWork)
}

func putFuncWork(fw *funcWork) {
	fw.ctx = nil
	funcWorkPool.Put(fw)
}

var funcWorkPool sync.Pool

type funcWork struct {
	ctx  interface{}
	done chan struct{}
}
