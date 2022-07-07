package dns

import (
	"context"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

type DialerWrapper struct {
	dialer    N.Dialer
	strategy  C.DomainStrategy
	client    adapter.DNSClient
	transport adapter.DNSTransport
}

func NewDialerWrapper(dialer N.Dialer, strategy C.DomainStrategy, client adapter.DNSClient, transport adapter.DNSTransport) N.Dialer {
	return &DialerWrapper{dialer, strategy, client, transport}
}

func (d *DialerWrapper) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if destination.IsIP() {
		return d.dialer.DialContext(ctx, network, destination)
	}
	addresses, err := d.client.Lookup(ctx, d.transport, destination.Fqdn, d.strategy)
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

func (d *DialerWrapper) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if destination.IsIP() {
		return d.dialer.ListenPacket(ctx, destination)
	}
	addresses, err := d.client.Lookup(ctx, d.transport, destination.Fqdn, d.strategy)
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

func (d *DialerWrapper) Upstream() any {
	return d.dialer
}
