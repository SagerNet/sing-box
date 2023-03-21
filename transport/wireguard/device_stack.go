//go:build with_gvisor

package wireguard

import (
	"context"
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	wgTun "github.com/sagernet/wireguard-go/tun"

	"gvisor.dev/gvisor/pkg/bufferv2"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

var _ NatDevice = (*StackDevice)(nil)

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
	mapping        *tun.NatMapping
	writer         *tun.NatWriter
}

func NewStackDevice(localAddresses []netip.Prefix, mtu uint32, ipRewrite bool) (*StackDevice, error) {
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
		mapping:        tun.NewNatMapping(ipRewrite),
	}
	err := ipStack.CreateNIC(defaultNIC, (*wireEndpoint)(tunDevice))
	if err != nil {
		return nil, E.New(err.String())
	}
	for _, prefix := range localAddresses {
		addr := tcpip.Address(prefix.Addr().AsSlice())
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
	if ipRewrite {
		tunDevice.writer = tun.NewNatWriter(tunDevice.Inet4Address(), tunDevice.Inet6Address())
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
		Addr: tcpip.Address(destination.Addr.AsSlice()),
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
		tcpConn, err := gonet.DialTCPWithBind(ctx, w.stack, bind, addr, networkProtocol)
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
	if destination.IsIPv4() || w.addr6 == "" {
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
	return M.AddrFromIP(net.IP(w.addr4))
}

func (w *StackDevice) Inet6Address() netip.Addr {
	return M.AddrFromIP(net.IP(w.addr6))
}

func (w *StackDevice) Start() error {
	w.events <- wgTun.EventUp
	return nil
}

func (w *StackDevice) File() *os.File {
	return nil
}

func (w *StackDevice) Read(p []byte, offset int) (n int, err error) {
	select {
	case packetBuffer, ok := <-w.outbound:
		if !ok {
			return 0, os.ErrClosed
		}
		defer packetBuffer.DecRef()
		p = p[offset:]
		for _, slice := range packetBuffer.AsSlices() {
			n += copy(p[n:], slice)
		}
		return
	case packet := <-w.packetOutbound:
		defer packet.Release()
		n = copy(p[offset:], packet.Bytes())
		return
	case <-w.done:
		return 0, os.ErrClosed
	}
}

func (w *StackDevice) Write(p []byte, offset int) (n int, err error) {
	p = p[offset:]
	if len(p) == 0 {
		return
	}
	handled, err := w.mapping.WritePacket(p)
	if handled {
		return len(p), err
	}
	var networkProtocol tcpip.NetworkProtocolNumber
	switch header.IPVersion(p) {
	case header.IPv4Version:
		networkProtocol = header.IPv4ProtocolNumber
	case header.IPv6Version:
		networkProtocol = header.IPv6ProtocolNumber
	}
	packetBuffer := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: bufferv2.MakeWithData(p),
	})
	defer packetBuffer.DecRef()
	w.dispatcher.DeliverNetworkPacket(networkProtocol, packetBuffer)
	n = len(p)
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

func (w *StackDevice) Events() chan wgTun.Event {
	return w.events
}

func (w *StackDevice) Close() error {
	select {
	case <-w.done:
		return os.ErrClosed
	default:
	}
	w.stack.Close()
	for _, endpoint := range w.stack.CleanupEndpoints() {
		endpoint.Abort()
	}
	w.stack.Wait()
	close(w.done)
	return nil
}

func (w *StackDevice) CreateDestination(session tun.RouteSession, conn tun.RouteContext) tun.DirectDestination {
	w.mapping.CreateSession(session, conn)
	return &stackNatDestination{
		device:  w,
		session: session,
	}
}

type stackNatDestination struct {
	device  *StackDevice
	session tun.RouteSession
}

func (d *stackNatDestination) WritePacket(buffer *buf.Buffer) error {
	if d.device.writer != nil {
		d.device.writer.RewritePacket(buffer.Bytes())
	}
	d.device.packetOutbound <- buffer
	return nil
}

func (d *stackNatDestination) WritePacketBuffer(buffer *stack.PacketBuffer) error {
	if d.device.writer != nil {
		d.device.writer.RewritePacketBuffer(buffer)
	}
	d.device.outbound <- buffer
	return nil
}

func (d *stackNatDestination) Close() error {
	d.device.mapping.DeleteSession(d.session)
	return nil
}

func (d *stackNatDestination) Timeout() bool {
	return false
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
	return stack.CapabilityNone
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
