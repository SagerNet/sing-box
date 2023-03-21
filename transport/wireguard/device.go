package wireguard

import (
	"net/netip"

	"github.com/sagernet/sing-tun"
	N "github.com/sagernet/sing/common/network"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

type Device interface {
	wgTun.Device
	N.Dialer
	Start() error
	Inet4Address() netip.Addr
	Inet6Address() netip.Addr
	// NewEndpoint() (stack.LinkEndpoint, error)
}

type NatDevice interface {
	Device
	CreateDestination(session tun.RouteSession, conn tun.RouteContext) tun.DirectDestination
}
