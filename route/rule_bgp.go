package route

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
)

type BgpItem struct {
	router     adapter.Router
	logger     log.ContextLogger
	communitys []string
	asns       []string
	asnMap     map[string]bool
	comMap     map[string]bool
}

func NewBgpItem(router adapter.Router, logger log.ContextLogger, asns []string, communitys []string) *BgpItem {
	asnsMap := make(map[string]bool)
	commap := make(map[string]bool)
	if len(asns) > 0 {
		for _, asn := range asns {
			asnsMap[asn] = true
		}
	}
	if len(communitys) > 0 {
		for _, c := range communitys {
			commap[c] = true
		}
	}
	return &BgpItem{
		router:     router,
		logger:     logger,
		communitys: communitys,
		asns:       asns,
		asnMap:     asnsMap,
		comMap:     commap,
	}
}

func (r *BgpItem) Match(metadata *adapter.InboundContext) bool {
	bgpapi := r.router.BgpAPI()

	if len(metadata.BgpASN) > 0 {
		return r.asnMap[metadata.BgpASN]
	}
	if len(metadata.BgPCommunity) > 0 {
		return r.comMap[metadata.BgPCommunity]
	}
	if metadata.Destination.IsIP() {
		bgpattrs, err := bgpapi.Lookup(metadata.Destination.Addr, metadata.Destination.IsIPv4(), metadata.Destination.IsIPv6())
		if err != nil {
			return false
		}
		if len(bgpattrs.Attrs) <= 0 {
			return false
		}
		if len(bgpattrs.Attrs[0].ASN) > 0 {
			ASN := bgpattrs.Attrs[0].ASN
			metadata.BgpASN = ASN[len(ASN)-1]
		}
		if len(r.asns) > 0 {
			return r.asnMap[metadata.BgpASN]
		}
		if len(r.communitys) > 0 {
			for _, attr := range bgpattrs.Attrs {
				for _, co := range attr.Community {
					if r.comMap[co] {
						metadata.BgPCommunity = co
						return true
					}
				}
			}
			return false
		}
	}
	return false
}

func (r *BgpItem) String() string {
	var description string
	if len(r.asns) > 0 {
		description = "BGP ASN"
	}
	if len(r.communitys) > 0 {
		description = "BGP Communitys"
	}
	return description
}
