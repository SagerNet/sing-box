//go:build with_tailscale

package include

import (
	"github.com/sagernet/sing-box/adapter/certificate"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/protocol/tailscale"
	"github.com/sagernet/sing-box/service/derp"
)

func registerTailscaleEndpoint(registry *endpoint.Registry) {
	tailscale.RegisterEndpoint(registry)
}

func registerTailcatInbound(registry *inbound.Registry) {
	tailscale.RegisterTailcatInbound(registry)
}

func registerTailcatOutbound(registry *outbound.Registry) {
	tailscale.RegisterTailcatOutbound(registry)
}

func registerTailscaleTransport(registry *dns.TransportRegistry) {
	tailscale.RegistryTransport(registry)
}

func registerTailscaleCertificateProvider(registry *certificate.Registry) {
	tailscale.RegisterCertificateProvider(registry)
}

func registerDERPService(registry *service.Registry) {
	derp.Register(registry)
}
