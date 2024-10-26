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
	"github.com/sagernet/sing-shadowtls"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*ShadowTLS)(nil)

type ShadowTLS struct {
	myOutboundAdapter
	client *shadowtls.Client
}

func NewShadowTLS(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowTLSOutboundOptions) (*ShadowTLS, error) {
	outbound := &ShadowTLS{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeShadowTLS,
			network:      []string{N.NetworkTCP},
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}

	if options.Version == 0 {
		options.Version = 1
	}

	if options.Version == 1 {
		options.TLS.MinVersion = "1.2"
		options.TLS.MaxVersion = "1.2"
	}
	tlsConfig, err := tls.NewClient(ctx, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}

	var tlsHandshakeFunc shadowtls.TLSHandshakeFunc
	switch options.Version {
	case 1, 2:
		tlsHandshakeFunc = func(ctx context.Context, conn net.Conn, _ shadowtls.TLSSessionIDGeneratorFunc) error {
			return common.Error(tls.ClientHandshake(ctx, conn, tlsConfig))
		}
	case 3:
		if idConfig, loaded := tlsConfig.(tls.WithSessionIDGenerator); loaded {
			tlsHandshakeFunc = func(ctx context.Context, conn net.Conn, sessionIDGenerator shadowtls.TLSSessionIDGeneratorFunc) error {
				idConfig.SetSessionIDGenerator(sessionIDGenerator)
				return common.Error(tls.ClientHandshake(ctx, conn, tlsConfig))
			}
		} else {
			stdTLSConfig, err := tlsConfig.Config()
			if err != nil {
				return nil, err
			}
			tlsHandshakeFunc = shadowtls.DefaultTLSHandshakeFunc(options.Password, stdTLSConfig)
		}
	}
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	client, err := shadowtls.NewClient(shadowtls.ClientConfig{
		Version:      options.Version,
		Password:     options.Password,
		Server:       options.ServerOptions.Build(),
		Dialer:       outboundDialer,
		TLSHandshake: tlsHandshakeFunc,
		Logger:       logger,
	})
	if err != nil {
		return nil, err
	}
	outbound.client = client
	return outbound, nil
}

func (h *ShadowTLS) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		return h.client.DialContext(ctx)
	default:
		return nil, os.ErrInvalid
	}
}

func (h *ShadowTLS) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (h *ShadowTLS) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *ShadowTLS) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}
