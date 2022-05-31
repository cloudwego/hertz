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
	"testing"
)

func TestGenerateCreateFixedBody(t *testing.T) {
	bodySize := 10
	resFixedBody := "0123456789"
	b := CreateFixedBody(bodySize)
	if string(b) != resFixedBody {
		t.Fatalf("Unexpected %s. Expecting %s.", b, resFixedBody)
	}

	nilFixedBody := CreateFixedBody(0)
	if nilFixedBody != nil {
		t.Fatalf("Unexpected %s. Expecting a nil", nilFixedBody)
	}
}
