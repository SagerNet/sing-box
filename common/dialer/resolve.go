package dialer

import (
	"context"
	"net"
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

type ResolveDialer struct {
	dialer   N.Dialer
	router   adapter.Router
	strategy C.DomainStrategy
}

func NewResolveDialer(router adapter.Router, dialer N.Dialer, strategy C.DomainStrategy) *ResolveDialer {
	return &ResolveDialer{
		dialer,
		router,
		strategy,
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
	var conn net.Conn
	var connErrors []error
	for _, address := range addresses {
		conn, err = d.dialer.DialContext(ctx, network, M.SocksaddrFromAddrPort(address, destination.Port))
		if err != nil {
			connErrors = append(connErrors, err)
		}
		return conn, nil
	}
	return nil, E.Errors(connErrors...)
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
	var conn net.PacketConn
	var connErrors []error
	for _, address := range addresses {
		conn, err = d.dialer.ListenPacket(ctx, M.SocksaddrFromAddrPort(address, destination.Port))
		if err != nil {
			connErrors = append(connErrors, err)
		}
		return conn, nil
	}
	return nil, E.Errors(connErrors...)
}

func (d *ResolveDialer) Upstream() any {
	return d.dialer
}
