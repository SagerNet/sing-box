package direct

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.DirectOutboundOptions](registry, C.TypeDirect, NewOutbound)
}

var (
	_ N.ParallelDialer             = (*Outbound)(nil)
	_ dialer.ParallelNetworkDialer = (*Outbound)(nil)
)

type Outbound struct {
	outbound.Adapter
	logger               logger.ContextLogger
	dialer               dialer.ParallelInterfaceDialer
	domainStrategy       dns.DomainStrategy
	fallbackDelay        time.Duration
	networkStrategy      C.NetworkStrategy
	networkType          []C.InterfaceType
	fallbackNetworkType  []C.InterfaceType
	networkFallbackDelay time.Duration
	overrideOption       int
	overrideDestination  M.Socksaddr
	// loopBack *loopBackDetector
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.DirectOutboundOptions) (adapter.Outbound, error) {
	options.UDPFragmentDefault = true
	outboundDialer, err := dialer.NewDirect(ctx, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	outbound := &Outbound{
		Adapter:              outbound.NewAdapterWithDialerOptions(C.TypeDirect, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.DialerOptions),
		logger:               logger,
		domainStrategy:       dns.DomainStrategy(options.DomainStrategy),
		fallbackDelay:        time.Duration(options.FallbackDelay),
		networkStrategy:      C.NetworkStrategy(options.NetworkStrategy),
		networkType:          common.Map(options.NetworkType, option.InterfaceType.Build),
		fallbackNetworkType:  common.Map(options.FallbackNetworkType, option.InterfaceType.Build),
		networkFallbackDelay: time.Duration(options.NetworkFallbackDelay),
		dialer:               outboundDialer,
		// loopBack:       newLoopBackDetector(router),
	}
	//nolint:staticcheck
	if options.ProxyProtocol != 0 {
		return nil, E.New("Proxy Protocol is deprecated and removed in sing-box 1.6.0")
	}
	//nolint:staticcheck
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

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
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
	/*conn, err := h.dialer.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	return h.loopBack.NewConn(conn), nil*/
	return h.dialer.DialContext(ctx, network, destination)
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
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
	// conn = h.loopBack.NewPacketConn(bufio.NewPacketConn(conn), destination)
	if originDestination != destination {
		conn = bufio.NewNATPacketConn(bufio.NewPacketConn(conn), destination, originDestination)
	}
	return conn, nil
}

func (h *Outbound) DialParallel(ctx context.Context, network string, destination M.Socksaddr, destinationAddresses []netip.Addr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
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
		//nolint:staticcheck
		domainStrategy = dns.DomainStrategy(metadata.InboundOptions.DomainStrategy)
	}
	switch domainStrategy {
	case dns.DomainStrategyUseIPv4:
		destinationAddresses = common.Filter(destinationAddresses, netip.Addr.Is4)
		if len(destinationAddresses) == 0 {
			return nil, E.New("no IPv4 address available for ", destination)
		}
	case dns.DomainStrategyUseIPv6:
		destinationAddresses = common.Filter(destinationAddresses, netip.Addr.Is6)
		if len(destinationAddresses) == 0 {
			return nil, E.New("no IPv6 address available for ", destination)
		}
	}
	return dialer.DialParallelNetwork(ctx, h.dialer, network, destination, destinationAddresses, domainStrategy == dns.DomainStrategyPreferIPv6, h.networkStrategy, h.networkType, h.fallbackNetworkType, h.fallbackDelay)
}

func (h *Outbound) DialParallelNetwork(ctx context.Context, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, networkStrategy C.NetworkStrategy, networkType []C.InterfaceType, fallbackNetworkType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
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
		//nolint:staticcheck
		domainStrategy = dns.DomainStrategy(metadata.InboundOptions.DomainStrategy)
	}
	switch domainStrategy {
	case dns.DomainStrategyUseIPv4:
		destinationAddresses = common.Filter(destinationAddresses, netip.Addr.Is4)
		if len(destinationAddresses) == 0 {
			return nil, E.New("no IPv4 address available for ", destination)
		}
	case dns.DomainStrategyUseIPv6:
		destinationAddresses = common.Filter(destinationAddresses, netip.Addr.Is6)
		if len(destinationAddresses) == 0 {
			return nil, E.New("no IPv6 address available for ", destination)
		}
	}
	return dialer.DialParallelNetwork(ctx, h.dialer, network, destination, destinationAddresses, domainStrategy == dns.DomainStrategyPreferIPv6, networkStrategy, networkType, fallbackNetworkType, fallbackDelay)
}

func (h *Outbound) ListenSerialNetworkPacket(ctx context.Context, destination M.Socksaddr, destinationAddresses []netip.Addr, networkStrategy C.NetworkStrategy, networkType []C.InterfaceType, fallbackNetworkType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, netip.Addr, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
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
	if h.overrideOption == 0 {
		h.logger.InfoContext(ctx, "outbound packet connection")
	} else {
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	}
	conn, newDestination, err := dialer.ListenSerialNetworkPacket(ctx, h.dialer, destination, destinationAddresses, networkStrategy, networkType, fallbackNetworkType, fallbackDelay)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	return conn, newDestination, nil
}

/*func (h *Outbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	if h.loopBack.CheckConn(metadata.Source.AddrPort(), M.AddrPortFromNet(conn.LocalAddr())) {
		return E.New("reject loopback connection to ", metadata.Destination)
	}
	return NewConnection(ctx, h, conn, metadata)
}

func (h *Outbound) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	if h.loopBack.CheckPacketConn(metadata.Source.AddrPort(), M.AddrPortFromNet(conn.LocalAddr())) {
		return E.New("reject loopback packet connection to ", metadata.Destination)
	}
	return NewPacketConnection(ctx, h, conn, metadata)
}
*/
