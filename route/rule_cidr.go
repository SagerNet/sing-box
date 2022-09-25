package route

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
		return nil, E.Cause(err, "parse ip_cidr [", i, "]")
	}
	var description string
	if isSource {
		description = "source_ipcidr="
	} else {
		description = "ipcidr="
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

func (r *IPCIDRItem) Match(metadata *adapter.InboundContext) bool {
	if r.isSource {
		return r.ipSet.Contains(metadata.Source.Addr)
	} else {
		if metadata.Destination.IsIP() {
			return r.ipSet.Contains(metadata.Destination.Addr)
		} else {
			for _, address := range metadata.DestinationAddresses {
				if r.ipSet.Contains(address) {
					return true
				}
			}
		}
	}
	return false
}

func (r *IPCIDRItem) String() string {
	return r.description
}
