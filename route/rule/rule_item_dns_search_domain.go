package rule

import (
	"context"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

var _ RuleItem = (*DNSSearchDomainItem)(nil)

type DNSSearchDomainItem struct {
	ctx           context.Context
	transportTags []string
	searchDomains [][]string
	transports    []adapter.DNSTransportWithConfiguration
	description   string
}

func NewDNSSearchDomainItem(ctx context.Context, searchDomains *badjson.TypedMap[string, badoption.Listable[string]]) *DNSSearchDomainItem {
	item := &DNSSearchDomainItem{
		ctx: ctx,
	}
	var entryDescriptions []string
	for _, entry := range searchDomains.Entries() {
		item.transportTags = append(item.transportTags, entry.Key)
		item.searchDomains = append(item.searchDomains, common.Map(entry.Value, mDNS.CanonicalName))
		entryDescriptions = append(entryDescriptions, entry.Key+"="+strings.Join(entry.Value, ","))
	}
	item.description = "dns_search_domain=[" + strings.Join(entryDescriptions, " ") + "]"
	return item
}

func (r *DNSSearchDomainItem) Start() error {
	transportManager := service.FromContext[adapter.DNSTransportManager](r.ctx)
	for _, transportTag := range r.transportTags {
		rawTransport, loaded := transportManager.Transport(transportTag)
		if !loaded {
			return E.New("DNS server not found: ", transportTag)
		}
		transportWithConfiguration, withConfiguration := rawTransport.(adapter.DNSTransportWithConfiguration)
		if !withConfiguration {
			return E.New("DNS server type does not support dns_search_domain: ", rawTransport.Type())
		}
		r.transports = append(r.transports, transportWithConfiguration)
	}
	return nil
}

func (r *DNSSearchDomainItem) Match(metadata *adapter.InboundContext) bool {
	for i, transport := range r.transports {
		domains := r.searchDomains[i]
		if !slices.ContainsFunc(transport.SearchDomains(), func(searchDomain string) bool {
			return slices.Contains(domains, mDNS.CanonicalName(searchDomain))
		}) {
			return false
		}
	}
	return true
}

func (r *DNSSearchDomainItem) String() string {
	return r.description
}
