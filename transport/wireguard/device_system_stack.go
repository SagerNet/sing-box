//go:build with_gvisor

package wireguard

import (
	"net/netip"
	"sync"

	"github.com/sagernet/gvisor/pkg/buffer"
	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/header"
	"github.com/sagernet/gvisor/pkg/tcpip/network/ipv4"
	"github.com/sagernet/gvisor/pkg/tcpip/network/ipv6"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/icmp"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/tcp"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/udp"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/wireguard-go/device"
)

var _ Device = (*systemStackDevice)(nil)

type systemStackDevice struct {
	*systemDevice
	stack         *stack.Stack
	endpoint      *deviceEndpoint
	icmpForwarder *tun.ICMPForwarder
	writeBufs     [][]byte
	closeOnce     sync.Once
}

func newSystemStackDevice(options DeviceOptions) (*systemStackDevice, error) {
	system, err := newSystemDevice(options)
	if err != nil {
		return nil, err
	}
	endpoint := &deviceEndpoint{
		mtu:  options.MTU,
		done: make(chan struct{}),
	}
	ipStack, err := tun.NewGVisorStackWithOptions(endpoint, stack.NICOptions{}, true)
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
			protoAddr.Protocol = ipv4.ProtocolNumber
		} else {
			protoAddr.Protocol = ipv6.ProtocolNumber
		}
		gErr := ipStack.AddProtocolAddress(tun.DefaultNIC, protoAddr, stack.AddressProperties{})
		if gErr != nil {
			return nil, E.New("parse local address ", protoAddr.AddressWithPrefix, ": ", gErr.String())
		}
	}
	stackDevice := &systemStackDevice{
		systemDevice: system,
		stack:        ipStack,
		endpoint:     endpoint,
	}
	if options.Handler != nil {
		ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tun.NewTCPForwarder(options.Context, ipStack, options.Handler).HandlePacket)
		ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, tun.NewUDPForwarder(options.Context, ipStack, options.Handler, options.UDPTimeout).HandlePacket)
		icmpForwarder := tun.NewICMPForwarder(ipStack, options.Handler, options.Logger)
		ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber4, icmpForwarder.HandlePacket)
		ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber6, icmpForwarder.HandlePacket)
		stackDevice.icmpForwarder = icmpForwarder
	}
	return stackDevice, nil
}

func (w *systemStackDevice) SetDevice(device *device.Device) {
	w.endpoint.device = device
}

func (w *systemStackDevice) Write(bufs [][]byte, offset int) (count int, err error) {
	if w.batchDevice != nil {
		w.writeBufs = w.writeBufs[:0]
		for _, packet := range bufs {
			if !w.writeStack(packet[offset:]) {
				w.writeBufs = append(w.writeBufs, packet)
			}
		}
		if len(w.writeBufs) > 0 {
			return w.batchDevice.BatchWrite(w.writeBufs, offset)
		}
	} else {
		for _, packet := range bufs {
			if !w.writeStack(packet[offset:]) {
				if tun.PacketOffset > 0 {
					clear(packet[offset-tun.PacketOffset : offset])
					tun.PacketFillHeader(packet[offset-tun.PacketOffset:], tun.PacketIPVersion(packet[offset:]))
				}
				_, err = w.device.Write(packet[offset-tun.PacketOffset:])
			}
			if err != nil {
				return
			}
		}
	}
	// WireGuard will not read count
	return
}

func (w *systemStackDevice) Close() error {
	var err error
	w.closeOnce.Do(func() {
		close(w.endpoint.done)
		if w.icmpForwarder != nil {
			w.icmpForwarder.Close()
		}
		w.stack.Close()
		for _, endpoint := range w.stack.CleanupEndpoints() {
			endpoint.Abort()
		}
		w.stack.Wait()
		err = w.systemDevice.Close()
	})
	return err
}

func (w *systemStackDevice) writeStack(packet []byte) bool {
	var (
		networkProtocol tcpip.NetworkProtocolNumber
		destination     netip.Addr
	)
	switch header.IPVersion(packet) {
	case header.IPv4Version:
		networkProtocol = header.IPv4ProtocolNumber
		destination = netip.AddrFrom4(header.IPv4(packet).DestinationAddress().As4())
	case header.IPv6Version:
		networkProtocol = header.IPv6ProtocolNumber
		destination = netip.AddrFrom16(header.IPv6(packet).DestinationAddress().As16())
	}
	for _, prefix := range w.options.Address {
		if prefix.Contains(destination) {
			return false
		}
	}
	packetBuffer := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(packet),
	})
	w.endpoint.dispatcher.DeliverNetworkPacket(networkProtocol, packetBuffer)
	packetBuffer.DecRef()
	return true
}

type deviceEndpoint struct {
	mtu        uint32
	done       chan struct{}
	device     *device.Device
	dispatcher stack.NetworkDispatcher
}

func (ep *deviceEndpoint) MTU() uint32 {
	return ep.mtu
}

func (ep *deviceEndpoint) SetMTU(mtu uint32) {
}

func (ep *deviceEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (ep *deviceEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (ep *deviceEndpoint) SetLinkAddress(addr tcpip.LinkAddress) {
}

func (ep *deviceEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityRXChecksumOffload
}

func (ep *deviceEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	ep.dispatcher = dispatcher
}

func (ep *deviceEndpoint) IsAttached() bool {
	return ep.dispatcher != nil
}

func (ep *deviceEndpoint) Wait() {
}

func (ep *deviceEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (ep *deviceEndpoint) AddHeader(buffer *stack.PacketBuffer) {
}

func (ep *deviceEndpoint) ParseHeader(ptr *stack.PacketBuffer) bool {
	return true
}

func (ep *deviceEndpoint) WritePackets(list stack.PacketBufferList) (int, tcpip.Error) {
	for _, packetBuffer := range list.AsSlice() {
		destination := packetBuffer.Network().DestinationAddress()
		ep.device.InputPacket(destination.AsSlice(), packetBuffer.AsSlices())
	}
	return list.Len(), nil
}

func (ep *deviceEndpoint) Close() {
}

func (ep *deviceEndpoint) SetOnCloseAction(f func()) {
}
