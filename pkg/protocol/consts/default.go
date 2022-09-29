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

package consts

import "time"

const (
	// *** Server default value ***

	// DefaultMaxInMemoryFileSize defines the in memory file size when parse
	// multipart_form. If the size exceeds, then hertz will write to disk.
	DefaultMaxInMemoryFileSize = 16 * 1024 * 1024

	// *** Client default value start from here ***

	// DefaultDialTimeout is timeout used by Dialer and DialDualStack
	// for establishing TCP connections.
	DefaultDialTimeout = time.Second

	// DefaultMaxConnsPerHost is the maximum number of concurrent connections
	// http client may establish per host by default (i.e. if
	// Client.MaxConnsPerHost isn't set).
	DefaultMaxConnsPerHost = 512

	// DefaultMaxIdleConnDuration is the default duration before idle keep-alive
	// connection is closed.
	DefaultMaxIdleConnDuration = 10 * time.Second

	// DefaultMaxIdempotentCallAttempts is the default idempotent calls attempts count.
	DefaultMaxIdempotentCallAttempts = 1

	// DefaultMaxRetryTimes is the default call times of retry
	DefaultMaxRetryTimes = 1
)
