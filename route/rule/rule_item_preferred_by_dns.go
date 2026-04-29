package rule

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

var _ RuleItem = (*PreferredByDNSItem)(nil)

type PreferredByDNSItem struct {
	ctx           context.Context
	transportTags []string
	transports    []adapter.DNSTransportWithPreferredDomain
}

func NewPreferredByDNSItem(ctx context.Context, transportTags []string) *PreferredByDNSItem {
	return &PreferredByDNSItem{
		ctx:           ctx,
		transportTags: transportTags,
	}
}

func (r *PreferredByDNSItem) Start() error {
	transportManager := service.FromContext[adapter.DNSTransportManager](r.ctx)
	for _, transportTag := range r.transportTags {
		rawTransport, loaded := transportManager.Transport(transportTag)
		if !loaded {
			return E.New("DNS server not found: ", transportTag)
		}
		transportWithPreferredDomain, withPreferredDomain := rawTransport.(adapter.DNSTransportWithPreferredDomain)
		if !withPreferredDomain {
			return E.New("DNS server type does not support preferred_by: ", rawTransport.Type())
		}
		r.transports = append(r.transports, transportWithPreferredDomain)
	}
	return nil
}

func (r *PreferredByDNSItem) Match(metadata *adapter.InboundContext) bool {
	var domainHost string
	if metadata.Domain != "" {
		domainHost = metadata.Domain
	} else {
		domainHost = metadata.Destination.Fqdn
	}
	if domainHost == "" {
		return false
	}
	canonical := mDNS.CanonicalName(domainHost)
	for _, transport := range r.transports {
		if transport.PreferredDomain(canonical) {
			return true
		}
	}
	return false
}

func (r *PreferredByDNSItem) String() string {
	description := "preferred_by="
	pLen := len(r.transportTags)
	if pLen == 1 {
		description += F.ToString(r.transportTags[0])
	} else {
		description += "[" + strings.Join(F.MapToString(r.transportTags), " ") + "]"
	}
	return description
}
