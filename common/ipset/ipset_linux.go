//go:build linux

package ipset

import (
	"net"

	"github.com/sagernet/netlink"
)

// Test whether the ip is in the set or not
func Test(setName string, ip net.IP) (bool, error) {
	return netlink.IpsetTest(setName, &netlink.IPSetEntry{
		IP: ip,
	})
}

// Verify dumps a specific ipset to check if we can use the set normally
func Verify(setName string) error {
	_, err := netlink.IpsetList(setName)
	return err
}
