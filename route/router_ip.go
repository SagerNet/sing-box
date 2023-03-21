package route

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
)

func (r *Router) RouteIPConnection(ctx context.Context, conn tun.RouteContext, metadata adapter.InboundContext) tun.RouteAction {
	for i, rule := range r.ipRules {
		if rule.Match(&metadata) {
			if rule.Action() == tun.ActionTypeReject {
				r.logger.InfoContext(ctx, "match[", i, "] ", rule.String(), " => reject")
				return (*tun.ActionReject)(nil)
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
