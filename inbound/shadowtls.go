package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowtls"
	"github.com/sagernet/sing/common"
	N "github.com/sagernet/sing/common/network"
)

type ShadowTLS struct {
	myInboundAdapter
	service *shadowtls.Service
}

func NewShadowTLS(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowTLSInboundOptions) (*ShadowTLS, error) {
	inbound := &ShadowTLS{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeShadowTLS,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}

	var handshakeForServerName map[string]shadowtls.HandshakeConfig
	if options.Version > 1 {
		handshakeForServerName = make(map[string]shadowtls.HandshakeConfig)
		for serverName, serverOptions := range options.HandshakeForServerName {
			handshakeForServerName[serverName] = shadowtls.HandshakeConfig{
				Server: serverOptions.ServerOptions.Build(),
				Dialer: dialer.New(router, serverOptions.DialerOptions),
			}
		}
	}
	service, err := shadowtls.NewService(shadowtls.ServiceConfig{
		Version:  options.Version,
		Password: options.Password,
		Users: common.Map(options.Users, func(it option.ShadowTLSUser) shadowtls.User {
			return (shadowtls.User)(it)
		}),
		Handshake: shadowtls.HandshakeConfig{
			Server: options.Handshake.ServerOptions.Build(),
			Dialer: dialer.New(router, options.Handshake.DialerOptions),
		},
		HandshakeForServerName: handshakeForServerName,
		Handler:                inbound.upstreamContextHandler(),
		Logger:                 logger,
	})
	if err != nil {
		return nil, err
	}
	inbound.service = service
	inbound.connHandler = inbound
	return inbound, nil
}

func (h *ShadowTLS) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return h.service.NewConnection(adapter.WithContext(log.ContextWithNewID(ctx), &metadata), conn, adapter.UpstreamMetadata(metadata))
}
