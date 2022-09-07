//go:build no_gvisor

package wireguard

import "github.com/sagernet/sing-tun"

func NewStackDevice(localAddresses []netip.Prefix, mtu uint32) (Device, error) {
	return nil, tun.ErrGVisorNotIncluded
}
