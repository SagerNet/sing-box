package bridge

import (
	"context"
	"net"
	"net/netip"

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
	_ adapter.Outbound     = (*Outbound)(nil)
	_ adapter.FlowOutbound = (*Outbound)(nil)
	_ adapter.Lifecycle    = (*Outbound)(nil)
)

type Backend interface {
	adapter.Lifecycle
	tun.Port
}

type Outbound struct {
	outbound.Adapter
	backend Backend
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.BridgeOutboundOptions) (adapter.Outbound, error) {
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	outboundBackend, err := newBackend(ctx, logger, networkManager, tag, options)
	if err != nil {
		return nil, err
	}
	return &Outbound{
		Adapter: outbound.NewAdapter(C.TypeBridge, tag, []string{N.NetworkTCP, N.NetworkUDP, N.NetworkICMP}, nil),
		backend: outboundBackend,
	}, nil
}

func (o *Outbound) Start(stage adapter.StartStage) error {
	return o.backend.Start(stage)
}

func (o *Outbound) Close() error {
	return o.backend.Close()
}

func (o *Outbound) SupportsFlow(network string) bool {
	return true
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
	return nil, E.New("Only L3 traffic is supported by bridge")
}

func (o *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, E.New("Only L3 traffic is supported by bridge")
}
