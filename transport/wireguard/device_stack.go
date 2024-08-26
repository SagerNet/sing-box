//go:build with_gvisor

package wireguard

import (
	"context"
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/gvisor/pkg/buffer"
	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/adapters/gonet"
	"github.com/sagernet/gvisor/pkg/tcpip/header"
	"github.com/sagernet/gvisor/pkg/tcpip/network/ipv4"
	"github.com/sagernet/gvisor/pkg/tcpip/network/ipv6"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/icmp"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/tcp"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/udp"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

var _ Device = (*StackDevice)(nil)

const defaultNIC tcpip.NICID = 1

type StackDevice struct {
	stack          *stack.Stack
	mtu            uint32
	events         chan wgTun.Event
	outbound       chan *stack.PacketBuffer
	packetOutbound chan *buf.Buffer
	done           chan struct{}
	dispatcher     stack.NetworkDispatcher
	addr4          tcpip.Address
	addr6          tcpip.Address
}

func NewStackDevice(localAddresses []netip.Prefix, mtu uint32) (*StackDevice, error) {
	ipStack := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol4, icmp.NewProtocol6},
		HandleLocal:        true,
	})
	tunDevice := &StackDevice{
		stack:          ipStack,
		mtu:            mtu,
		events:         make(chan wgTun.Event, 1),
		outbound:       make(chan *stack.PacketBuffer, 256),
		packetOutbound: make(chan *buf.Buffer, 256),
		done:           make(chan struct{}),
	}
	err := ipStack.CreateNIC(defaultNIC, (*wireEndpoint)(tunDevice))
	if err != nil {
		return nil, E.New(err.String())
	}
	for _, prefix := range localAddresses {
		addr := tun.AddressFromAddr(prefix.Addr())
		protoAddr := tcpip.ProtocolAddress{
			AddressWithPrefix: tcpip.AddressWithPrefix{
				Address:   addr,
				PrefixLen: prefix.Bits(),
			},
		}
		if prefix.Addr().Is4() {
			tunDevice.addr4 = addr
			protoAddr.Protocol = ipv4.ProtocolNumber
		} else {
			tunDevice.addr6 = addr
			protoAddr.Protocol = ipv6.ProtocolNumber
		}
		err = ipStack.AddProtocolAddress(defaultNIC, protoAddr, stack.AddressProperties{})
		if err != nil {
			return nil, E.New("parse local address ", protoAddr.AddressWithPrefix, ": ", err.String())
		}
	}
	sOpt := tcpip.TCPSACKEnabled(true)
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &sOpt)
	cOpt := tcpip.CongestionControlOption("cubic")
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &cOpt)
	ipStack.AddRoute(tcpip.Route{Destination: header.IPv4EmptySubnet, NIC: defaultNIC})
	ipStack.AddRoute(tcpip.Route{Destination: header.IPv6EmptySubnet, NIC: defaultNIC})
	return tunDevice, nil
}

func (w *StackDevice) NewEndpoint() (stack.LinkEndpoint, error) {
	return (*wireEndpoint)(w), nil
}

func (w *StackDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	addr := tcpip.FullAddress{
		NIC:  defaultNIC,
		Port: destination.Port,
		Addr: tun.AddressFromAddr(destination.Addr),
	}
	bind := tcpip.FullAddress{
		NIC: defaultNIC,
	}
	var networkProtocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		networkProtocol = header.IPv4ProtocolNumber
		bind.Addr = w.addr4
	} else {
		networkProtocol = header.IPv6ProtocolNumber
		bind.Addr = w.addr6
	}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		tcpConn, err := DialTCPWithBind(ctx, w.stack, bind, addr, networkProtocol)
		if err != nil {
			return nil, err
		}
		return tcpConn, nil
	case N.NetworkUDP:
		udpConn, err := gonet.DialUDP(w.stack, &bind, &addr, networkProtocol)
		if err != nil {
			return nil, err
		}
		return udpConn, nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (w *StackDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	bind := tcpip.FullAddress{
		NIC: defaultNIC,
	}
	var networkProtocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		networkProtocol = header.IPv4ProtocolNumber
		bind.Addr = w.addr4
	} else {
		networkProtocol = header.IPv6ProtocolNumber
		bind.Addr = w.addr6
	}
	udpConn, err := gonet.DialUDP(w.stack, &bind, nil, networkProtocol)
	if err != nil {
		return nil, err
	}
	return udpConn, nil
}

func (w *StackDevice) Inet4Address() netip.Addr {
	return tun.AddrFromAddress(w.addr4)
}

func (w *StackDevice) Inet6Address() netip.Addr {
	return tun.AddrFromAddress(w.addr6)
}

func (w *StackDevice) Start() error {
	w.events <- wgTun.EventUp
	return nil
}

func (w *StackDevice) File() *os.File {
	return nil
}

func (w *StackDevice) Read(bufs [][]byte, sizes []int, offset int) (count int, err error) {
	select {
	case packetBuffer, ok := <-w.outbound:
		if !ok {
			return 0, os.ErrClosed
		}
		defer packetBuffer.DecRef()
		p := bufs[0]
		p = p[offset:]
		n := 0
		for _, slice := range packetBuffer.AsSlices() {
			n += copy(p[n:], slice)
		}
		sizes[0] = n
		count = 1
		return
	case packet := <-w.packetOutbound:
		defer packet.Release()
		sizes[0] = copy(bufs[0][offset:], packet.Bytes())
		count = 1
		return
	case <-w.done:
		return 0, os.ErrClosed
	}
}

func (w *StackDevice) Write(bufs [][]byte, offset int) (count int, err error) {
	for _, b := range bufs {
		b = b[offset:]
		if len(b) == 0 {
			continue
		}
		var networkProtocol tcpip.NetworkProtocolNumber
		switch header.IPVersion(b) {
		case header.IPv4Version:
			networkProtocol = header.IPv4ProtocolNumber
		case header.IPv6Version:
			networkProtocol = header.IPv6ProtocolNumber
		}
		packetBuffer := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(b),
		})
		w.dispatcher.DeliverNetworkPacket(networkProtocol, packetBuffer)
		packetBuffer.DecRef()
		count++
	}
	return
}

func (w *StackDevice) Flush() error {
	return nil
}

func (w *StackDevice) MTU() (int, error) {
	return int(w.mtu), nil
}

func (w *StackDevice) Name() (string, error) {
	return "sing-box", nil
}

func (w *StackDevice) Events() <-chan wgTun.Event {
	return w.events
}

func (w *StackDevice) Close() error {
	close(w.done)
	close(w.events)
	w.stack.Close()
	for _, endpoint := range w.stack.CleanupEndpoints() {
		endpoint.Abort()
	}
	w.stack.Wait()
	return nil
}

func (w *StackDevice) BatchSize() int {
	return 1
}

var _ stack.LinkEndpoint = (*wireEndpoint)(nil)

type wireEndpoint StackDevice

func (ep *wireEndpoint) MTU() uint32 {
	return ep.mtu
}

func (ep *wireEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (ep *wireEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (ep *wireEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityRXChecksumOffload
}

func (ep *wireEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	ep.dispatcher = dispatcher
}

func (ep *wireEndpoint) IsAttached() bool {
	return ep.dispatcher != nil
}

func (ep *wireEndpoint) Wait() {
}

func (ep *wireEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (ep *wireEndpoint) AddHeader(buffer *stack.PacketBuffer) {
}

func (ep *wireEndpoint) ParseHeader(ptr *stack.PacketBuffer) bool {
	return true
}

func (ep *wireEndpoint) WritePackets(list stack.PacketBufferList) (int, tcpip.Error) {
	for _, packetBuffer := range list.AsSlice() {
		packetBuffer.IncRef()
		select {
		case <-ep.done:
			return 0, &tcpip.ErrClosedForSend{}
		case ep.outbound <- packetBuffer:
		}
	}
	return list.Len(), nil
}
