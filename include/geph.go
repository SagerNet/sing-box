package include

import (
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/protocol/geph"
)

func registerGephEndpoint(registry *endpoint.Registry) {
	geph.RegisterEndpoint(registry)
}
