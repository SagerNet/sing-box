//go:build !with_gvisor

package wireguard

import (
	"net/netip"

	"github.com/sagernet/sing-tun"
)

func NewStackDevice(localAddresses []netip.Prefix, mtu uint32, ipRewrite bool) (Device, error) {
	return nil, tun.ErrGVisorNotIncluded
}
