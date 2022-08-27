//go:build with_grpc

package v2ray

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2raygrpc"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewGRPCServer(ctx context.Context, options option.V2RayGRPCOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler, errorHandler E.Handler) (adapter.V2RayServerTransport, error) {
	return v2raygrpc.NewServer(ctx, options, tlsConfig, handler), nil
}

func NewGRPCClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayGRPCOptions, tlsConfig *tls.Config) (adapter.V2RayClientTransport, error) {
	return v2raygrpc.NewClient(ctx, dialer, serverAddr, options, tlsConfig), nil
}
