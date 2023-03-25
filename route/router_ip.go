package route

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
)

func (r *Router) RouteIPConnection(ctx context.Context, conn tun.RouteContext, metadata adapter.InboundContext) tun.RouteAction {
	if r.fakeIPStore != nil && r.fakeIPStore.Contains(metadata.Destination.Addr) {
		domain, loaded := r.fakeIPStore.Lookup(metadata.Destination.Addr)
		if !loaded {
			r.logger.ErrorContext(ctx, "missing fakeip context")
			return (*tun.ActionReturn)(nil)
		}
		metadata.Destination = M.Socksaddr{
			Fqdn: domain,
			Port: metadata.Destination.Port,
		}
		r.logger.DebugContext(ctx, "found fakeip domain: ", domain)
	}
	if r.dnsReverseMapping != nil && metadata.Domain == "" {
		domain, loaded := r.dnsReverseMapping.Query(metadata.Destination.Addr)
		if loaded {
			metadata.Domain = domain
			r.logger.DebugContext(ctx, "found reserve mapped domain: ", metadata.Domain)
		}
	}
	if metadata.Destination.IsFqdn() && dns.DomainStrategy(metadata.InboundOptions.DomainStrategy) != dns.DomainStrategyAsIS {
		addresses, err := r.Lookup(adapter.WithContext(ctx, &metadata), metadata.Destination.Fqdn, dns.DomainStrategy(metadata.InboundOptions.DomainStrategy))
		if err != nil {
			r.logger.ErrorContext(ctx, err)
			return (*tun.ActionReturn)(nil)
		}
		metadata.DestinationAddresses = addresses
		r.dnsLogger.DebugContext(ctx, "resolved [", strings.Join(F.MapToString(metadata.DestinationAddresses), " "), "]")
	}
	for i, rule := range r.ipRules {
		if rule.Match(&metadata) {
			if rule.Action() == tun.ActionTypeBlock {
				r.logger.InfoContext(ctx, "match[", i, "] ", rule.String(), " => block")
				return (*tun.ActionBlock)(nil)
			}
			detour := rule.Outbound()
			r.logger.InfoContext(ctx, "match[", i, "] ", rule.String(), " => ", detour)
			outbound, loaded := r.Outbound(detour)
			if !loaded {
				r.logger.ErrorContext(ctx, "outbound not found: ", detour)
				break
			}
			ipOutbound, loaded := outbound.(adapter.IPOutbound)
			if !loaded {
				r.logger.ErrorContext(ctx, "outbound have no ip connection support: ", detour)
				break
			}
			destination, err := ipOutbound.NewIPConnection(ctx, conn, metadata)
			if err != nil {
				r.logger.ErrorContext(ctx, err)
				break
			}
			return &tun.ActionDirect{DirectDestination: destination}
		}
	}
	return (*tun.ActionReturn)(nil)
}

func (r *Router) NatRequired(outbound string) bool {
	for _, ipRule := range r.ipRules {
		if ipRule.Outbound() == outbound {
			return true
		}
	}
	return false
}
