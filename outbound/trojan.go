package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/mux"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/trojan"
)

var _ adapter.Outbound = (*Trojan)(nil)

type Trojan struct {
	myOutboundAdapter
	dialer          N.Dialer
	serverAddr      M.Socksaddr
	key             [56]byte
	multiplexDialer N.Dialer
}

func NewTrojan(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TrojanOutboundOptions) (*Trojan, error) {
	outbound := &Trojan{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeTrojan,
			network:  options.Network.Build(),
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		serverAddr: options.ServerOptions.Build(),
		key:        trojan.Key(options.Password),
	}
	var err error
	outbound.dialer, err = dialer.NewTLS(dialer.NewOutbound(router, options.OutboundDialerOptions), options.Server, common.PtrValueOrDefault(options.TLSOptions))
	if err != nil {
		return nil, err
	}
	outbound.multiplexDialer, err = mux.NewClientWithOptions(ctx, (*TrojanDialer)(outbound), common.PtrValueOrDefault(options.MultiplexOptions))
	if err != nil {
		return nil, err
	}
	return outbound, nil
}

func (h *Trojan) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if h.multiplexDialer == nil {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			h.logger.InfoContext(ctx, "outbound connection to ", destination)
		case N.NetworkUDP:
			h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		}
		return (*TrojanDialer)(h).DialContext(ctx, network, destination)
	} else {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			h.logger.InfoContext(ctx, "outbound multiplex connection to ", destination)
		case N.NetworkUDP:
			h.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		}
		return h.multiplexDialer.DialContext(ctx, network, destination)
	}
}

func (h *Trojan) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if h.multiplexDialer == nil {
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		return (*TrojanDialer)(h).ListenPacket(ctx, destination)
	} else {
		h.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		return h.multiplexDialer.ListenPacket(ctx, destination)
	}
}

func (h *Trojan) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewEarlyConnection(ctx, h, conn, metadata)
}

func (h *Trojan) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}

type TrojanDialer Trojan

func (h *TrojanDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		outConn, err := h.dialer.DialContext(ctx, N.NetworkTCP, h.serverAddr)
		if err != nil {
			return nil, err
		}
		return trojan.NewClientConn(outConn, h.key, destination), nil
	case N.NetworkUDP:
		outConn, err := h.dialer.DialContext(ctx, N.NetworkTCP, h.serverAddr)
		if err != nil {
			return nil, err
		}
		return trojan.NewClientPacketConn(outConn, h.key), nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (h *TrojanDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	conn, err := h.DialContext(ctx, N.NetworkUDP, destination)
	if err != nil {
		return nil, err
	}
	return conn.(*trojan.ClientPacketConn), nil
}
