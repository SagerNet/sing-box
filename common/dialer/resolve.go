package dialer

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type ResolveDialer struct {
	dialer        N.Dialer
	router        adapter.Router
	strategy      dns.DomainStrategy
	fallbackDelay time.Duration
}

func NewResolveDialer(router adapter.Router, dialer N.Dialer, strategy dns.DomainStrategy, fallbackDelay time.Duration) *ResolveDialer {
	return &ResolveDialer{
		dialer,
		router,
		strategy,
		fallbackDelay,
	}
}

func (d *ResolveDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if !destination.IsFqdn() {
		return d.dialer.DialContext(ctx, network, destination)
	}
	ctx, metadata := adapter.AppendContext(ctx)
	ctx = log.ContextWithOverrideLevel(ctx, log.LevelDebug)
	metadata.Destination = destination
	metadata.Domain = ""
	var addresses []netip.Addr
	var err error
	if d.strategy == dns.DomainStrategyAsIS {
		addresses, err = d.router.LookupDefault(ctx, destination.Fqdn)
	} else {
		addresses, err = d.router.Lookup(ctx, destination.Fqdn, d.strategy)
	}
	if err != nil {
		return nil, err
	}
	return N.DialParallel(ctx, d.dialer, network, destination, addresses, d.strategy == dns.DomainStrategyPreferIPv6, d.fallbackDelay)
}

func (d *ResolveDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if !destination.IsFqdn() {
		return d.dialer.ListenPacket(ctx, destination)
	}
	ctx, metadata := adapter.AppendContext(ctx)
	ctx = log.ContextWithOverrideLevel(ctx, log.LevelDebug)
	metadata.Destination = destination
	metadata.Domain = ""
	var addresses []netip.Addr
	var err error
	if d.strategy == dns.DomainStrategyAsIS {
		addresses, err = d.router.LookupDefault(ctx, destination.Fqdn)
	} else {
		addresses, err = d.router.Lookup(ctx, destination.Fqdn, d.strategy)
	}
	if err != nil {
		return nil, err
	}
	conn, destinationAddress, err := N.ListenSerial(ctx, d.dialer, destination, addresses)
	if err != nil {
		return nil, err
	}
	return bufio.NewNATPacketConn(bufio.NewPacketConn(conn), destination, M.SocksaddrFrom(destinationAddress, destination.Port)), nil
}

func (d *ResolveDialer) Upstream() any {
	return d.dialer
}
