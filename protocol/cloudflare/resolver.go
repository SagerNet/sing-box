//go:build with_cloudflared

package cloudflare

import (
	"context"
	"net"
	"net/netip"
	"sort"
	"strings"

	"github.com/sagernet/sing-box/adapter"

	mDNS "github.com/miekg/dns"
)

type routerResolver struct {
	dnsRouter    adapter.DNSRouter
	queryOptions adapter.DNSQueryOptions
}

func newRouterResolver(dnsRouter adapter.DNSRouter, queryOptions adapter.DNSQueryOptions) *routerResolver {
	return &routerResolver{dnsRouter: dnsRouter, queryOptions: queryOptions}
}

func (r *routerResolver) LookupNetIP(ctx context.Context, host string) ([]netip.Addr, error) {
	return r.dnsRouter.Lookup(ctx, strings.TrimSuffix(host, "."), r.queryOptions)
}

func (r *routerResolver) LookupSRV(ctx context.Context, service, proto, name string) ([]*net.SRV, error) {
	message := &mDNS.Msg{}
	message.SetQuestion(mDNS.Fqdn("_"+service+"._"+proto+"."+name), mDNS.TypeSRV)
	response, err := r.dnsRouter.Exchange(ctx, message, r.queryOptions)
	if err != nil {
		return nil, err
	}
	var records []*net.SRV
	for _, answer := range response.Answer {
		record, isSRV := answer.(*mDNS.SRV)
		if !isSRV {
			continue
		}
		records = append(records, &net.SRV{
			Target:   record.Target,
			Port:     record.Port,
			Priority: record.Priority,
			Weight:   record.Weight,
		})
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Priority != records[j].Priority {
			return records[i].Priority < records[j].Priority
		}
		return records[i].Weight > records[j].Weight
	})
	return records, nil
}
