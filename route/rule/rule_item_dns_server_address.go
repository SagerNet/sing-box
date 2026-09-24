package rule

import (
	"context"
	"net/netip"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/service"
)

var _ RuleItem = (*DNSServerAddressItem)(nil)

type DNSServerAddressItem struct {
	ctx             context.Context
	transportTags   []string
	serverAddresses [][]netip.Prefix
	transports      []adapter.DNSTransportWithConfiguration
	description     string
}

func NewDNSServerAddressItem(ctx context.Context, serverAddresses *badjson.TypedMap[string, badoption.Listable[*badoption.Prefixable]]) *DNSServerAddressItem {
	item := &DNSServerAddressItem{
		ctx: ctx,
	}
	var entryDescriptions []string
	for _, entry := range serverAddresses.Entries() {
		prefixes := make([]netip.Prefix, 0, len(entry.Value))
		for _, prefixable := range entry.Value {
			prefixes = append(prefixes, prefixable.Build(netip.Prefix{}))
		}
		item.transportTags = append(item.transportTags, entry.Key)
		item.serverAddresses = append(item.serverAddresses, prefixes)
		entryDescriptions = append(entryDescriptions, entry.Key+"="+strings.Join(common.Map(prefixes, netip.Prefix.String), ","))
	}
	item.description = "dns_server_address=[" + strings.Join(entryDescriptions, " ") + "]"
	return item
}

func (r *DNSServerAddressItem) Start() error {
	transportManager := service.FromContext[adapter.DNSTransportManager](r.ctx)
	for _, transportTag := range r.transportTags {
		rawTransport, loaded := transportManager.Transport(transportTag)
		if !loaded {
			return E.New("DNS server not found: ", transportTag)
		}
		transportWithConfiguration, withConfiguration := rawTransport.(adapter.DNSTransportWithConfiguration)
		if !withConfiguration {
			return E.New("DNS server type does not support dns_server_address: ", rawTransport.Type())
		}
		r.transports = append(r.transports, transportWithConfiguration)
	}
	return nil
}

func (r *DNSServerAddressItem) Match(metadata *adapter.InboundContext) bool {
	for i, transport := range r.transports {
		prefixes := r.serverAddresses[i]
		if !slices.ContainsFunc(transport.ServerAddresses(), func(address netip.Addr) bool {
			unwrappedAddress := address.Unmap().WithZone("")
			return slices.ContainsFunc(prefixes, func(prefix netip.Prefix) bool {
				return prefix.Contains(unwrappedAddress)
			})
		}) {
			return false
		}
	}
	return true
}

func (r *DNSServerAddressItem) String() string {
	return r.description
}
