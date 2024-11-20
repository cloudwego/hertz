package tagexpr_test

import (
	"testing"

	"github.com/bytedance/go-tagexpr/v2"
	"github.com/stretchr/testify/assert"
)

func TestIssue12(t *testing.T) {
	var vm = tagexpr.New("te")
	type I int
	type S struct {
		F    []I              `te:"range($, '>'+sprintf('%v:%v', #k, #v+2+len($)))"`
		Fs   [][]I            `te:"range($, range(#v, '>'+sprintf('%v:%v', #k, #v+2+##)))"`
		M    map[string]I     `te:"range($, '>'+sprintf('%s:%v', #k, #v+2+##))"`
		MFs  []map[string][]I `te:"range($, range(#v, range(#v, '>'+sprintf('%v:%v', #k, #v+2+##))))"`
		MFs2 []map[string][]I `te:"range($, range(#v, range(#v, '>'+sprintf('%v:%v', #k, #v+2+##))))"`
	}
	a := []I{2, 3}
	r := vm.MustRun(S{
		F:    a,
		Fs:   [][]I{a},
		M:    map[string]I{"m0": 2, "m1": 3},
		MFs:  []map[string][]I{{"m": a}},
		MFs2: []map[string][]I{},
	})
	assert.Equal(t, []interface{}{">0:6", ">1:7"}, r.Eval("F"))
	assert.Equal(t, []interface{}{[]interface{}{">0:6", ">1:7"}}, r.Eval("Fs"))
	assert.Equal(t, []interface{}{">m0:6", ">m1:7"}, r.Eval("M"))
	assert.Equal(t, []interface{}{[]interface{}{[]interface{}{">0:6", ">1:7"}}}, r.Eval("MFs"))
	assert.Equal(t, []interface{}{}, r.Eval("MFs2"))
	assert.Equal(t, true, r.EvalBool("MFs2"))
}
