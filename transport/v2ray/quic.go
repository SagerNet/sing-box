package v2ray

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	quicServerConstructor ServerConstructor[option.V2RayQUICOptions]
	quicClientConstructor ClientConstructor[option.V2RayQUICOptions]
)

func RegisterQUICConstructor(server ServerConstructor[option.V2RayQUICOptions], client ClientConstructor[option.V2RayQUICOptions]) {
	quicServerConstructor = server
	quicClientConstructor = client
}

func NewQUICServer(ctx context.Context, logger logger.ContextLogger, options option.V2RayQUICOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (adapter.V2RayServerTransport, error) {
	if quicServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return quicServerConstructor(ctx, logger, options, tlsConfig, handler)
}

func NewQUICClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayQUICOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	if quicClientConstructor == nil {
		return nil, os.ErrInvalid
	}
	return quicClientConstructor(ctx, dialer, serverAddr, options, tlsConfig)
}
