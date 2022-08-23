package proxyproto

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/pires/go-proxyproto"
)

var _ N.Dialer = (*Dialer)(nil)

type Dialer struct {
	N.Dialer
}

func (d *Dialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		conn, err := d.Dialer.DialContext(ctx, network, destination)
		if err != nil {
			return nil, err
		}
		var source M.Socksaddr
		metadata := adapter.ContextFrom(ctx)
		if metadata != nil {
			source = metadata.Source
		}
		if !source.IsValid() {
			source = M.SocksaddrFromNet(conn.LocalAddr())
		}
		if destination.Addr.Is6() {
			source = M.SocksaddrFrom(netip.AddrFrom16(source.Addr.As16()), source.Port)
		}
		h := proxyproto.HeaderProxyFromAddrs(1, source.TCPAddr(), destination.TCPAddr())
		_, err = h.WriteTo(conn)
		if err != nil {
			conn.Close()
			return nil, E.Cause(err, "write proxy protocol header")
		}
		return conn, nil
	default:
		return d.Dialer.DialContext(ctx, network, destination)
	}
}
