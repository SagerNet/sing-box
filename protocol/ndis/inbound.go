//go:build windows

package ndis

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/wiresock/ndisapi-go"
	"go4.org/netipx"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.NDISInboundOptions](registry, C.TypeNDIS, NewInbound)
}

type Inbound struct {
	inbound.Adapter
	ctx                         context.Context
	router                      adapter.Router
	logger                      log.ContextLogger
	api                         *ndisapi.NdisApi
	tracker                     conntrack.Tracker
	routeAddress                []netip.Prefix
	routeExcludeAddress         []netip.Prefix
	routeRuleSet                []adapter.RuleSet
	routeRuleSetCallback        []*list.Element[adapter.RuleSetUpdateCallback]
	routeExcludeRuleSet         []adapter.RuleSet
	routeExcludeRuleSetCallback []*list.Element[adapter.RuleSetUpdateCallback]
	stack                       *Stack
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.NDISInboundOptions) (adapter.Inbound, error) {
	api, err := ndisapi.NewNdisApi()
	if err != nil {
		return nil, E.Cause(err, "create NDIS API")
	}
	//if !api.IsDriverLoaded() {
	//	return nil, E.New("missing NDIS driver")
	//}
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	trackerOut := service.FromContext[conntrack.Tracker](ctx)
	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}
	var (
		routeRuleSet        []adapter.RuleSet
		routeExcludeRuleSet []adapter.RuleSet
	)
	for _, routeAddressSet := range options.RouteAddressSet {
		ruleSet, loaded := router.RuleSet(routeAddressSet)
		if !loaded {
			return nil, E.New("parse route_address_set: rule-set not found: ", routeAddressSet)
		}
		ruleSet.IncRef()
		routeRuleSet = append(routeRuleSet, ruleSet)
	}
	for _, routeExcludeAddressSet := range options.RouteExcludeAddressSet {
		ruleSet, loaded := router.RuleSet(routeExcludeAddressSet)
		if !loaded {
			return nil, E.New("parse route_exclude_address_set: rule-set not found: ", routeExcludeAddressSet)
		}
		ruleSet.IncRef()
		routeExcludeRuleSet = append(routeExcludeRuleSet, ruleSet)
	}
	trackerIn := conntrack.NewDefaultTracker(false, 0)
	return &Inbound{
		Adapter:             inbound.NewAdapter(C.TypeNDIS, tag),
		ctx:                 ctx,
		router:              router,
		logger:              logger,
		api:                 api,
		tracker:             trackerIn,
		routeRuleSet:        routeRuleSet,
		routeExcludeRuleSet: routeExcludeRuleSet,
		stack: &Stack{
			ctx:                 ctx,
			logger:              logger,
			network:             networkManager,
			trackerIn:           trackerIn,
			trackerOut:          trackerOut,
			api:                 api,
			udpTimeout:          udpTimeout,
			routeAddress:        options.RouteAddress,
			routeExcludeAddress: options.RouteExcludeAddress,
		},
	}, nil
}

func (t *Inbound) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateStart:
		monitor := taskmonitor.New(t.logger, C.StartTimeout)
		var (
			routeAddressSet        []*netipx.IPSet
			routeExcludeAddressSet []*netipx.IPSet
		)
		for _, routeRuleSet := range t.routeRuleSet {
			ipSets := routeRuleSet.ExtractIPSet()
			if len(ipSets) == 0 {
				t.logger.Warn("route_address_set: no destination IP CIDR rules found in rule-set: ", routeRuleSet.Name())
			}
			t.routeRuleSetCallback = append(t.routeRuleSetCallback, routeRuleSet.RegisterCallback(t.updateRouteAddressSet))
			routeRuleSet.DecRef()
			routeAddressSet = append(routeAddressSet, ipSets...)
		}
		for _, routeExcludeRuleSet := range t.routeExcludeRuleSet {
			ipSets := routeExcludeRuleSet.ExtractIPSet()
			if len(ipSets) == 0 {
				t.logger.Warn("route_exclude_address_set: no destination IP CIDR rules found in rule-set: ", routeExcludeRuleSet.Name())
			}
			t.routeExcludeRuleSetCallback = append(t.routeExcludeRuleSetCallback, routeExcludeRuleSet.RegisterCallback(t.updateRouteAddressSet))
			routeExcludeRuleSet.DecRef()
			routeExcludeAddressSet = append(routeExcludeAddressSet, ipSets...)
		}
		t.stack.routeAddressSet = routeAddressSet
		t.stack.routeExcludeAddressSet = routeExcludeAddressSet
		monitor.Start("starting NDIS stack")
		t.stack.handler = t
		err := t.stack.Start()
		monitor.Finish()
		if err != nil {
			return E.Cause(err, "starting NDIS stack")
		}
	}
	return nil
}

func (t *Inbound) Close() error {
	if t.api != nil {
		t.stack.Close()
		t.api.Close()
	}
	return nil
}

func (t *Inbound) PrepareConnection(network string, source M.Socksaddr, destination M.Socksaddr) error {
	return t.router.PreMatch(adapter.InboundContext{
		Inbound:     t.Tag(),
		InboundType: C.TypeNDIS,
		Network:     network,
		Source:      source,
		Destination: destination,
	})
}

func (t *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.Tag()
	metadata.InboundType = C.TypeNDIS
	metadata.Source = source
	metadata.Destination = destination
	t.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	done, err := t.tracker.NewConnEx(conn)
	if err != nil {
		t.logger.ErrorContext(ctx, E.Cause(err, "track inbound connection"))
		return
	}
	t.router.RouteConnectionEx(ctx, conn, metadata, N.AppendClose(onClose, done))
}

func (t *Inbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.Tag()
	metadata.InboundType = C.TypeNDIS
	metadata.Source = source
	metadata.Destination = destination
	t.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	done, err := t.tracker.NewPacketConnEx(conn)
	if err != nil {
		t.logger.ErrorContext(ctx, E.Cause(err, "track inbound connection"))
		return
	}
	t.router.RoutePacketConnectionEx(ctx, conn, metadata, N.AppendClose(onClose, done))
}

func (t *Inbound) updateRouteAddressSet(it adapter.RuleSet) {
	t.stack.routeAddressSet = common.FlatMap(t.routeRuleSet, adapter.RuleSet.ExtractIPSet)
	t.stack.routeExcludeAddressSet = common.FlatMap(t.routeExcludeRuleSet, adapter.RuleSet.ExtractIPSet)
}
