// Copyright 2019 Bytedance Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tagexpr

import (
	"reflect"
	"testing"
)

func TestTagparser(t *testing.T) {
	cases := []struct {
		tag    reflect.StructTag
		expect map[string]string
		fail   bool
	}{
		{
			tag: `tagexpr:"$>0"`,
			expect: map[string]string{
				"@": "$>0",
			},
		}, {
			tag:  `tagexpr:"$>0;'xxx'"`,
			fail: true,
		}, {
			tag: `tagexpr:"$>0;b:sprintf('%[1]T; %[1]v',(X)$)"`,
			expect: map[string]string{
				"@": `$>0`,
				"b": `sprintf('%[1]T; %[1]v',(X)$)`,
			},
		}, {
			tag: `tagexpr:"a:$=='0;1;';b:sprintf('%[1]T; %[1]v',(X)$)"`,
			expect: map[string]string{
				"a": `$=='0;1;'`,
				"b": `sprintf('%[1]T; %[1]v',(X)$)`,
			},
		}, {
			tag: `tagexpr:"a:1;;b:2"`,
			expect: map[string]string{
				"a": `1`,
				"b": `2`,
			},
		}, {
			tag: `tagexpr:";a:1;;b:2;;;"`,
			expect: map[string]string{
				"a": `1`,
				"b": `2`,
			},
		}, {
			tag: `tagexpr:";a:'123\\'';;b:'1\\'23';c:'1\\'2\\'3';;"`,
			expect: map[string]string{
				"a": `'123\''`,
				"b": `'1\'23'`,
				"c": `'1\'2\'3'`,
			},
		}, {
			tag: `tagexpr:"email($)"`,
			expect: map[string]string{
				"@": `email($)`,
			},
		}, {
			tag: `tagexpr:"false"`,
			expect: map[string]string{
				"@": `false`,
			},
		},
	}

	for _, c := range cases {
		r, e := parseTag(c.tag.Get("tagexpr"))
		if e != nil == c.fail {
			if !reflect.DeepEqual(c.expect, r) {
				t.Fatal(c.expect, r, c.tag)
			}
		} else {
			t.Fatalf("tag:%s kvs:%v, err:%v", c.tag, r, e)
		}
		if e != nil {
			t.Logf("tag:%q, errMsg:%v", c.tag, e)
		}
	}
}
