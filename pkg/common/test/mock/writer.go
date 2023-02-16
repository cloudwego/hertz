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

package mock

import "bytes"

type ExtWriter struct {
	tmp     []byte
	Buf     *bytes.Buffer
	IsFinal *bool
}

func (m *ExtWriter) Write(p []byte) (n int, err error) {
	m.tmp = p
	return len(p), nil
}

func (m *ExtWriter) Flush() error {
	_, err := m.Buf.Write(m.tmp)
	return err
}

func (m *ExtWriter) Finalize() error {
	if !*m.IsFinal {
		*m.IsFinal = true
	}
	return nil
}

func (m *ExtWriter) SetBody(body []byte) {
	m.Buf.Reset()
	m.tmp = body
}
