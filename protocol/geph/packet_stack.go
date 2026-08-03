//go:build with_gvisor

package geph

import (
	"context"
	"net"
	"net/netip"
	"os"
	"sync"

	"github.com/sagernet/gvisor/pkg/buffer"
	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/adapters/gonet"
	"github.com/sagernet/gvisor/pkg/tcpip/header"
	"github.com/sagernet/gvisor/pkg/tcpip/network/ipv4"
	"github.com/sagernet/gvisor/pkg/tcpip/network/ipv6"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type packetStack interface {
	DialContext(context.Context, string, M.Socksaddr) (net.Conn, error)
	ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error)
	Close() error
}

type gephStack struct {
	stack        *stack.Stack
	mtu          uint32
	incoming     <-chan []byte
	send         func([]byte) error
	done         chan struct{}
	closeOnce    sync.Once
	dispatcher   stack.NetworkDispatcher
	inet4, inet6 netip.Addr
}

func newPacketStack(incoming <-chan []byte, send func([]byte) error) (packetStack, error) {
	g := &gephStack{mtu: 16384, incoming: incoming, send: send, done: make(chan struct{})}
	ipStack, err := tun.NewGVisorStackWithOptions((*gephLink)(g), stack.NICOptions{}, true)
	if err != nil {
		return nil, err
	}
	for _, prefix := range []netip.Prefix{netip.MustParsePrefix("100.64.0.1/10"), netip.MustParsePrefix("fd00:6765::1/64")} {
		address := tun.AddressFromAddr(prefix.Addr())
		protocol := ipv4.ProtocolNumber
		if prefix.Addr().Is6() {
			protocol = ipv6.ProtocolNumber
			g.inet6 = prefix.Addr()
		} else {
			g.inet4 = prefix.Addr()
		}
		gErr := ipStack.AddProtocolAddress(tun.DefaultNIC, tcpip.ProtocolAddress{Protocol: protocol, AddressWithPrefix: tcpip.AddressWithPrefix{Address: address, PrefixLen: prefix.Bits()}}, stack.AddressProperties{})
		if gErr != nil {
			ipStack.Close()
			return nil, E.New("add Geph stack address: ", gErr)
		}
	}
	g.stack = ipStack
	go g.readLoop()
	return g, nil
}

func (g *gephStack) readLoop() {
	for packet := range g.incoming {
		if len(packet) == 0 {
			continue
		}
		var protocol tcpip.NetworkProtocolNumber
		switch header.IPVersion(packet) {
		case header.IPv4Version:
			protocol = header.IPv4ProtocolNumber
		case header.IPv6Version:
			protocol = header.IPv6ProtocolNumber
		default:
			continue
		}
		pb := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(packet)})
		if g.dispatcher != nil {
			g.dispatcher.DeliverNetworkPacket(protocol, pb)
		}
		pb.DecRef()
	}
}

func (g *gephStack) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	var local netip.Addr
	var protocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		local, protocol = g.inet4, header.IPv4ProtocolNumber
	} else {
		local, protocol = g.inet6, header.IPv6ProtocolNumber
	}
	if !local.IsValid() {
		return nil, E.New("Geph stack has no address for destination family")
	}
	bind := tcpip.FullAddress{NIC: tun.DefaultNIC, Addr: tun.AddressFromAddr(local)}
	remote := tcpip.FullAddress{NIC: tun.DefaultNIC, Addr: tun.AddressFromAddr(destination.Addr), Port: destination.Port}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		return gonet.DialTCPWithBind(ctx, g.stack, bind, remote, protocol)
	case N.NetworkUDP:
		return gonet.DialUDP(g.stack, &bind, &remote, protocol)
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (g *gephStack) ListenPacket(_ context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	var local netip.Addr
	var protocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		local, protocol = g.inet4, header.IPv4ProtocolNumber
	} else {
		local, protocol = g.inet6, header.IPv6ProtocolNumber
	}
	if !local.IsValid() {
		return nil, E.New("Geph stack has no address for destination family")
	}
	bind := tcpip.FullAddress{NIC: tun.DefaultNIC, Addr: tun.AddressFromAddr(local)}
	return gonet.DialUDP(g.stack, &bind, nil, protocol)
}

func (g *gephStack) Close() error {
	g.closeOnce.Do(func() {
		close(g.done)
		g.stack.Close()
		for _, ep := range g.stack.CleanupEndpoints() {
			ep.Abort()
		}
		g.stack.Wait()
	})
	return nil
}

type gephLink gephStack

func (e *gephLink) MTU() uint32                      { return e.mtu }
func (e *gephLink) SetMTU(uint32)                    {}
func (e *gephLink) MaxHeaderLength() uint16          { return 0 }
func (e *gephLink) LinkAddress() tcpip.LinkAddress   { return "" }
func (e *gephLink) SetLinkAddress(tcpip.LinkAddress) {}
func (e *gephLink) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityRXChecksumOffload
}
func (e *gephLink) Attach(d stack.NetworkDispatcher)        { e.dispatcher = d }
func (e *gephLink) IsAttached() bool                        { return e.dispatcher != nil }
func (e *gephLink) Wait()                                   {}
func (e *gephLink) ARPHardwareType() header.ARPHardwareType { return header.ARPHardwareNone }
func (e *gephLink) AddHeader(*stack.PacketBuffer)           {}
func (e *gephLink) ParseHeader(*stack.PacketBuffer) bool    { return true }
func (e *gephLink) Close()                                  {}
func (e *gephLink) SetOnCloseAction(func())                 {}
func (e *gephLink) WritePackets(list stack.PacketBufferList) (int, tcpip.Error) {
	for _, packet := range list.AsSlice() {
		var data []byte
		for _, view := range packet.AsSlices() {
			data = append(data, view...)
		}
		if err := e.send(data); err != nil {
			return 0, &tcpip.ErrClosedForSend{}
		}
	}
	return list.Len(), nil
}

var _ stack.LinkEndpoint = (*gephLink)(nil)
var _ = os.ErrClosed
