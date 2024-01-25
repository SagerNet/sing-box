package outbound

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound = (*Direct)(nil)
	_ N.ParallelDialer = (*Direct)(nil)
)

type Direct struct {
	myOutboundAdapter
	dialer              N.Dialer
	domainStrategy      dns.DomainStrategy
	fallbackDelay       time.Duration
	overrideOption      int
	overrideDestination M.Socksaddr
	loopBack            *loopBackDetector
}

func NewDirect(router adapter.Router, logger log.ContextLogger, tag string, options option.DirectOutboundOptions) (*Direct, error) {
	options.UDPFragmentDefault = true
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	outbound := &Direct{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeDirect,
			network:      []string{N.NetworkTCP, N.NetworkUDP},
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
		domainStrategy: dns.DomainStrategy(options.DomainStrategy),
		fallbackDelay:  time.Duration(options.FallbackDelay),
		dialer:         outboundDialer,
		loopBack:       newLoopBackDetector(),
	}
	if options.ProxyProtocol != 0 {
		return nil, E.New("Proxy Protocol is deprecated and removed in sing-box 1.6.0")
	}
	if options.OverrideAddress != "" && options.OverridePort != 0 {
		outbound.overrideOption = 1
		outbound.overrideDestination = M.ParseSocksaddrHostPort(options.OverrideAddress, options.OverridePort)
	} else if options.OverrideAddress != "" {
		outbound.overrideOption = 2
		outbound.overrideDestination = M.ParseSocksaddrHostPort(options.OverrideAddress, options.OverridePort)
	} else if options.OverridePort != 0 {
		outbound.overrideOption = 3
		outbound.overrideDestination = M.Socksaddr{Port: options.OverridePort}
	}
	return outbound, nil
}

func (h *Direct) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	switch h.overrideOption {
	case 1:
		destination = h.overrideDestination
	case 2:
		newDestination := h.overrideDestination
		newDestination.Port = destination.Port
		destination = newDestination
	case 3:
		destination.Port = h.overrideDestination.Port
	}
	network = N.NetworkName(network)
	switch network {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	conn, err := h.dialer.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	return h.loopBack.NewConn(conn), nil
}

func (h *Direct) DialParallel(ctx context.Context, network string, destination M.Socksaddr, destinationAddresses []netip.Addr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	switch h.overrideOption {
	case 1, 2:
		// override address
		return h.DialContext(ctx, network, destination)
	case 3:
		destination.Port = h.overrideDestination.Port
	}
	network = N.NetworkName(network)
	switch network {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
	case N.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	var domainStrategy dns.DomainStrategy
	if h.domainStrategy != dns.DomainStrategyAsIS {
		domainStrategy = h.domainStrategy
	} else {
		domainStrategy = dns.DomainStrategy(metadata.InboundOptions.DomainStrategy)
	}
	return N.DialParallel(ctx, h.dialer, network, destination, destinationAddresses, domainStrategy == dns.DomainStrategyPreferIPv6, h.fallbackDelay)
}

func (h *Direct) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	originDestination := destination
	switch h.overrideOption {
	case 1:
		destination = h.overrideDestination
	case 2:
		newDestination := h.overrideDestination
		newDestination.Port = destination.Port
		destination = newDestination
	case 3:
		destination.Port = h.overrideDestination.Port
	}
	if h.overrideOption == 0 {
		h.logger.InfoContext(ctx, "outbound packet connection")
	} else {
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	conn, err := h.dialer.ListenPacket(ctx, destination)
	if err != nil {
		return nil, err
	}
	conn = h.loopBack.NewPacketConn(bufio.NewPacketConn(conn))
	if originDestination != destination {
		conn = bufio.NewNATPacketConn(bufio.NewPacketConn(conn), destination, originDestination)
	}
	return conn, nil
}

func (h *Direct) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if h.loopBack.CheckConn(metadata.Source.AddrPort()) {
		return E.New("reject loopback connection to ", metadata.Destination)
	}
	return NewConnection(ctx, h, conn, metadata)
}

func (h *Direct) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	if h.loopBack.CheckPacketConn(metadata.Source.AddrPort()) {
		return E.New("reject loopback packet connection to ", metadata.Destination)
	}
	return NewPacketConnection(ctx, h, conn, metadata)
}
