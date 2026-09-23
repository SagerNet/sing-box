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

type Options struct {
	Context                 context.Context
	Options                 option.DialerOptions
	RemoteIsDomain          bool
	DirectResolver          bool
	ResolverOnDetour        bool
	NewDialer               bool
	DisableEmptyDirectCheck bool
	DirectOutbound          bool
	DefaultOutbound         bool
}

// TODO: merge with NewWithOptions
func New(ctx context.Context, options option.DialerOptions, remoteIsDomain bool) (N.Dialer, error) {
	return NewWithOptions(Options{
		Context:        ctx,
		Options:        options,
		RemoteIsDomain: remoteIsDomain,
	})
}

func NewWithOptions(options Options) (N.Dialer, error) {
	dialOptions := options.Options
	var (
		dialer N.Dialer
		err    error
	)
	hasDetour := dialOptions.Detour != "" || options.DefaultOutbound
	if dialOptions.Detour != "" {
		outboundManager := service.FromContext[adapter.OutboundManager](options.Context)
		if outboundManager == nil {
			return nil, E.New("missing outbound manager")
		}
		dialer = NewDetour(outboundManager, dialOptions.Detour, options.DisableEmptyDirectCheck)
	} else if options.DefaultOutbound {
		outboundManager := service.FromContext[adapter.OutboundManager](options.Context)
		if outboundManager == nil {
			return nil, E.New("missing outbound manager")
		}
		dialer = NewDefaultOutboundDetour(outboundManager)
	} else {
		dialer, err = NewDefault(options.Context, dialOptions)
		if err != nil {
			return nil, err
		}
	}
	if options.RemoteIsDomain && (!hasDetour || options.ResolverOnDetour || dialOptions.DomainResolver != nil && dialOptions.DomainResolver.Server != "") {
		var (
			server          string
			dnsQueryOptions adapter.DNSQueryOptions
		)
		hasDomainResolver := dialOptions.DomainResolver != nil && dialOptions.DomainResolver.Server != ""
		if options.DirectResolver {
			if !hasDomainResolver {
				return nil, E.New("missing domain resolver for domain server address")
			}
			dnsQueryOptions = domainResolveQueryOptions(dialOptions.DomainResolver)
		} else {
			dnsQueryOptions, err = NewDNSQueryOptions(options.Context, dialOptions.DomainResolver, options.NewDialer)
			if err != nil {
				return nil, err
			}
		}
		if hasDomainResolver {
			server = dialOptions.DomainResolver.Server
		}
		if
		//nolint:staticcheck
		dialOptions.DomainStrategy != option.DomainStrategy(C.DomainStrategyAsIS) && (!hasDomainResolver || dialOptions.DomainResolver.Strategy == option.DomainStrategy(C.DomainStrategyAsIS)) {
			//nolint:staticcheck
			dnsQueryOptions.Strategy = C.DomainStrategy(dialOptions.DomainStrategy)
			deprecated.Report(options.Context, deprecated.OptionLegacyDomainStrategyOptions)
		}
		dialer = NewResolveDialer(
			options.Context,
			dialer,
			dialOptions.Detour == "" && !dialOptions.TCPFastOpen,
			server,
			dnsQueryOptions,
			time.Duration(dialOptions.FallbackDelay),
		)
	}
	return dialer, nil
}

func NewDNSQueryOptions(ctx context.Context, domainResolver *option.DomainResolveOptions, newDialer bool) (adapter.DNSQueryOptions, error) {
	dnsTransport := service.FromContext[adapter.DNSTransportManager](ctx)
	if domainResolver != nil && domainResolver.Server != "" {
		transport, loaded := dnsTransport.Transport(domainResolver.Server)
		if !loaded {
			return adapter.DNSQueryOptions{}, E.New("domain resolver not found: ", domainResolver.Server)
		}
		dnsQueryOptions := domainResolveQueryOptions(domainResolver)
		dnsQueryOptions.Transport = transport
		return dnsQueryOptions, nil
	}
	defaultOptions := service.FromContext[adapter.NetworkManager](ctx).DefaultOptions()
	if defaultOptions.DomainResolver != "" {
		transport, loaded := dnsTransport.Transport(defaultOptions.DomainResolver)
		if !loaded {
			return adapter.DNSQueryOptions{}, E.New("default domain resolver not found: ", defaultOptions.DomainResolver)
		}
		dnsQueryOptions := defaultOptions.DomainResolveOptions
		dnsQueryOptions.Transport = transport
		return dnsQueryOptions, nil
	}
	if len(dnsTransport.Transports()) < 2 {
		return adapter.DNSQueryOptions{Transport: dnsTransport.Default()}, nil
	}
	if newDialer {
		return adapter.DNSQueryOptions{}, E.New("missing domain resolver for domain server address")
	}
	deprecated.Report(ctx, deprecated.OptionMissingDomainResolver)
	return adapter.DNSQueryOptions{}, nil
}

func domainResolveQueryOptions(domainResolver *option.DomainResolveOptions) adapter.DNSQueryOptions {
	return adapter.DNSQueryOptions{
		Strategy:               C.DomainStrategy(domainResolver.Strategy),
		Timeout:                time.Duration(domainResolver.Timeout),
		DisableCache:           domainResolver.DisableCache,
		DisableOptimisticCache: domainResolver.DisableOptimisticCache,
		RewriteTTL:             domainResolver.RewriteTTL,
		ClientSubnet:           domainResolver.ClientSubnet.Build(netip.Prefix{}),
	}
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

type PacketDialerWithDestination interface {
	ListenPacketWithDestination(ctx context.Context, destination M.Socksaddr) (net.PacketConn, netip.Addr, error)
}
