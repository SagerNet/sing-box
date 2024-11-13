package dialer

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func New(ctx context.Context, options option.DialerOptions) (N.Dialer, error) {
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	if options.IsWireGuardListener {
		return NewDefault(networkManager, options)
	}
	var (
		dialer N.Dialer
		err    error
	)
	if options.Detour == "" {
		dialer, err = NewDefault(networkManager, options)
		if err != nil {
			return nil, err
		}
	} else {
		outboundManager := service.FromContext[adapter.OutboundManager](ctx)
		if outboundManager == nil {
			return nil, E.New("missing outbound manager")
		}
		dialer = NewDetour(outboundManager, options.Detour)
	}
	if networkManager == nil {
		return NewDefault(networkManager, options)
	}
	if options.Detour == "" {
		router := service.FromContext[adapter.Router](ctx)
		if router != nil {
			dialer = NewResolveDialer(
				router,
				dialer,
				options.Detour == "" && !options.TCPFastOpen,
				dns.DomainStrategy(options.DomainStrategy),
				time.Duration(options.FallbackDelay))
		}
	}
	return dialer, nil
}

func NewDirect(ctx context.Context, options option.DialerOptions) (ParallelInterfaceDialer, error) {
	if options.Detour != "" {
		return nil, E.New("`detour` is not supported in direct context")
	}
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	if options.IsWireGuardListener {
		return NewDefault(networkManager, options)
	}
	dialer, err := NewDefault(networkManager, options)
	if err != nil {
		return nil, err
	}
	return NewResolveParallelInterfaceDialer(
		service.FromContext[adapter.Router](ctx),
		dialer,
		true,
		dns.DomainStrategy(options.DomainStrategy),
		time.Duration(options.FallbackDelay),
	), nil
}

type ParallelInterfaceDialer interface {
	N.Dialer
	DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error)
	ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error)
}

type ParallelNetworkDialer interface {
	DialParallelNetwork(ctx context.Context, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error)
	ListenSerialNetworkPacket(ctx context.Context, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, netip.Addr, error)
}
