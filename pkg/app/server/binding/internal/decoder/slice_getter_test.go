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

	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func TestGetterSlice(t *testing.T) {
	req := &protocol.Request{}
	args := reqPostArgs(req)

	{ // path
		ss := pathSlice(nil, nil, "key")
		assert.DeepEqual(t, []string(nil), ss)

		ss = pathSlice(nil, makeparams("key", "value"), "key")
		assert.DeepEqual(t, []string{"value"}, ss)
	}

	{ // formSlice
		ss := formSlice(req, nil, "key")
		assert.DeepEqual(t, []string(nil), ss)

		// post
		args.Reset()
		args.Set("key", "value")
		ss = formSlice(req, nil, "key")
		assert.DeepEqual(t, []string{"value"}, ss)
		args.Reset()

		// querySlice
		req.SetRequestURI("http://example.com?k=v")
		ss = formSlice(req, nil, "k")
		assert.DeepEqual(t, []string{"v"}, ss)
	}

	{ // querySlice
		ss := querySlice(req, nil, "key")
		assert.DeepEqual(t, []string(nil), ss)

		req.SetRequestURI("http://example.com?k=v")
		ss = querySlice(req, nil, "k")
		assert.DeepEqual(t, []string{"v"}, ss)
	}

	{ // cookieSlice
		ss := cookieSlice(req, nil, "key")
		assert.DeepEqual(t, []string(nil), ss)

		req.Header.SetCookie("cookiek", "cookiev")
		ss = cookieSlice(req, nil, "cookiek")
		assert.DeepEqual(t, []string{"cookiev"}, ss)
	}

	{ // headerSlice
		ss := headerSlice(req, nil, "key")
		assert.DeepEqual(t, []string(nil), ss)

		req.Header.Set("Hk", "hv")
		ss = headerSlice(req, nil, "Hk")
		assert.DeepEqual(t, []string{"hv"}, ss)
	}

}
