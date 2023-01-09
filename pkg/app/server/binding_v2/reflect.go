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
