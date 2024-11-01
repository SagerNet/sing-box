//go:build !with_quic

package include

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func init() {
	dns.RegisterTransport([]string{"quic", "h3"}, func(options dns.TransportOptions) (dns.Transport, error) {
		return nil, C.ErrQUICNotIncluded
	})
	v2ray.RegisterQUICConstructor(
		func(ctx context.Context, logger logger.ContextLogger, options option.V2RayQUICOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (adapter.V2RayServerTransport, error) {
			return nil, C.ErrQUICNotIncluded
		},
		func(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayQUICOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
			return nil, C.ErrQUICNotIncluded
		},
	)
}

func registerQUICOutbounds(registry *outbound.Registry) {
	outbound.Register[option.HysteriaOutboundOptions](registry, C.TypeHysteria, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaOutboundOptions) (adapter.Outbound, error) {
		return nil, C.ErrQUICNotIncluded
	})
	outbound.Register[option.TUICOutboundOptions](registry, C.TypeTUIC, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TUICOutboundOptions) (adapter.Outbound, error) {
		return nil, C.ErrQUICNotIncluded
	})
	outbound.Register[option.Hysteria2OutboundOptions](registry, C.TypeHysteria2, func(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.Hysteria2OutboundOptions) (adapter.Outbound, error) {
		return nil, C.ErrQUICNotIncluded
	})
}
