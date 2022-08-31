package outbound

import (
	"context"
	"crypto/tls"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*ShadowTLS)(nil)

type ShadowTLS struct {
	myOutboundAdapter
	dialer     N.Dialer
	serverAddr M.Socksaddr
	tlsConfig  *tls.Config
}

func NewShadowTLS(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowTLSOutboundOptions) (*ShadowTLS, error) {
	outbound := &ShadowTLS{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeShadowTLS,
			network:  []string{N.NetworkTCP},
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		dialer:     dialer.NewOutbound(router, options.OutboundDialerOptions),
		serverAddr: options.ServerOptions.Build(),
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	options.TLS.MinVersion = "1.2"
	options.TLS.MaxVersion = "1.2"
	var err error
	outbound.tlsConfig, err = dialer.TLSConfig(options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	return outbound, nil
}

func (s *ShadowTLS) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
	default:
		return nil, os.ErrInvalid
	}
	conn, err := s.dialer.DialContext(ctx, N.NetworkTCP, s.serverAddr)
	if err != nil {
		return nil, err
	}
	tlsConn, err := dialer.TLSClient(ctx, conn, s.tlsConfig)
	if err != nil {
		return nil, err
	}
	err = tlsConn.HandshakeContext(ctx)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (s *ShadowTLS) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (s *ShadowTLS) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, s, conn, metadata)
}

func (s *ShadowTLS) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}
