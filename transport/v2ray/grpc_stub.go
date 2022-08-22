//go:build !with_grpc

package v2ray

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var errGRPCNotIncluded = E.New("gRPC is not included in this build, rebuild with -tags with_grpc")

func NewGRPCServer(ctx context.Context, serviceName string, tlsConfig *tls.Config, handler N.TCPConnectionHandler) (adapter.V2RayServerTransport, error) {
	return nil, errGRPCNotIncluded
}

func NewGRPCClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, serviceName string, tlsConfig *tls.Config) (adapter.V2RayClientTransport, error) {
	return nil, errGRPCNotIncluded
}
