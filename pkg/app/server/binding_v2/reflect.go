package binding_v2

import (
	"reflect"
	"unsafe"
)

func valueAndTypeID(v interface{}) (reflect.Value, uintptr) {
	header := (*emptyInterface)(unsafe.Pointer(&v))

	rv := reflect.ValueOf(v)
	return rv, header.typeID
}

type emptyInterface struct {
	typeID  uintptr
	dataPtr unsafe.Pointer
}

// ReferenceValue convert T to *T, the ptrDepth is the count of '*'.
func ReferenceValue(v reflect.Value, ptrDepth int) reflect.Value {
	switch {
	case ptrDepth > 0:
		for ; ptrDepth > 0; ptrDepth-- {
			vv := reflect.New(v.Type())
			vv.Elem().Set(v)
			v = vv
		}
	case ptrDepth < 0:
		for ; ptrDepth < 0 && v.Kind() == reflect.Ptr; ptrDepth++ {
			v = v.Elem()
		}
	}
	return v
}

func GetNonNilReferenceValue(v reflect.Value) (reflect.Value, int) {
	var ptrDepth int
	t := v.Type()
	elemKind := t.Kind()
	for elemKind == reflect.Ptr {
		t = t.Elem()
		elemKind = t.Kind()
		ptrDepth++
	}
	val := reflect.New(t).Elem()
	return val, ptrDepth
}

func GetFieldValue(reqValue reflect.Value, parentIndex []int) reflect.Value {
	for _, idx := range parentIndex {
		if reqValue.Kind() == reflect.Ptr && reqValue.IsNil() {
			nonNilVal, ptrDepth := GetNonNilReferenceValue(reqValue)
			reqValue.Set(ReferenceValue(nonNilVal, ptrDepth))
		}
		for reqValue.Kind() == reflect.Ptr {
			reqValue = reqValue.Elem()
		}
		reqValue = reqValue.Field(idx)
	}

	// 父 struct 有可能也是一个指针，所以需要再处理一次才能得到最终的父Value(非nil的reflect.Value)
	for reqValue.Kind() == reflect.Ptr {
		if reqValue.IsNil() {
			nonNilVal, ptrDepth := GetNonNilReferenceValue(reqValue)
			reqValue.Set(ReferenceValue(nonNilVal, ptrDepth))
		}
		reqValue = reqValue.Elem()
	}

	return reqValue
}
