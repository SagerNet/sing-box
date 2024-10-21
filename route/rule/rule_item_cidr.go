package rule

import (
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"

	"go4.org/netipx"
)

var _ RuleItem = (*IPCIDRItem)(nil)

type IPCIDRItem struct {
	ipSet       *netipx.IPSet
	isSource    bool
	description string
}

func NewIPCIDRItem(isSource bool, prefixStrings []string) (*IPCIDRItem, error) {
	var builder netipx.IPSetBuilder
	for i, prefixString := range prefixStrings {
		prefix, err := netip.ParsePrefix(prefixString)
		if err == nil {
			builder.AddPrefix(prefix)
			continue
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
		return r.ipSet.Contains(metadata.Source.Addr)
	}
	if metadata.Destination.IsIP() {
		return r.ipSet.Contains(metadata.Destination.Addr)
	}
	if len(metadata.DestinationAddresses) > 0 {
		for _, address := range metadata.DestinationAddresses {
			if r.ipSet.Contains(address) {
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
