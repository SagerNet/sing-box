package bridge

import (
	"context"
	"net"
	"net/netip"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.BridgeOutboundOptions](registry, C.TypeBridge, NewOutbound)
}

var (
	_ adapter.Outbound                    = (*Outbound)(nil)
	_ adapter.FlowOutbound                = (*Outbound)(nil)
	_ adapter.OutboundWithPreferredRoutes = (*Outbound)(nil)
	_ adapter.Lifecycle                   = (*Outbound)(nil)
)

type Backend interface {
	adapter.Lifecycle
	tun.Port
}

type Outbound struct {
	outbound.Adapter
	logger            log.ContextLogger
	networkManager    adapter.NetworkManager
	platformInterface adapter.PlatformInterface
	backend           Backend
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.BridgeOutboundOptions) (adapter.Outbound, error) {
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	outboundBackend, err := newBackend(ctx, logger, networkManager, tag, options)
	if err != nil {
		return nil, err
	}
	return &Outbound{
		Adapter:           outbound.NewAdapter(C.TypeBridge, tag, []string{N.NetworkTCP, N.NetworkUDP, N.NetworkICMP}, nil),
		logger:            logger,
		networkManager:    networkManager,
		platformInterface: service.FromContext[adapter.PlatformInterface](ctx),
		backend:           outboundBackend,
	}, nil
}

func (o *Outbound) Start(stage adapter.StartStage) error {
	return o.backend.Start(stage)
}

func (o *Outbound) Close() error {
	return o.backend.Close()
}

func (o *Outbound) PreferredDomain(metadata *adapter.InboundContext, domain string) bool {
	return false
}

func (o *Outbound) PreferredAddress(metadata *adapter.InboundContext, address netip.Addr) bool {
	return metadata.PreMatch && !o.isLocalDestination(address)
}

func (o *Outbound) PreMatchFlow(network string, destination netip.Addr) adapter.PreMatchAction {
	if o.isLocalDestination(destination) {
		o.logger.Warn("rejected connection to local destination ", destination, ": traffic to local addresses is not supported by bridge, exclude them in route rules")
		return adapter.PreMatchReject
	}
	return adapter.PreMatchFlow
}

func (o *Outbound) isLocalDestination(destination netip.Addr) bool {
	if !destination.IsValid() {
		return false
	}
	destination = destination.Unmap()
	if destination.IsLoopback() || destination.IsUnspecified() {
		return true
	}
	if o.platformInterface != nil && slices.Contains(o.platformInterface.MyInterfaceAddress(), destination) {
		return true
	}
	for _, netInterface := range o.networkManager.InterfaceFinder().Interfaces() {
		for _, prefix := range netInterface.Addresses {
			if prefix.Addr() == destination {
				return true
			}
		}
	}
	return false
}

func (o *Outbound) PortAddresses() (netip.Addr, netip.Addr) {
	return o.backend.PortAddresses()
}

func (o *Outbound) PortMTU() uint32 {
	return o.backend.PortMTU()
}

func (o *Outbound) AttachReturn(returnPath tun.Return) error {
	return o.backend.AttachReturn(returnPath)
}

func (o *Outbound) DetachReturn(returnPath tun.Return) error {
	return o.backend.DetachReturn(returnPath)
}

func (o *Outbound) WritePackets(packets [][]byte) error {
	return o.backend.WritePackets(packets)
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, E.New("only L3 traffic is supported by bridge")
}

func (o *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, E.New("only L3 traffic is supported by bridge")
}
