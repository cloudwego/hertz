package tagexpr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExprSelector(t *testing.T) {
	es := ExprSelector("F1.Index")
	field, ok := es.ParentField()
	assert.True(t, ok)
	assert.Equal(t, "F1", field)
}
