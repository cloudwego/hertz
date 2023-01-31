package binding_v2

import (
	"github.com/cloudwego/hertz/pkg/protocol"
)

// PathParams parameter acquisition interface on the URL path
type PathParams interface {
	Get(name string) (string, bool)
}

type Binder interface {
	Name() string
	Bind(*protocol.Request, PathParams, interface{}) error
}

var DefaultBinder Binder = &Bind{}
