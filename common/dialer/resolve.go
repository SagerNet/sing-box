package dialer

import (
	"context"
	"net"
	"net/netip"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

type ResolveDialer struct {
	dialer        N.Dialer
	router        adapter.Router
	strategy      C.DomainStrategy
	fallbackDelay time.Duration
}

func NewResolveDialer(router adapter.Router, dialer N.Dialer, strategy C.DomainStrategy, fallbackDelay time.Duration) *ResolveDialer {
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
	var addresses []netip.Addr
	var err error
	if d.strategy == C.DomainStrategyAsIS {
		addresses, err = d.router.LookupDefault(ctx, destination.Fqdn)
	} else {
		addresses, err = d.router.Lookup(ctx, destination.Fqdn, d.strategy)
	}
	if err != nil {
		return nil, err
	}
	return DialParallel(ctx, d.dialer, network, destination, addresses, d.strategy, d.fallbackDelay)
}

func (d *ResolveDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if !destination.IsFqdn() {
		return d.dialer.ListenPacket(ctx, destination)
	}
	var addresses []netip.Addr
	var err error
	if d.strategy == C.DomainStrategyAsIS {
		addresses, err = d.router.LookupDefault(ctx, destination.Fqdn)
	} else {
		addresses, err = d.router.Lookup(ctx, destination.Fqdn, d.strategy)
	}
	if err != nil {
		return nil, err
	}
	conn, err := ListenSerial(ctx, d.dialer, destination, addresses)
	if err != nil {
		return nil, err
	}
	return NewResolvePacketConn(d.router, d.strategy, conn), nil
}

func (d *ResolveDialer) Upstream() any {
	return d.dialer
}
