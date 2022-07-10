//go:build !linux

package tun

import (
	"net/netip"
	"os"
)

func Open(name string) (uintptr, error) {
	return 0, os.ErrInvalid
}

func Configure(name string, inet4Address netip.Prefix, inet6Address netip.Prefix, mtu uint32, autoRoute bool) error {
	return os.ErrInvalid
}

func UnConfigure(name string, inet4Address netip.Prefix, inet6Address netip.Prefix, autoRoute bool) error {
	return os.ErrInvalid
}
