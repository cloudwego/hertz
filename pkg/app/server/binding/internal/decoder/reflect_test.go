package decoder

import (
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"reflect"
	"testing"
)

type foo struct {
	F1 string
}

type fooq struct {
	F1 **string
}

func Test_ReferenceValue(t *testing.T) {
	foo1 := foo{F1: "f1"}
	foo1Val := reflect.ValueOf(foo1)
	foo1PointerVal := ReferenceValue(foo1Val, 5)
	assert.DeepEqual(t, "f1", foo1.F1)
	assert.DeepEqual(t, "f1", foo1Val.Field(0).Interface().(string))
	if foo1PointerVal.Kind() != reflect.Ptr {
		t.Errorf("expect a pointer, but get nil")
	}
	assert.DeepEqual(t, "*****decoder.foo", foo1PointerVal.Type().String())

	deFoo1PointerVal := ReferenceValue(foo1PointerVal, -5)
	if deFoo1PointerVal.Kind() == reflect.Ptr {
		t.Errorf("expect a non-pointer, but get a pointer")
	}
	assert.DeepEqual(t, "f1", deFoo1PointerVal.Field(0).Interface().(string))
}

func Test_GetNonNilReferenceValue(t *testing.T) {
	foo1 := (****foo)(nil)
	foo1Val := reflect.ValueOf(foo1)
	foo1ValNonNil, ptrDepth := GetNonNilReferenceValue(foo1Val)
	if !foo1ValNonNil.IsValid() {
		t.Errorf("expect a valid value, but get nil")
	}
	if !foo1ValNonNil.CanSet() {
		t.Errorf("expect can set value, but not")
	}

	foo1ReferPointer := ReferenceValue(foo1ValNonNil, ptrDepth)
	if foo1ReferPointer.Kind() != reflect.Ptr {
		t.Errorf("expect a pointer, but get nil")
	}
}

func Test_GetFieldValue(t *testing.T) {
	type bar struct {
		B1 **fooq
	}
	bar1 := (***bar)(nil)
	parentIdx := []int{0}
	idx := 0

	bar1Val := reflect.ValueOf(bar1)
	parentFieldVal := GetFieldValue(bar1Val, parentIdx)
	if parentFieldVal.Kind() == reflect.Ptr {
		t.Errorf("expect a non-pointer, but get a pointer")
	}
	if !parentFieldVal.CanSet() {
		t.Errorf("expect can set value, but not")
	}
	fooFieldVal := parentFieldVal.Field(idx)
	assert.DeepEqual(t, "**string", fooFieldVal.Type().String())
	if !fooFieldVal.CanSet() {
		t.Errorf("expect can set value, but not")
	}
}
