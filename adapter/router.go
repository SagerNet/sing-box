package adapter

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-tun"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"

	"go4.org/netipx"
)

type Router interface {
	Lifecycle
	ConnectionRouter
	PreMatch(metadata InboundContext, context tun.DirectRouteContext, timeout time.Duration, supportBypass bool) (tun.DirectRouteDestination, error)
	ConnectionRouterEx
	RuleSet(tag string) (RuleSet, bool)
	Rules() []Rule
	NeedFindProcess() bool
	NeedFindNeighbor() bool
	NeighborResolver() NeighborResolver
	AppendTracker(tracker ConnectionTracker)
	ResetNetwork()
}

type ConnectionTracker interface {
	RoutedConnection(ctx context.Context, conn net.Conn, metadata InboundContext, matchedRule Rule, matchOutbound Outbound) net.Conn
	RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, matchedRule Rule, matchOutbound Outbound) N.PacketConn
}

// Deprecated: Use ConnectionRouterEx instead.
type ConnectionRouter interface {
	RouteConnection(ctx context.Context, conn net.Conn, metadata InboundContext) error
	RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext) error
}

type ConnectionRouterEx interface {
	ConnectionRouter
	RouteConnectionEx(ctx context.Context, conn net.Conn, metadata InboundContext, onClose N.CloseHandlerFunc)
	RoutePacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata InboundContext, onClose N.CloseHandlerFunc)
}

type RuleSet interface {
	Name() string
	StartContext(ctx context.Context, startContext *HTTPStartContext) error
	PostStart() error
	Metadata() RuleSetMetadata
	ExtractIPSet() []*netipx.IPSet
	IncRef()
	DecRef()
	Cleanup()
	RegisterCallback(callback RuleSetUpdateCallback) *list.Element[RuleSetUpdateCallback]
	UnregisterCallback(element *list.Element[RuleSetUpdateCallback])
	Close() error
	HeadlessRule
}

type RuleSetUpdateCallback func(it RuleSet)

type DNSRuleSetUpdateValidator interface {
	ValidateRuleSetMetadataUpdate(tag string, metadata RuleSetMetadata) error
}

// ip_version is not a headless-rule item, so ContainsIPVersionRule is intentionally absent.
type RuleSetMetadata struct {
	ContainsProcessRule      bool
	ContainsWIFIRule         bool
	ContainsIPCIDRRule       bool
	ContainsDNSQueryTypeRule bool
	// ContainsNonIPCIDRRule signals that the rule-set carries at least one sub-rule
	// with a predicate other than destination ip_cidr / ip_set, so it can contribute
	// to DNS pre-response matching. A rule-set where this is false and
	// ContainsIPCIDRRule is true is "pure-IP" and matches nothing before a DNS
	// response is available.
	ContainsNonIPCIDRRule bool
}
