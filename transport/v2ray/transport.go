package v2ray

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2rayhttp"
	"github.com/sagernet/sing-box/transport/v2raywebsocket"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewServerTransport(ctx context.Context, options option.V2RayTransportOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler, errorHandler E.Handler) (adapter.V2RayServerTransport, error) {
	if options.Type == "" {
		return nil, nil
	}
	switch options.Type {
	case C.V2RayTransportTypeHTTP:
		return v2rayhttp.NewServer(ctx, options.HTTPOptions, tlsConfig, handler, errorHandler), nil
	case C.V2RayTransportTypeWebsocket:
		return v2raywebsocket.NewServer(ctx, options.WebsocketOptions, tlsConfig, handler, errorHandler), nil
	case C.V2RayTransportTypeQUIC:
		if tlsConfig == nil {
			return nil, C.ErrTLSRequired
		}
		return NewQUICServer(ctx, options.QUICOptions, tlsConfig, handler, errorHandler)
	case C.V2RayTransportTypeGRPC:
		return NewGRPCServer(ctx, options.GRPCOptions, tlsConfig, handler, errorHandler)
	default:
		return nil, E.New("unknown transport type: " + options.Type)
	}
}

func NewClientTransport(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayTransportOptions, tlsConfig *tls.Config) (adapter.V2RayClientTransport, error) {
	if options.Type == "" {
		return nil, nil
	}
	switch options.Type {
	case C.V2RayTransportTypeHTTP:
		return v2rayhttp.NewClient(ctx, dialer, serverAddr, options.HTTPOptions, tlsConfig), nil
	case C.V2RayTransportTypeGRPC:
		if tlsConfig == nil {
			return nil, C.ErrTLSRequired
		}
		return NewGRPCClient(ctx, dialer, serverAddr, options.GRPCOptions, tlsConfig)
	case C.V2RayTransportTypeWebsocket:
		return v2raywebsocket.NewClient(ctx, dialer, serverAddr, options.WebsocketOptions, tlsConfig), nil
	case C.V2RayTransportTypeQUIC:
		if tlsConfig == nil {
			return nil, C.ErrTLSRequired
		}
		return NewQUICClient(ctx, dialer, serverAddr, options.QUICOptions, tlsConfig)

	default:
		return nil, E.New("unknown transport type: " + options.Type)
	}
}
