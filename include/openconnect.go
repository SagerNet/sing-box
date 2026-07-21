//go:build with_openconnect

package include

import (
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/protocol/openconnect"
)

func registerOpenConnectEndpoint(registry *endpoint.Registry) {
	openconnect.RegisterEndpoint(registry)
}

func registerOpenConnectDNSTransport(registry *dns.TransportRegistry) {
	openconnect.RegisterDNSTransport(registry)
}
