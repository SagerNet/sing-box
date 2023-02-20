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

	service, err := shadowtls.NewService(shadowtls.ServiceConfig{
		Version:         options.Version,
		Password:        options.Password,
		HandshakeServer: options.Handshake.ServerOptions.Build(),
		HandshakeDialer: dialer.New(router, options.Handshake.DialerOptions),
		Handler:         inbound.upstreamContextHandler(),
		Logger:          logger,
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
