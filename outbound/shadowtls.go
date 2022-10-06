package outbound

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/shadowtls"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*ShadowTLS)(nil)

type ShadowTLS struct {
	myOutboundAdapter
	dialer     N.Dialer
	serverAddr M.Socksaddr
	tlsConfig  tls.Config
	v2         bool
	password   string
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
		dialer:     dialer.New(router, options.DialerOptions),
		serverAddr: options.ServerOptions.Build(),
		password:   options.Password,
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	switch options.Version {
	case 0:
		fallthrough
	case 1:
		options.TLS.MinVersion = "1.2"
		options.TLS.MaxVersion = "1.2"
	case 2:
		outbound.v2 = true
	default:
		return nil, E.New("unknown shadowtls protocol version: ", options.Version)
	}
	var err error
	outbound.tlsConfig, err = tls.NewClient(router, options.Server, common.PtrValueOrDefault(options.TLS))
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
	if !s.v2 {
		_, err = tls.ClientHandshake(ctx, conn, s.tlsConfig)
		if err != nil {
			return nil, err
		}
		return conn, nil
	} else {
		hashConn := shadowtls.NewHashReadConn(conn, s.password)
		_, err = tls.ClientHandshake(ctx, hashConn, s.tlsConfig)
		if err != nil {
			return nil, err
		}
		return shadowtls.NewClientConn(hashConn), nil
	}
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
