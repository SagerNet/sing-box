//go:build go1.20

package dialer

import (
	"context"
	"net"

	"github.com/metacubex/tfo-go"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

// Custom TCP dialer with extra features such as "TCP Fast Open" or "TLS Fragmentation"
type ExtendedTCPDialer struct {
	net.Dialer
	DisableTFO  bool
	TLSFragment TLSFragment
}

func (d *ExtendedTCPDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if (d.DisableTFO && !d.TLSFragment.Enabled) || N.NetworkName(network) != N.NetworkTCP {
		switch N.NetworkName(network) {
		case N.NetworkTCP, N.NetworkUDP:
			return d.Dialer.DialContext(ctx, network, destination.String())
		default:
			return d.Dialer.DialContext(ctx, network, destination.AddrString())
		}
	}
	// Create a fragment dialer
	if d.TLSFragment.Enabled {
		fragmentConn := &fragmentConn{
			dialer:      d.Dialer,
			fragment:    d.TLSFragment,
			network:     network,
			destination: destination,
		}
		conn, err := d.Dialer.DialContext(ctx, network, destination.String())
		if err != nil {
			fragmentConn.err = err
			return nil, err
		}
		fragmentConn.conn = conn
		return fragmentConn, nil
	}
	// Create a TFO dialer
	return &slowOpenConn{
			dialer:      &tfo.Dialer{Dialer: d.Dialer, DisableTFO: d.DisableTFO},
			ctx:         ctx,
			network:     network,
			destination: destination,
			create:      make(chan struct{}),
		},
		nil
}
