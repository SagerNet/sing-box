//go:build with_grpc

package v2ray

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/transport/v2raygrpc"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewGRPCServer(ctx context.Context, serviceName string, tlsConfig *tls.Config, handler N.TCPConnectionHandler) (adapter.V2RayServerTransport, error) {
	return v2raygrpc.NewServer(ctx, serviceName, tlsConfig, handler), nil
}

func NewGRPCClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, serviceName string, tlsConfig *tls.Config) (adapter.V2RayClientTransport, error) {
	return v2raygrpc.NewClient(ctx, dialer, serverAddr, serviceName, tlsConfig), nil
}
