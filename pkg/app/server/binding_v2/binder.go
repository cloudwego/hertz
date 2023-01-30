package binding_v2

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/cloudwego/hertz/internal/bytesconv"
	"github.com/cloudwego/hertz/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// PathParams parameter acquisition interface on the URL path
type PathParams interface {
	Get(name string) (string, bool)
}

type Binder interface {
	Name() string
	Bind(*protocol.Request, PathParams, interface{}) error
}

type Bind struct {
	decoderCache sync.Map
}

func (b *Bind) Name() string {
	return "hertz"
}

func (b *Bind) Bind(req *protocol.Request, params PathParams, v interface{}) error {
	// todo: 先做 body unmarshal, 先尝试做 body 绑定，然后再尝试绑定其他内容
	err := b.PreBindBody(req, v)
	if err != nil {
		return err
	}
	rv, typeID := valueAndTypeID(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("receiver must be a non-nil pointer")
	}
	if rv.Elem().Kind() == reflect.Map {
		return nil
	}
	cached, ok := b.decoderCache.Load(typeID)
	if ok {
		// cached decoder, fast path
		decoder := cached.(Decoder)
		return decoder(req, params, rv.Elem())
	}

	decoder, err := getReqDecoder(rv.Type())
	if err != nil {
		return err
	}

	b.decoderCache.Store(typeID, decoder)
	return decoder(req, params, rv.Elem())
}

var (
	jsonContentTypeBytes = "application/json; charset=utf-8"
	protobufContentType  = "application/x-protobuf"
)

// best effort binding
func (b *Bind) PreBindBody(req *protocol.Request, v interface{}) error {
	if req.Header.ContentLength() <= 0 {
		return nil
	}
	switch bytesconv.B2s(req.Header.ContentType()) {
	case jsonContentTypeBytes:
		// todo: 对齐gin， 添加 "EnableDecoderUseNumber"/"EnableDecoderDisallowUnknownFields" 接口
		return jsonUnmarshalFunc(req.Body(), v)
	case protobufContentType:
		msg, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("%s can not implement 'proto.Message'", v)
		}
		return proto.Unmarshal(req.Body(), msg)
	default:
		return nil
	}
}
