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
	"bytes"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestExtWriter(t *testing.T) {
	b1 := []byte("abcdef4343")
	buf := new(bytes.Buffer)
	isFinal := false
	w := &ExtWriter{
		Buf:     buf,
		IsFinal: &isFinal,
	}

	// write
	n, err := w.Write(b1)
	assert.DeepEqual(t, nil, err)
	assert.DeepEqual(t, len(b1), n)

	// flush
	err = w.Flush()
	assert.DeepEqual(t, nil, err)
	assert.DeepEqual(t, b1, w.Buf.Bytes())

	// setbody
	b2 := []byte("abc")
	w.SetBody(b2)
	err = w.Flush()
	assert.DeepEqual(t, nil, err)
	assert.DeepEqual(t, b2, w.Buf.Bytes())

	w.Finalize()
	assert.DeepEqual(t, true, *(w.IsFinal))
}
