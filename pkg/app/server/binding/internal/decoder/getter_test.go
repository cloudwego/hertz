/*
 * Copyright 2023 CloudWeGo Authors
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

package decoder

import (
	"testing"
	"unsafe"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func reqPostArgs(p *protocol.Request) *protocol.Args {
	off := requestFieldOffset(p, "postArgs")
	return (*protocol.Args)(unsafe.Add(unsafe.Pointer(p), off))
}

func TestGetter(t *testing.T) {
	fi := &fieldInfo{}
	req := &protocol.Request{}
	args := reqPostArgs(req)

	{ // path
		s, ok := path(nil, nil, nil, "key")
		assert.Assert(t, ok == false)
		assert.Assert(t, s == "")

		s, ok = path(nil, nil, makeparams("key", "value"), "key")
		assert.Assert(t, ok == true)
		assert.Assert(t, s == "value")
	}

	{ // form
		s, ok := form(fi, req, nil, "key")
		assert.Assert(t, ok == false)
		assert.Assert(t, s == "")

		// post
		args.Reset()
		args.Set("key", "value")
		s, ok = form(fi, req, nil, "key")
		assert.Assert(t, ok == true)
		assert.Assert(t, s == "value")
		args.Reset()

		// query
		req.SetRequestURI("http://example.com?k=v")
		s, ok = form(fi, req, nil, "k")
		assert.Assert(t, ok == true)
		assert.Assert(t, s == "v")
	}

	{ // query
		s, ok := query(fi, req, nil, "key")
		assert.Assert(t, ok == false)
		assert.Assert(t, s == "")

		req.SetRequestURI("http://example.com?k=v")
		s, ok = query(fi, req, nil, "k")
		assert.Assert(t, ok == true)
		assert.Assert(t, s == "v")
	}

	{ // cookie
		s, ok := cookie(fi, req, nil, "key")
		assert.Assert(t, ok == false)
		assert.Assert(t, s == "")

		req.Header.SetCookie("cookiek", "cookiev")
		s, ok = cookie(fi, req, nil, "cookiek")
		assert.Assert(t, ok == true)
		assert.Assert(t, s == "cookiev")
	}

	{ // header
		s, ok := header(fi, req, nil, "key")
		assert.Assert(t, ok == false)
		assert.Assert(t, s == "")

		req.Header.Set("hk", "hv")
		s, ok = header(fi, req, nil, "hk")
		assert.Assert(t, ok == true)
		assert.Assert(t, s == "hv")
	}

	{ // raw_body
		s, ok := rawBody(fi, req, nil, "key")
		assert.Assert(t, ok == false)
		assert.Assert(t, s == "")

		update_rawbody(req, []byte("hello"))
		req.Header.SetContentLength(len("hello"))
		s, ok = rawBody(fi, req, nil, "")
		assert.Assert(t, ok == true)
		assert.Assert(t, s == "hello")
	}
}
