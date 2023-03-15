package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
	"github.com/sagernet/sing/protocol/socks"
)

var _ adapter.Outbound = (*Socks)(nil)

type Socks struct {
	myOutboundAdapter
	client     *socks.Client
	resolve    bool
	uot        bool
	uotVersion int
}

func NewSocks(router adapter.Router, logger log.ContextLogger, tag string, options option.SocksOutboundOptions) (*Socks, error) {
	var version socks.Version
	var err error
	if options.Version != "" {
		version, err = socks.ParseVersion(options.Version)
	} else {
		version = socks.Version5
	}
	if err != nil {
		return nil, err
	}
	outbound := &Socks{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeSocks,
			network:  options.Network.Build(),
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		client:  socks.NewClient(dialer.New(router, options.DialerOptions), options.ServerOptions.Build(), version, options.Username, options.Password),
		resolve: version == socks.Version4,
		uot:     options.UoT,
	}
	switch options.UoTVersion {
	case uot.LegacyVersion:
		outbound.uotVersion = uot.LegacyVersion
	case 0, uot.Version:
		outbound.uotVersion = uot.Version
	default:
		return nil, E.New("unknown udp over tcp protocol version ", options.UoTVersion)
	}
	return outbound, nil
}

func (h *Socks) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		if h.uot {
			h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
			var uotDestination M.Socksaddr
			if h.uotVersion == uot.Version {
				uotDestination.Fqdn = uot.MagicAddress
			} else {
				uotDestination.Fqdn = uot.LegacyMagicAddress
			}
			tcpConn, err := h.client.DialContext(ctx, N.NetworkTCP, uotDestination)
			if err != nil {
				return nil, err
			}
			if h.uotVersion == uot.Version {
				return uot.NewLazyConn(tcpConn, uot.Request{IsConnect: true, Destination: destination}), nil
			} else {
				return uot.NewConn(tcpConn, false, destination), nil
			}
		}
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
	if h.resolve && destination.IsFqdn() {
		addrs, err := h.router.LookupDefault(ctx, destination.Fqdn)
		if err != nil {
			return nil, err
		}
		return N.DialSerial(ctx, h.client, network, destination, addrs)
	}
	return h.client.DialContext(ctx, network, destination)
}

func (h *Socks) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	if h.uot {
		h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
		var uotDestination M.Socksaddr
		if h.uotVersion == uot.Version {
			uotDestination.Fqdn = uot.MagicAddress
		} else {
			uotDestination.Fqdn = uot.LegacyMagicAddress
		}
		tcpConn, err := h.client.DialContext(ctx, N.NetworkTCP, uotDestination)
		if err != nil {
			return nil, err
		}
		if h.uotVersion == uot.Version {
			return uot.NewLazyConn(tcpConn, uot.Request{Destination: destination}), nil
		} else {
			return uot.NewConn(tcpConn, false, destination), nil
		}
	}
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	return h.client.ListenPacket(ctx, destination)
}

func (h *Socks) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *Socks) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}
