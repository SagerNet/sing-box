package v2ray

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewServerTransport(ctx context.Context, options option.V2RayInboundTransportOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler) (adapter.V2RayServerTransport, error) {
	if options.Type == "" {
		return nil, nil
	}
	switch options.Type {
	case C.V2RayTransportTypeGRPC:
		return NewGRPCServer(ctx, options.GRPCOptions.ServiceName, tlsConfig, handler)
	default:
		return nil, E.New("unknown transport type: " + options.Type)
	}
}

func NewClientTransport(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayOutboundTransportOptions, tlsConfig *tls.Config) (adapter.V2RayClientTransport, error) {
	if options.Type == "" {
		return nil, nil
	}
	switch options.Type {
	case C.V2RayTransportTypeGRPC:
		return NewGRPCClient(ctx, dialer, serverAddr, options.GRPCOptions.ServiceName, tlsConfig)
	default:
		return nil, E.New("unknown transport type: " + options.Type)
	}
}
