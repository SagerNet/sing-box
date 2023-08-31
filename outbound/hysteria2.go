//go:build with_quic

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
	"github.com/sagernet/sing-box/transport/hysteria2"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound                = (*TUIC)(nil)
	_ adapter.InterfaceUpdateListener = (*TUIC)(nil)
)

type Hysteria2 struct {
	myOutboundAdapter
	client *hysteria2.Client
}

func NewHysteria2(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.Hysteria2OutboundOptions) (*Hysteria2, error) {
	options.UDPFragmentDefault = true
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	tlsConfig, err := tls.NewClient(ctx, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	var salamanderPassword string
	if options.Obfs != nil {
		if options.Obfs.Password == "" {
			return nil, E.New("missing obfs password")
		}
		switch options.Obfs.Type {
		case hysteria2.ObfsTypeSalamander:
			salamanderPassword = options.Obfs.Password
		default:
			return nil, E.New("unknown obfs type: ", options.Obfs.Type)
		}
	}
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	networkList := options.Network.Build()
	client, err := hysteria2.NewClient(hysteria2.ClientOptions{
		Context:            ctx,
		Dialer:             outboundDialer,
		ServerAddress:      options.ServerOptions.Build(),
		SendBPS:            uint64(options.UpMbps * 1024 * 1024),
		ReceiveBPS:         uint64(options.DownMbps * 1024 * 1024),
		SalamanderPassword: salamanderPassword,
		Password:           options.Password,
		TLSConfig:          tlsConfig,
		UDPDisabled:        !common.Contains(networkList, N.NetworkUDP),
	})
	if err != nil {
		return nil, err
	}
	return &Hysteria2{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeHysteria2,
			network:      networkList,
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
		client: client,
	}, nil
}

func (h *Hysteria2) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		return h.client.DialConn(ctx, destination)
	case N.NetworkUDP:
		conn, err := h.ListenPacket(ctx, destination)
		if err != nil {
			return nil, err
		}
		return bufio.NewBindPacketConn(conn, destination), nil
	default:
		return nil, E.New("unsupported network: ", network)
	}
}

func (h *Hysteria2) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	return h.client.ListenPacket(ctx)
}

func (h *Hysteria2) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *Hysteria2) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}

func (h *Hysteria2) InterfaceUpdated() error {
	return h.client.CloseWithError(E.New("network changed"))
}

func (h *Hysteria2) Close() error {
	return h.client.CloseWithError(os.ErrClosed)
}
