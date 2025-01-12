package dialer

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func New(ctx context.Context, options option.DialerOptions, remoteIsDomain bool) (N.Dialer, error) {
	if options.IsWireGuardListener {
		return NewDefault(ctx, options)
	}
	var (
		dialer N.Dialer
		err    error
	)
	if options.Detour != "" {
		outboundManager := service.FromContext[adapter.OutboundManager](ctx)
		if outboundManager == nil {
			return nil, E.New("missing outbound manager")
		}
		dialer = NewDetour(outboundManager, options.Detour)
	} else {
		dialer, err = NewDefault(ctx, options)
		if err != nil {
			return nil, err
		}
	}
	if remoteIsDomain && options.Detour == "" {
		networkManager := service.FromContext[adapter.NetworkManager](ctx)
		dnsTransport := service.FromContext[adapter.DNSTransportManager](ctx)
		var defaultOptions adapter.NetworkOptions
		if networkManager != nil {
			defaultOptions = networkManager.DefaultOptions()
		}
		var (
			dnsQueryOptions      adapter.DNSQueryOptions
			resolveFallbackDelay time.Duration
		)
		if options.DomainResolver != nil && options.DomainResolver.Server != "" {
			transport, loaded := dnsTransport.Transport(options.DomainResolver.Server)
			if !loaded {
				return nil, E.New("domain resolver not found: " + options.DomainResolver.Server)
			}
			var strategy C.DomainStrategy
			if options.DomainResolver.Strategy != option.DomainStrategy(C.DomainStrategyAsIS) {
				strategy = C.DomainStrategy(options.DomainResolver.Strategy)
			} else if
			//nolint:staticcheck
			options.DomainStrategy != option.DomainStrategy(C.DomainStrategyAsIS) {
				//nolint:staticcheck
				strategy = C.DomainStrategy(options.DomainStrategy)
			}
			dnsQueryOptions = adapter.DNSQueryOptions{
				Transport:    transport,
				Strategy:     strategy,
				DisableCache: options.DomainResolver.DisableCache,
				RewriteTTL:   options.DomainResolver.RewriteTTL,
				ClientSubnet: options.DomainResolver.ClientSubnet.Build(netip.Prefix{}),
			}
			resolveFallbackDelay = time.Duration(options.FallbackDelay)
		} else if defaultOptions.DomainResolver != "" {
			dnsQueryOptions = defaultOptions.DomainResolveOptions
			transport, loaded := dnsTransport.Transport(defaultOptions.DomainResolver)
			if !loaded {
				return nil, E.New("default domain resolver not found: " + defaultOptions.DomainResolver)
			}
			dnsQueryOptions.Transport = transport
			resolveFallbackDelay = time.Duration(options.FallbackDelay)
		} else {
			deprecated.Report(ctx, deprecated.OptionMissingDomainResolver)
		}
		dialer = NewResolveDialer(
			ctx,
			dialer,
			options.Detour == "" && !options.TCPFastOpen,
			dnsQueryOptions,
			resolveFallbackDelay,
		)
	}
	return dialer, nil
}

type ParallelInterfaceDialer interface {
	N.Dialer
	DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error)
	ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error)
}

type ParallelNetworkDialer interface {
	DialParallelNetwork(ctx context.Context, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error)
	ListenSerialNetworkPacket(ctx context.Context, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, netip.Addr, error)
}
