package include

import (
	"context"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/service"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/dns/transport/fakeip"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/anytls"
	"github.com/sagernet/sing-box/protocol/block"
	"github.com/sagernet/sing-box/protocol/direct"
	protocolDNS "github.com/sagernet/sing-box/protocol/dns"
	"github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing-box/protocol/http"
	"github.com/sagernet/sing-box/protocol/mixed"
	"github.com/sagernet/sing-box/protocol/naive"
	"github.com/sagernet/sing-box/protocol/redirect"
	"github.com/sagernet/sing-box/protocol/shadowsocks"
	"github.com/sagernet/sing-box/protocol/shadowtls"
	"github.com/sagernet/sing-box/protocol/socks"
	"github.com/sagernet/sing-box/protocol/ssh"
	"github.com/sagernet/sing-box/protocol/tor"
	"github.com/sagernet/sing-box/protocol/trojan"
	"github.com/sagernet/sing-box/protocol/tun"
	"github.com/sagernet/sing-box/protocol/vless"
	"github.com/sagernet/sing-box/protocol/vmess"
	"github.com/sagernet/sing-box/service/resolved"
	"github.com/sagernet/sing-box/service/ssmapi"
	E "github.com/sagernet/sing/common/exceptions"
)

func Context(ctx context.Context) context.Context {
	return box.Context(ctx, InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry())
}

func InboundRegistry() *inbound.Registry {
	registry := inbound.NewRegistry()

	tun.RegisterInbound(registry)
	redirect.RegisterRedirect(registry)
	redirect.RegisterTProxy(registry)
	direct.RegisterInbound(registry)

	socks.RegisterInbound(registry)
	http.RegisterInbound(registry)
	mixed.RegisterInbound(registry)

	shadowsocks.RegisterInbound(registry)
	vmess.RegisterInbound(registry)
	trojan.RegisterInbound(registry)
	naive.RegisterInbound(registry)
	shadowtls.RegisterInbound(registry)
	vless.RegisterInbound(registry)
	anytls.RegisterInbound(registry)

	registerQUICInbounds(registry)
	registerStubForRemovedInbounds(registry)

	return registry
}

func OutboundRegistry() *outbound.Registry {
	registry := outbound.NewRegistry()

	direct.RegisterOutbound(registry)

	block.RegisterOutbound(registry)
	protocolDNS.RegisterOutbound(registry)

	group.RegisterSelector(registry)
	group.RegisterURLTest(registry)

	socks.RegisterOutbound(registry)
	http.RegisterOutbound(registry)
	shadowsocks.RegisterOutbound(registry)
	vmess.RegisterOutbound(registry)
	trojan.RegisterOutbound(registry)
	tor.RegisterOutbound(registry)
	ssh.RegisterOutbound(registry)
	shadowtls.RegisterOutbound(registry)
	vless.RegisterOutbound(registry)
	anytls.RegisterOutbound(registry)

	registerQUICOutbounds(registry)
	registerWireGuardOutbound(registry)
	registerStubForRemovedOutbounds(registry)

	return registry
}

func EndpointRegistry() *endpoint.Registry {
	registry := endpoint.NewRegistry()

	registerWireGuardEndpoint(registry)
	registerTailscaleEndpoint(registry)

	return registry
}

func DNSTransportRegistry() *dns.TransportRegistry {
	registry := dns.NewTransportRegistry()

	transport.RegisterTCP(registry)
	transport.RegisterUDP(registry)
	transport.RegisterTLS(registry)
	transport.RegisterHTTPS(registry)
	hosts.RegisterTransport(registry)
	local.RegisterTransport(registry)
	fakeip.RegisterTransport(registry)
	resolved.RegisterTransport(registry)

	registerQUICTransports(registry)
	registerDHCPTransport(registry)
	registerTailscaleTransport(registry)

	return registry
}

func ServiceRegistry() *service.Registry {
	registry := service.NewRegistry()

	resolved.RegisterService(registry)
	ssmapi.RegisterService(registry)

	registerDERPService(registry)
	registerCCMService(registry)

	return registry
}

func registerStubForRemovedInbounds(registry *inbound.Registry) {
	inbound.Register[option.ShadowsocksInboundOptions](registry, C.TypeShadowsocksR, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksInboundOptions) (adapter.Inbound, error) {
		return nil, E.New("ShadowsocksR is deprecated and removed in sing-box 1.6.0")
	})
}

func registerStubForRemovedOutbounds(registry *outbound.Registry) {
	outbound.Register[option.ShadowsocksROutboundOptions](registry, C.TypeShadowsocksR, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksROutboundOptions) (adapter.Outbound, error) {
		return nil, E.New("ShadowsocksR is deprecated and removed in sing-box 1.6.0")
	})
}
