//go:build !darwin

package main

import (
	"errors"
	"net/netip"
)

func defaultRouteIP() (netip.Addr, error) {
	return netip.Addr{}, errors.New("not supported")
}
