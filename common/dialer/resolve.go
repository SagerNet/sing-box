package dialer

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ N.Dialer                = (*resolveDialer)(nil)
	_ ParallelInterfaceDialer = (*resolveParallelNetworkDialer)(nil)
)

type resolveDialer struct {
	dialer        N.Dialer
	parallel      bool
	router        adapter.DNSRouter
	strategy      C.DomainStrategy
	fallbackDelay time.Duration
}

func NewResolveDialer(router adapter.DNSRouter, dialer N.Dialer, parallel bool, strategy C.DomainStrategy, fallbackDelay time.Duration) N.Dialer {
	return &resolveDialer{
		dialer,
		parallel,
		router,
		strategy,
		fallbackDelay,
	}
}

type resolveParallelNetworkDialer struct {
	resolveDialer
	dialer ParallelInterfaceDialer
}

func NewResolveParallelInterfaceDialer(router adapter.DNSRouter, dialer ParallelInterfaceDialer, parallel bool, strategy C.DomainStrategy, fallbackDelay time.Duration) ParallelInterfaceDialer {
	return &resolveParallelNetworkDialer{
		resolveDialer{
			dialer,
			parallel,
			router,
			strategy,
			fallbackDelay,
		},
		dialer,
	}
}

func (d *resolveDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if !destination.IsFqdn() {
		return d.dialer.DialContext(ctx, network, destination)
	}
	ctx = log.ContextWithOverrideLevel(ctx, log.LevelDebug)
	addresses, err := d.router.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{Strategy: d.strategy})
	if err != nil {
		return nil, err
	}
	if d.parallel {
		return N.DialParallel(ctx, d.dialer, network, destination, addresses, d.strategy == C.DomainStrategyPreferIPv6, d.fallbackDelay)
	} else {
		return N.DialSerial(ctx, d.dialer, network, destination, addresses)
	}
}

func (d *resolveDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if !destination.IsFqdn() {
		return d.dialer.ListenPacket(ctx, destination)
	}
	ctx = log.ContextWithOverrideLevel(ctx, log.LevelDebug)
	addresses, err := d.router.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{Strategy: d.strategy})
	if err != nil {
		return nil, err
	}
	conn, destinationAddress, err := N.ListenSerial(ctx, d.dialer, destination, addresses)
	if err != nil {
		return nil, err
	}
	return bufio.NewNATPacketConn(bufio.NewPacketConn(conn), M.SocksaddrFrom(destinationAddress, destination.Port), destination), nil
}

func (d *resolveParallelNetworkDialer) DialParallelInterface(ctx context.Context, network string, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	if !destination.IsFqdn() {
		return d.dialer.DialContext(ctx, network, destination)
	}
	ctx = log.ContextWithOverrideLevel(ctx, log.LevelDebug)
	addresses, err := d.router.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{
		Strategy: d.strategy,
	})
	if err != nil {
		return nil, err
	}
	if fallbackDelay == 0 {
		fallbackDelay = d.fallbackDelay
	}
	if d.parallel {
		return DialParallelNetwork(ctx, d.dialer, network, destination, addresses, d.strategy == C.DomainStrategyPreferIPv6, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	} else {
		return DialSerialNetwork(ctx, d.dialer, network, destination, addresses, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	}
}

func (d *resolveParallelNetworkDialer) ListenSerialInterfacePacket(ctx context.Context, destination M.Socksaddr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	if !destination.IsFqdn() {
		return d.dialer.ListenPacket(ctx, destination)
	}
	ctx = log.ContextWithOverrideLevel(ctx, log.LevelDebug)
	addresses, err := d.router.Lookup(ctx, destination.Fqdn, adapter.DNSQueryOptions{Strategy: d.strategy})
	if err != nil {
		return nil, err
	}
	conn, destinationAddress, err := ListenSerialNetworkPacket(ctx, d.dialer, destination, addresses, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	if err != nil {
		return nil, err
	}
	return bufio.NewNATPacketConn(bufio.NewPacketConn(conn), M.SocksaddrFrom(destinationAddress, destination.Port), destination), nil
}

func (d *resolveDialer) Upstream() any {
	return d.dialer
}
