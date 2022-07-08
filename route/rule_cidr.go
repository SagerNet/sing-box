package route

import (
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*IPCIDRItem)(nil)

type IPCIDRItem struct {
	prefixes []netip.Prefix
	isSource bool
}

func NewIPCIDRItem(isSource bool, prefixStrings []string) (*IPCIDRItem, error) {
	prefixes := make([]netip.Prefix, 0, len(prefixStrings))
	for i, prefixString := range prefixStrings {
		prefix, err := netip.ParsePrefix(prefixString)
		if err != nil {
			return nil, E.Cause(err, "parse prefix [", i, "]")
		}
		prefixes = append(prefixes, prefix)
	}
	return &IPCIDRItem{
		prefixes: prefixes,
		isSource: isSource,
	}, nil
}

func (r *IPCIDRItem) Match(metadata *adapter.InboundContext) bool {
	if r.isSource {
		for _, prefix := range r.prefixes {
			if prefix.Contains(metadata.Source.Addr) {
				return true
			}
		}
	} else {
		if metadata.Destination.IsIP() {
			for _, prefix := range r.prefixes {
				if prefix.Contains(metadata.Destination.Addr) {
					return true
				}
			}
		} else {
			for _, address := range metadata.DestinationAddresses {
				for _, prefix := range r.prefixes {
					if prefix.Contains(address) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (r *IPCIDRItem) String() string {
	var description string
	if r.isSource {
		description = "source_ipcidr="
	} else {
		description = "ipcidr="
	}
	pLen := len(r.prefixes)
	if pLen == 1 {
		description += r.prefixes[0].String()
	} else {
		description += "[" + strings.Join(common.Map(r.prefixes, F.ToString0[netip.Prefix]), " ") + "]"
	}
	return description
}
