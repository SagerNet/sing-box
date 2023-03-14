package v2ray

import (
	"context"
	"os"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/common/tls"
	"github.com/jobberrt/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
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

func NewQUICServer(ctx context.Context, options option.V2RayQUICOptions, tlsConfig tls.ServerConfig, handler N.TCPConnectionHandler, errorHandler E.Handler) (adapter.V2RayServerTransport, error) {
	if quicServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return quicServerConstructor(ctx, options, tlsConfig, handler, errorHandler)
}

func NewQUICClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayQUICOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	if quicClientConstructor == nil {
		return nil, os.ErrInvalid
	}
	return quicClientConstructor(ctx, dialer, serverAddr, options, tlsConfig)
}
