package adapter

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"

	"go4.org/netipx"
)

type Router interface {
	Lifecycle
	ConnectionRouter
	PreMatch(metadata InboundContext) PreMatchResult
	ConnectionRouterEx
	RuleSet(tag string) (RuleSet, bool)
	Rules() []Rule
	NeedFindProcess() bool
	NeedFindNeighbor() bool
	NeighborResolver() NeighborResolver
	AppendTracker(tracker ConnectionTracker)
	ResetNetwork()
}

type PreMatchAction uint8

const (
	PreMatchContinue PreMatchAction = iota
	PreMatchFlow
	PreMatchReject
	PreMatchDrop
	PreMatchBypass
)

type PreMatchResult struct {
	Action      PreMatchAction
	Outbound    Outbound
	Destination netip.AddrPort
}

func JudgeFlow(router Router, inbound string, inboundType string, network uint8, source netip.AddrPort, destination netip.AddrPort) tun.FlowVerdict {
	var networkName string
	switch network {
	case uint8(header.TCPProtocolNumber):
		networkName = N.NetworkTCP
	case uint8(header.UDPProtocolNumber):
		networkName = N.NetworkUDP
	case uint8(header.ICMPv4ProtocolNumber), uint8(header.ICMPv6ProtocolNumber):
		networkName = N.NetworkICMP
	default:
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	metadata := InboundContext{
		Inbound:     inbound,
		InboundType: inboundType,
		Network:     networkName,
		Source:      M.SocksaddrFromNetIP(source),
		Destination: M.SocksaddrFromNetIP(destination),
	}
	if networkName == N.NetworkICMP {
		metadata.Source.Port = 0
		metadata.Destination.Port = 0
	}
	result := router.PreMatch(metadata)
	switch result.Action {
	case PreMatchFlow:
		port, isPort := result.Outbound.(tun.Port)
		if !isPort {
			return tun.FlowVerdict{Action: tun.ActionAccept}
		}
		verdict := tun.FlowVerdict{Action: tun.ActionFlow, Port: port}
		if result.Destination.IsValid() {
			destinationPort := result.Destination.Port()
			if networkName == N.NetworkICMP {
				destinationPort = destination.Port()
			}
			verdict.Destination = netip.AddrPortFrom(result.Destination.Addr(), destinationPort)
		}
		return verdict
	case PreMatchReject:
		return tun.FlowVerdict{Action: tun.ActionReject}
	case PreMatchDrop:
		return tun.FlowVerdict{Action: tun.ActionDrop}
	case PreMatchBypass:
		return tun.FlowVerdict{Action: tun.ActionBypass}
	default:
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
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
