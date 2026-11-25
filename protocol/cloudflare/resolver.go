//go:build with_cloudflared

package cloudflare

import (
	"context"
	"net"
	"net/netip"
	"sort"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
	"golang.org/x/net/dns/dnsmessage"
)

type routerResolver struct {
	dnsRouter    adapter.DNSRouter
	queryOptions adapter.DNSQueryOptions
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

func (r *routerResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	message := &mDNS.Msg{}
	message.SetQuestion(mDNS.Fqdn(name), mDNS.TypeTXT)
	response, err := r.dnsRouter.Exchange(ctx, message, r.queryOptions)
	if err != nil {
		return nil, err
	}
	rawResponse, err := response.Pack()
	if err != nil {
		return nil, E.Cause(err, "pack TXT response")
	}
	var parsedResponse dnsmessage.Message
	err = parsedResponse.Unpack(rawResponse)
	if err != nil {
		return nil, E.Cause(err, "unpack TXT response")
	}
	return common.FilterIsInstance(parsedResponse.Answers, func(it dnsmessage.Resource) (string, bool) {
		record, isTXT := it.Body.(*dnsmessage.TXTResource)
		if !isTXT {
			return "", false
		}
		return strings.Join(record.TXT, ""), true
	}), nil
}
