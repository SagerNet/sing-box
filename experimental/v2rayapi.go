package experimental

import (
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

type V2RayServerConstructor = func(logger log.Logger, options option.V2RayAPIOptions) (adapter.V2RayServer, error)

var v2rayServerConstructor V2RayServerConstructor

func RegisterV2RayServerConstructor(constructor V2RayServerConstructor) {
	v2rayServerConstructor = constructor
}

func NewV2RayServer(logger log.Logger, options option.V2RayAPIOptions) (adapter.V2RayServer, error) {
	if v2rayServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return v2rayServerConstructor(logger, options)
}
