package dns

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
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
	return dialer.DialSerial(ctx, d.dialer, network, destination, addresses)
}

func (d *DialerWrapper) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if destination.IsIP() {
		return d.dialer.ListenPacket(ctx, destination)
	}
	addresses, err := d.client.Lookup(ctx, d.transport, destination.Fqdn, d.strategy)
	if err != nil {
		return nil, err
	}
	return dialer.ListenSerial(ctx, d.dialer, destination, addresses)
}

func (d *DialerWrapper) Upstream() any {
	return d.dialer
}
