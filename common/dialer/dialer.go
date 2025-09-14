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
	Context          context.Context
	Options          option.DialerOptions
	RemoteIsDomain   bool
	DirectResolver   bool
	ResolverOnDetour bool
	NewDialer        bool
	LegacyDNSDialer  bool
	DirectOutbound   bool
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
	if dialOptions.Detour != "" {
		outboundManager := service.FromContext[adapter.OutboundManager](options.Context)
		if outboundManager == nil {
			return nil, E.New("missing outbound manager")
		}
		dialer = NewDetour(outboundManager, dialOptions.Detour, options.LegacyDNSDialer)
	} else {
		dialer, err = NewDefault(options.Context, dialOptions)
		if err != nil {
			return nil, err
		}
	}
	if options.RemoteIsDomain && (dialOptions.Detour == "" || options.ResolverOnDetour || dialOptions.DomainResolver != nil && dialOptions.DomainResolver.Server != "") {
		networkManager := service.FromContext[adapter.NetworkManager](options.Context)
		dnsTransport := service.FromContext[adapter.DNSTransportManager](options.Context)
		var defaultOptions adapter.NetworkOptions
		if networkManager != nil {
			defaultOptions = networkManager.DefaultOptions()
		}
		var (
			server               string
			dnsQueryOptions      adapter.DNSQueryOptions
			resolveFallbackDelay time.Duration
		)
		if dialOptions.DomainResolver != nil && dialOptions.DomainResolver.Server != "" {
			var transport adapter.DNSTransport
			if !options.DirectResolver {
				var loaded bool
				transport, loaded = dnsTransport.Transport(dialOptions.DomainResolver.Server)
				if !loaded {
					return nil, E.New("domain resolver not found: " + dialOptions.DomainResolver.Server)
				}
			}
			var strategy C.DomainStrategy
			if dialOptions.DomainResolver.Strategy != option.DomainStrategy(C.DomainStrategyAsIS) {
				strategy = C.DomainStrategy(dialOptions.DomainResolver.Strategy)
			} else if
			//nolint:staticcheck
			dialOptions.DomainStrategy != option.DomainStrategy(C.DomainStrategyAsIS) {
				//nolint:staticcheck
				strategy = C.DomainStrategy(dialOptions.DomainStrategy)
				deprecated.Report(options.Context, deprecated.OptionLegacyDomainStrategyOptions)
			}
			server = dialOptions.DomainResolver.Server
			dnsQueryOptions = adapter.DNSQueryOptions{
				Transport:    transport,
				Strategy:     strategy,
				DisableCache: dialOptions.DomainResolver.DisableCache,
				RewriteTTL:   dialOptions.DomainResolver.RewriteTTL,
				ClientSubnet: dialOptions.DomainResolver.ClientSubnet.Build(netip.Prefix{}),
			}
			resolveFallbackDelay = time.Duration(dialOptions.FallbackDelay)
		} else if options.DirectResolver {
			return nil, E.New("missing domain resolver for domain server address")
		} else {
			if defaultOptions.DomainResolver != "" {
				dnsQueryOptions = defaultOptions.DomainResolveOptions
				transport, loaded := dnsTransport.Transport(defaultOptions.DomainResolver)
				if !loaded {
					return nil, E.New("default domain resolver not found: " + defaultOptions.DomainResolver)
				}
				dnsQueryOptions.Transport = transport
				resolveFallbackDelay = time.Duration(dialOptions.FallbackDelay)
			} else {
				transports := dnsTransport.Transports()
				if len(transports) < 2 {
					dnsQueryOptions.Transport = dnsTransport.Default()
				} else if options.NewDialer {
					return nil, E.New("missing domain resolver for domain server address")
				} else {
					deprecated.Report(options.Context, deprecated.OptionMissingDomainResolver)
				}
			}
			if
			//nolint:staticcheck
			dialOptions.DomainStrategy != option.DomainStrategy(C.DomainStrategyAsIS) {
				//nolint:staticcheck
				dnsQueryOptions.Strategy = C.DomainStrategy(dialOptions.DomainStrategy)
				deprecated.Report(options.Context, deprecated.OptionLegacyDomainStrategyOptions)
			}
		}
		dialer = NewResolveDialer(
			options.Context,
			dialer,
			dialOptions.Detour == "" && !dialOptions.TCPFastOpen,
			server,
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
