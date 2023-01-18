package binding_v2

import (
	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// todo: 优化，对于非数组类型的解析，要不要再提供一个不返回 []string 的

type getter func(req *protocol.Request, params PathParams, key string, defaultValue ...string) (ret []string)

// todo string 强转优化
func PathParam(req *protocol.Request, params PathParams, key string, defaultValue ...string) (ret []string) {
	var value string
	if params != nil {
		value, _ = params.Get(key)
	}

	if len(value) == 0 && len(defaultValue) != 0 {
		value = defaultValue[0]
	}
	if len(value) != 0 {
		ret = append(ret, value)
	}

	return
}

// todo 区分postform和multipartform
func Form(req *protocol.Request, params PathParams, key string, defaultValue ...string) (ret []string) {
	req.URI().QueryArgs().VisitAll(func(queryKey, value []byte) {
		if bytesconv.B2s(queryKey) == key {
			ret = append(ret, string(value))
		}
	})
	req.PostArgs().VisitAll(func(formKey, value []byte) {
		if bytesconv.B2s(formKey) == key {
			ret = append(ret, string(value))
		}
	})

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func Query(req *protocol.Request, params PathParams, key string, defaultValue ...string) (ret []string) {
	req.URI().QueryArgs().VisitAll(func(queryKey, value []byte) {
		if bytesconv.B2s(queryKey) == key {
			ret = append(ret, string(value))
		}
	})
	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

// todo: cookie
func Cookie(req *protocol.Request, params PathParams, key string, defaultValue ...string) (ret []string) {
	req.Header.VisitAllCookie(func(cookieKey, value []byte) {
		if bytesconv.B2s(cookieKey) == key {
			ret = append(ret, string(value))
		}
	})

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func Header(req *protocol.Request, params PathParams, key string, defaultValue ...string) (ret []string) {
	req.Header.VisitAll(func(headerKey, value []byte) {
		if bytesconv.B2s(headerKey) == key {
			ret = append(ret, string(value))
		}
	})

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func Json(req *protocol.Request, params PathParams, key string, defaultValue ...string) (ret []string) {
	// do nothing
	return
}

func RawBody(req *protocol.Request, params PathParams, key string, defaultValue ...string) (ret []string) {
	if req.Header.ContentLength() > 0 {
		ret = append(ret, string(req.Body()))
	}
	return
}
