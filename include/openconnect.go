//go:build with_openconnect

package include

import (
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/protocol/openconnect"
)

func registerOpenConnectEndpoint(registry *endpoint.Registry) {
	openconnect.RegisterEndpoint(registry)
}
