package dns

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func NewLocalDialer(ctx context.Context, options option.LocalDNSServerOptions) (N.Dialer, error) {
	if options.LegacyDefaultDialer {
		return dialer.NewDefaultOutbound(ctx), nil
	} else {
		return dialer.New(ctx, options.DialerOptions)
	}
}

func NewRemoteDialer(ctx context.Context, options option.RemoteDNSServerOptions) (N.Dialer, error) {
	transportDialer, err := NewLocalDialer(ctx, options.LocalDNSServerOptions)
	if err != nil {
		return nil, err
	}
	if options.AddressResolver != "" {
		transport := service.FromContext[adapter.DNSTransportManager](ctx)
		resolverTransport, loaded := transport.Transport(options.AddressResolver)
		if !loaded {
			return nil, E.New("address resolver not found: ", options.AddressResolver)
		}
		transportDialer = NewTransportDialer(transportDialer, service.FromContext[adapter.DNSRouter](ctx), resolverTransport, C.DomainStrategy(options.AddressStrategy), time.Duration(options.AddressFallbackDelay))
	} else if M.IsDomainName(options.Server) {
		return nil, E.New("missing address resolver for server: ", options.Server)
	}
	return transportDialer, nil
}

type TransportDialer struct {
	dialer        N.Dialer
	dnsRouter     adapter.DNSRouter
	transport     adapter.DNSTransport
	strategy      C.DomainStrategy
	fallbackDelay time.Duration
}

func NewTransportDialer(dialer N.Dialer, dnsRouter adapter.DNSRouter, transport adapter.DNSTransport, strategy C.DomainStrategy, fallbackDelay time.Duration) *TransportDialer {
	return &TransportDialer{
		dialer,
		dnsRouter,
		transport,
		strategy,
		fallbackDelay,
	}
}

func (d *TransportDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if destination.IsIP() {
		return d.dialer.DialContext(ctx, network, destination)
	}
	addresses, err := d.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{
		Transport: d.transport,
		Strategy:  d.strategy,
	})
	if err != nil {
		return nil, err
	}
	return N.DialParallel(ctx, d.dialer, network, destination, addresses, d.strategy == C.DomainStrategyPreferIPv6, d.fallbackDelay)
}

func (d *TransportDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if destination.IsIP() {
		return d.dialer.ListenPacket(ctx, destination)
	}
	addresses, err := d.dnsRouter.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{
		Transport: d.transport,
		Strategy:  d.strategy,
	})
	if err != nil {
		return nil, err
	}
	conn, _, err := N.ListenSerial(ctx, d.dialer, destination, addresses)
	return conn, err
}

func (d *TransportDialer) Upstream() any {
	return d.dialer
}
