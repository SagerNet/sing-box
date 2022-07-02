package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

func NewRule(router adapter.Router, logger log.Logger, options option.Rule) (adapter.Rule, error) {
	switch options.Type {
	case "", C.RuleTypeDefault:
		return NewDefaultRule(router, logger, common.PtrValueOrDefault(options.DefaultOptions))
	case C.RuleTypeLogical:
		return NewLogicalRule(router, logger, common.PtrValueOrDefault(options.LogicalOptions))
	default:
		return nil, E.New("unknown rule type: ", options.Type)
	}
}

var _ adapter.Rule = (*DefaultRule)(nil)

type DefaultRule struct {
	index    int
	outbound string
	items    []RuleItem
}

type RuleItem interface {
	Match(metadata *adapter.InboundContext) bool
	String() string
}

func NewDefaultRule(router adapter.Router, logger log.Logger, options option.DefaultRule) (*DefaultRule, error) {
	rule := &DefaultRule{
		outbound: options.Outbound,
	}
	if len(options.Inbound) > 0 {
		rule.items = append(rule.items, NewInboundRule(options.Inbound))
	}
	if options.IPVersion > 0 {
		switch options.IPVersion {
		case 4, 6:
			rule.items = append(rule.items, NewIPVersionItem(options.IPVersion == 6))
		default:
			return nil, E.New("invalid ip version: ", options.IPVersion)
		}
	}
	if options.Network != "" {
		switch options.Network {
		case C.NetworkTCP, C.NetworkUDP:
			rule.items = append(rule.items, NewNetworkItem(options.Network))
		default:
			return nil, E.New("invalid network: ", options.Network)
		}
	}
	if len(options.Protocol) > 0 {
		rule.items = append(rule.items, NewProtocolItem(options.Protocol))
	}
	if len(options.Domain) > 0 || len(options.DomainSuffix) > 0 {
		rule.items = append(rule.items, NewDomainItem(options.Domain, options.DomainSuffix))
	}
	if len(options.DomainKeyword) > 0 {
		rule.items = append(rule.items, NewDomainKeywordItem(options.DomainKeyword))
	}
	if len(options.SourceGeoIP) > 0 {
		rule.items = append(rule.items, NewGeoIPItem(router, logger, true, options.SourceGeoIP))
	}
	if len(options.GeoIP) > 0 {
		rule.items = append(rule.items, NewGeoIPItem(router, logger, false, options.GeoIP))
	}
	if len(options.SourceIPCIDR) > 0 {
		item, err := NewIPCIDRItem(true, options.SourceIPCIDR)
		if err != nil {
			return nil, err
		}
		rule.items = append(rule.items, item)
	}
	if len(options.IPCIDR) > 0 {
		item, err := NewIPCIDRItem(false, options.IPCIDR)
		if err != nil {
			return nil, err
		}
		rule.items = append(rule.items, item)
	}
	if len(options.SourcePort) > 0 {
		rule.items = append(rule.items, NewPortItem(true, options.SourcePort))
	}
	if len(options.Port) > 0 {
		rule.items = append(rule.items, NewPortItem(false, options.Port))
	}
	return rule, nil
}

func (r *DefaultRule) Match(metadata *adapter.InboundContext) bool {
	for _, item := range r.items {
		if item.Match(metadata) {
			return true
		}
	}
	return false
}

func (r *DefaultRule) Outbound() string {
	return r.outbound
}

func (r *DefaultRule) String() string {
	return strings.Join(common.Map(r.items, F.ToString0[RuleItem]), " ")
}
