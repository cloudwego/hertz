package tagexpr

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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
			assert.Equal(t, c.expect, r, c.tag)
		} else {
			assert.Failf(t, string(c.tag), "kvs:%v, err:%v", r, e)
		}
		if e != nil {
			t.Logf("tag:%q, errMsg:%v", c.tag, e)
		}
	}
}
