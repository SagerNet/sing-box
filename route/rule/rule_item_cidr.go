package rule

import (
	"errors"
	"net"
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"

	"go4.org/netipx"
)

var _ RuleItem = (*IPCIDRItem)(nil)

type IPCIDRItem struct {
	ipSet       *netipx.IPSet
	ipifSet     ipInterfaceSet
	isSource    bool
	description string
}

func NewIPCIDRItem(isSource bool, prefixStrings []string) (*IPCIDRItem, error) {
	var builder netipx.IPSetBuilder
	ipifs := make([]ipInterface, 0)
	for i, prefixString := range prefixStrings {
		prefix, err := netip.ParsePrefix(prefixString)
		if err == nil {
			builder.AddPrefix(prefix)
			continue
		}
		ipif, addrErr := parseIPInterface(prefixString)
		if addrErr == nil {
			ipifs = append(ipifs, ipif)
			continue
		}
		if addrErr != errNotIPInterface {
			return nil, E.Cause(addrErr, "parse [", i, "]")
		}
		addr, addrErr := netip.ParseAddr(prefixString)
		if addrErr == nil {
			builder.Add(addr)
			continue
		}
		return nil, E.Cause(err, "parse [", i, "]")
	}
	var description string
	if isSource {
		description = "source_ip_cidr="
	} else {
		description = "ip_cidr="
	}
	if dLen := len(prefixStrings); dLen == 1 {
		description += prefixStrings[0]
	} else if dLen > 3 {
		description += "[" + strings.Join(prefixStrings[:3], " ") + "...]"
	} else {
		description += "[" + strings.Join(prefixStrings, " ") + "]"
	}
	ipSet, err := builder.IPSet()
	if err != nil {
		return nil, err
	}
	return &IPCIDRItem{
		ipSet:       ipSet,
		ipifSet:     ipInterfaceSet(ipifs),
		isSource:    isSource,
		description: description,
	}, nil
}

func NewRawIPCIDRItem(isSource bool, ipSet *netipx.IPSet) *IPCIDRItem {
	var description string
	if isSource {
		description = "source_ip_cidr="
	} else {
		description = "ip_cidr="
	}
	description += "<binary>"
	return &IPCIDRItem{
		ipSet:       ipSet,
		isSource:    isSource,
		description: description,
	}
}

func (r *IPCIDRItem) Match(metadata *adapter.InboundContext) bool {
	if r.isSource || metadata.IPCIDRMatchSource {
		if r.ipSet.Contains(metadata.Source.Addr) {
			return true
		}
		return r.ipifSet.Contains(metadata.Source.Addr)
	}
	if metadata.Destination.IsIP() {
		if r.ipSet.Contains(metadata.Destination.Addr) {
			return true
		}
		return r.ipifSet.Contains(metadata.Destination.Addr)
	}
	if len(metadata.DestinationAddresses) > 0 {
		for _, address := range metadata.DestinationAddresses {
			if r.ipSet.Contains(address) {
				return true
			}
			if r.ipifSet.Contains(address) {
				return true
			}
		}
		return false
	}
	return metadata.IPCIDRAcceptEmpty
}

func (r *IPCIDRItem) String() string {
	return r.description
}

type ipInterfaceSet []ipInterface

func (ipifs ipInterfaceSet) Contains(ip netip.Addr) bool {
	for _, ipif := range ipifs {
		if ipif.EqualInterfaceID(ip) {
			return true
		}
	}
	return false
}

type ipInterface struct {
	id   netip.Addr
	bits int
}

var errNotIPInterface = errors.New("not in ::1/::ffff form")

func parseIPInterface(s string) (ipInterface, error) {
	var ipif ipInterface
	parts := strings.Split(s, "/")
	if len(parts) != 2 || !strings.ContainsRune(parts[0], ':') || !strings.ContainsRune(parts[1], ':') {
		return ipif, errNotIPInterface
	}
	idip, err := netip.ParseAddr(parts[0])
	if err != nil {
		return ipif, err
	}
	maskip, err := netip.ParseAddr(parts[1])
	if err != nil {
		return ipif, err
	}
	ms := maskip.AsSlice()
	for i, b := range ms {
		ms[i] = ^b
	}
	mask := net.IPMask(ms)
	ones, bits := mask.Size()
	if ones == 0 && bits == 0 || ones == idip.BitLen() {
		return ipif, errors.New("invalid mask: " + parts[1])
	}
	ipif.id = maskNetwork(idip, ones)
	ipif.bits = ones
	return ipif, nil
}

func (ipif ipInterface) EqualInterfaceID(ip netip.Addr) bool {
	idip := maskNetwork(ip, ipif.bits)
	return ipif.id == idip
}

func maskNetwork(ip netip.Addr, bits int) netip.Addr {
	n := bits / 8
	m := bits % 8
	s := ip.AsSlice()
	for i := 0; i < n; i++ {
		s[i] = 0
	}
	if m != 0 {
		mask := byte((1 << (8 - m)) - 1)
		s[n] &= mask
	}
	masked, _ := netip.AddrFromSlice(s)
	return masked
}
