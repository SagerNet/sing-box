//go:build !go1.20

package dialer

import (
	"context"
	"net"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func DialSlowContext(dialer *tcpDialer, ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP, N.NetworkUDP:
		return dialer.DialContext(ctx, network, destination.String())
	default:
		return dialer.DialContext(ctx, network, destination.AddrString())
	}
}
