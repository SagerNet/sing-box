package route

import (
	"net"
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/dns"
)

func lookupAddressesByHostname(
	hostname string,
	ipToHostname map[netip.Addr]string,
	macToHostname map[string]string,
	ipToMACTables ...map[netip.Addr]net.HardwareAddr,
) []netip.Addr {
	hostname = dns.FqdnToDomain(hostname)
	if hostname == "" {
		return nil
	}
	resultSet := make(map[netip.Addr]struct{})
	var result []netip.Addr
	addAddress := func(address netip.Addr) {
		if isScopedIPv6Address(address) {
			return
		}
		if _, exists := resultSet[address]; exists {
			return
		}
		resultSet[address] = struct{}{}
		result = append(result, address)
	}
	for address, entryHostname := range ipToHostname {
		if strings.EqualFold(entryHostname, hostname) {
			addAddress(address)
		}
	}
	for mac, entryHostname := range macToHostname {
		if !strings.EqualFold(entryHostname, hostname) {
			continue
		}
		for _, table := range ipToMACTables {
			for address, entryMAC := range table {
				if entryMAC.String() == mac {
					addAddress(address)
				}
			}
		}
	}
	return result
}

func isScopedIPv6Address(address netip.Addr) bool {
	// DNS AAAA records cannot carry an interface zone.
	return address.Is6() && (address.IsLinkLocalUnicast() || address.Zone() != "")
}
