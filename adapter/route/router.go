package route

import (
	"context"
	"net"

	"github.com/oschwald/geoip2-golang"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Router = (*Router)(nil)

type Router struct {
	logger          log.Logger
	defaultOutbound adapter.Outbound
	outboundByTag   map[string]adapter.Outbound

	rules     []adapter.Rule
	geoReader *geoip2.Reader
}

func NewRouter(logger log.Logger) *Router {
	return &Router{
		logger:        logger.WithPrefix("router: "),
		outboundByTag: make(map[string]adapter.Outbound),
	}
}

func (r *Router) DefaultOutbound() adapter.Outbound {
	if r.defaultOutbound == nil {
		panic("missing default outbound")
	}
	return r.defaultOutbound
}

func (r *Router) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := r.outboundByTag[tag]
	return outbound, loaded
}

func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	for _, rule := range r.rules {
		if rule.Match(metadata) {
			r.logger.WithContext(ctx).Info("match ", rule.String())
			if outbound, loaded := r.Outbound(rule.Outbound()); loaded {
				return outbound.NewConnection(ctx, conn, metadata.Destination)
			}
			r.logger.WithContext(ctx).Error("outbound ", rule.Outbound(), " not found")
		}
	}
	r.logger.WithContext(ctx).Info("no match => ", r.defaultOutbound.Tag())
	return r.defaultOutbound.NewConnection(ctx, conn, metadata.Destination)
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	for _, rule := range r.rules {
		if rule.Match(metadata) {
			r.logger.WithContext(ctx).Info("match ", rule.String())
			if outbound, loaded := r.Outbound(rule.Outbound()); loaded {
				return outbound.NewPacketConnection(ctx, conn, metadata.Destination)
			}
			r.logger.WithContext(ctx).Error("outbound ", rule.Outbound(), " not found")
		}
	}
	r.logger.WithContext(ctx).Info("no match => ", r.defaultOutbound.Tag())
	return r.defaultOutbound.NewPacketConnection(ctx, conn, metadata.Destination)
}

func (r *Router) Close() error {
	return common.Close(
		common.PtrOrNil(r.geoReader),
	)
}

func (r *Router) UpdateOutbounds(outbounds []adapter.Outbound) {
	var defaultOutbound adapter.Outbound
	outboundByTag := make(map[string]adapter.Outbound)
	if len(outbounds) > 0 {
		defaultOutbound = outbounds[0]
	}
	for _, outbound := range outbounds {
		outboundByTag[outbound.Tag()] = outbound
	}
	r.defaultOutbound = defaultOutbound
	r.outboundByTag = outboundByTag
}

func (r *Router) UpdateRules(options []option.Rule) error {
	rules := make([]adapter.Rule, 0, len(options))
	for i, rule := range options {
		switch rule.Type {
		case "", C.RuleTypeDefault:
			rules = append(rules, NewDefaultRule(i, rule.DefaultOptions))
		}
	}
	r.rules = rules
	return nil
}
