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
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/wireguard-go/device"
	wgTun "github.com/sagernet/wireguard-go/tun"
)

var _ NatDevice = (*stackDevice)(nil)

type stackDevice struct {
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

func newStackDevice(options DeviceOptions) (*stackDevice, error) {
	tunDevice := &stackDevice{
		mtu:            options.MTU,
		events:         make(chan wgTun.Event, 1),
		outbound:       make(chan *stack.PacketBuffer, 256),
		packetOutbound: make(chan *buf.Buffer, 256),
		done:           make(chan struct{}),
		mapping:        tun.NewNatMapping(true),
	}
	ipStack, err := tun.NewGVisorStack((*wireEndpoint)(tunDevice))
	if err != nil {
		return nil, err
	}
	for _, prefix := range options.Address {
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
		gErr := ipStack.AddProtocolAddress(tun.DefaultNIC, protoAddr, stack.AddressProperties{})
		if gErr != nil {
			return nil, E.New("parse local address ", protoAddr.AddressWithPrefix, ": ", gErr.String())
		}
	}
	tunDevice.writer = tun.NewNatWriter(tunDevice.Inet4Address(), tunDevice.Inet6Address())
	tunDevice.stack = ipStack
	if options.Handler != nil {
		ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tun.NewTCPForwarder(options.Context, ipStack, options.Handler).HandlePacket)
		ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, tun.NewUDPForwarder(options.Context, ipStack, options.Handler, options.UDPTimeout).HandlePacket)
		icmpForwarder := tun.NewICMPForwarder(options.Context, ipStack, options.Handler, options.UDPTimeout)
		ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber4, icmpForwarder.HandlePacket)
		ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber6, icmpForwarder.HandlePacket)
	}
	return tunDevice, nil
}

func (w *stackDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	addr := tcpip.FullAddress{
		NIC:  tun.DefaultNIC,
		Port: destination.Port,
		Addr: tun.AddressFromAddr(destination.Addr),
	}
	bind := tcpip.FullAddress{
		NIC: tun.DefaultNIC,
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

func (w *stackDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	bind := tcpip.FullAddress{
		NIC: tun.DefaultNIC,
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

func (w *stackDevice) Inet4Address() netip.Addr {
	return netip.AddrFrom4(w.addr4.As4())
}

func (w *stackDevice) Inet6Address() netip.Addr {
	return netip.AddrFrom16(w.addr6.As16())
}

func (w *stackDevice) SetDevice(device *device.Device) {
}

func (w *stackDevice) Start() error {
	w.events <- wgTun.EventUp
	return nil
}

func (w *stackDevice) File() *os.File {
	return nil
}

func (w *stackDevice) Read(bufs [][]byte, sizes []int, offset int) (count int, err error) {
	select {
	case packet, ok := <-w.outbound:
		if !ok {
			return 0, os.ErrClosed
		}
		defer packet.DecRef()
		var copyN int
		/*rangeIterate(packet.Data().AsRange(), func(view *buffer.View) {
			copyN += copy(bufs[0][offset+copyN:], view.AsSlice())
		})*/
		for _, view := range packet.AsSlices() {
			copyN += copy(bufs[0][offset+copyN:], view)
		}
		sizes[0] = copyN
		return 1, nil
	case packet := <-w.packetOutbound:
		defer packet.Release()
		sizes[0] = copy(bufs[0][offset:], packet.Bytes())
		return 1, nil
	case <-w.done:
		return 0, os.ErrClosed
	}
}

func (w *stackDevice) Write(bufs [][]byte, offset int) (count int, err error) {
	for _, b := range bufs {
		b = b[offset:]
		if len(b) == 0 {
			continue
		}
		handled, err := w.mapping.WritePacket(b)
		if handled {
			if err != nil {
				return count, err
			}
			count++
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

func (w *stackDevice) Flush() error {
	return nil
}

func (w *stackDevice) MTU() (int, error) {
	return int(w.mtu), nil
}

func (w *stackDevice) Name() (string, error) {
	return "sing-box", nil
}

func (w *stackDevice) Events() <-chan wgTun.Event {
	return w.events
}

func (w *stackDevice) Close() error {
	close(w.done)
	close(w.events)
	w.stack.Close()
	for _, endpoint := range w.stack.CleanupEndpoints() {
		endpoint.Abort()
	}
	w.stack.Wait()
	return nil
}

func (w *stackDevice) BatchSize() int {
	return 1
}

var _ stack.LinkEndpoint = (*wireEndpoint)(nil)

type wireEndpoint stackDevice

func (ep *wireEndpoint) MTU() uint32 {
	return ep.mtu
}

func (ep *wireEndpoint) SetMTU(mtu uint32) {
}

func (ep *wireEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (ep *wireEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (ep *wireEndpoint) SetLinkAddress(addr tcpip.LinkAddress) {
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

func (ep *wireEndpoint) Close() {
}

func (ep *wireEndpoint) SetOnCloseAction(f func()) {
}

func (w *stackDevice) CreateDestination(metadata adapter.InboundContext, routeContext tun.DirectRouteContext) (tun.DirectRouteDestination, error) {
	/*	var wq waiter.Queue
		ep, err := raw.NewEndpoint(w.stack, ipv4.ProtocolNumber, icmp.ProtocolNumber4, &wq)
		if err != nil {
			return nil, E.Cause(gonet.TranslateNetstackError(err), "create endpoint")
		}
		err = ep.Connect(tcpip.FullAddress{
			NIC:  tun.DefaultNIC,
			Port: metadata.Destination.Port,
			Addr: tun.AddressFromAddr(metadata.Destination.Addr),
		})
		if err != nil {
			ep.Close()
			return nil, E.Cause(gonet.TranslateNetstackError(err), "ICMP connect ", metadata.Destination)
		}
		fmt.Println("linked ", metadata.Network, " connection to ", metadata.Destination.AddrString())
		destination := &endpointNatDestination{
			ep:      ep,
			wq:      &wq,
			context: routeContext,
		}
		go destination.loopRead()
		return destination, nil*/
	session := tun.DirectRouteSession{
		Source:      metadata.Source.Addr,
		Destination: metadata.Destination.Addr,
	}
	w.mapping.CreateSession(session, routeContext)
	return &stackNatDestination{
		device:  w,
		session: session,
	}, nil
}

type stackNatDestination struct {
	device  *stackDevice
	session tun.DirectRouteSession
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

/*type endpointNatDestination struct {
	ep           tcpip.Endpoint
	wq           *waiter.Queue
	networkProto tcpip.NetworkProtocolNumber
	context      tun.DirectRouteContext
	done         chan struct{}
}

func (d *endpointNatDestination) loopRead() {
	for {
		println("start read")
		buffer, err := commonRead(d.ep, d.wq, d.done)
		if err != nil {
			log.Error(err)
			return
		}
		println("done read")
		ipHdr := header.IPv4(buffer.Bytes())
		if ipHdr.TransportProtocol() != header.ICMPv4ProtocolNumber {
			buffer.Release()
			continue
		}
		icmpHdr := header.ICMPv4(ipHdr.Payload())
		if icmpHdr.Type() != header.ICMPv4EchoReply {
			buffer.Release()
			continue
		}
		fmt.Println("read echo reply")
		_ = d.context.WritePacket(ipHdr)
		buffer.Release()
	}
}

func commonRead(ep tcpip.Endpoint, wq *waiter.Queue, done chan struct{}) (*buf.Buffer, error) {
	buffer := buf.NewPacket()
	result, err := ep.Read(buffer, tcpip.ReadOptions{})
	if err != nil {
		if _, ok := err.(*tcpip.ErrWouldBlock); ok {
			waitEntry, notifyCh := waiter.NewChannelEntry(waiter.ReadableEvents)
			wq.EventRegister(&waitEntry)
			defer wq.EventUnregister(&waitEntry)
			for {
				result, err = ep.Read(buffer, tcpip.ReadOptions{})
				if _, ok := err.(*tcpip.ErrWouldBlock); !ok {
					break
				}
				select {
				case <-notifyCh:
				case <-done:
					buffer.Release()
					return nil, context.DeadlineExceeded
				}
			}
		}
		return nil, gonet.TranslateNetstackError(err)
	}
	buffer.Truncate(result.Count)
	return buffer, nil
}

func (d *endpointNatDestination) WritePacket(buffer *buf.Buffer) error {
	_, err := d.ep.Write(buffer, tcpip.WriteOptions{})
	if err != nil {
		return gonet.TranslateNetstackError(err)
	}
	return nil
}

func (d *endpointNatDestination) WritePacketBuffer(buffer *stack.PacketBuffer) error {
	data := buffer.ToView().AsSlice()
	println("write echo request buffer :" + fmt.Sprint(data))
	_, err := d.ep.Write(bytes.NewReader(data), tcpip.WriteOptions{})
	if err != nil {
		log.Error(err)
		return gonet.TranslateNetstackError(err)
	}
	return nil
}

func (d *endpointNatDestination) Close() error {
	d.ep.Abort()
	close(d.done)
	return nil
}

func (d *endpointNatDestination) Timeout() bool {
	return false
}
*/
