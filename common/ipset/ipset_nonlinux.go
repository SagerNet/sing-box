//go:build !linux

package ipset

import (
	"net"
)

// Always return false in non-linux
func Test(setName string, ip net.IP) (bool, error) {
	return false, nil
}

// Always pass in non-linux
func Verify(setName string) error {
	return nil
}
