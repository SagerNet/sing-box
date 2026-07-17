//go:build with_gvisor

package openvpn

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

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
)

var _ Device = (*stackDevice)(nil)

type stackDevice struct {
	baseDevice
	stateAccess     sync.RWMutex
	options         DeviceOptions
	stack           *stack.Stack
	endpoint        *stackEndpoint
	inet4Address    netip.Addr
	inet6Address    netip.Addr
	icmpForwarder   *tun.ICMPForwarder
	logRouteOptions bool
	closeOnce       sync.Once
}

func newStackDevice(options DeviceOptions) (*stackDevice, error) {
	if options.MTU == 0 {
		options.MTU = DefaultMTU
	}
	device := &stackDevice{
		options:         options,
		logRouteOptions: true,
	}
	endpoint := &stackEndpoint{
		device: device,
		done:   make(chan struct{}),
	}
	endpoint.mtu.Store(options.MTU)
	ipStack, err := tun.NewGVisorStackWithOptions(endpoint, stack.NICOptions{}, true)
	if err != nil {
		return nil, err
	}
	device.stack = ipStack
	device.endpoint = endpoint
	err = device.updateAddresses(nil, options.Configuration.Address)
	if err != nil {
		ipStack.Close()
		return nil, err
	}
	if options.Handler != nil {
		ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tun.NewTCPForwarder(options.Context, ipStack, options.Handler).HandlePacket)
		ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, tun.NewUDPForwarder(options.Context, ipStack, options.Handler, tun.UDPNatOptions{
			Timeout: options.UDPTimeout,
		}).HandlePacket)
		icmpForwarder := tun.NewICMPForwarder(ipStack, options.Handler, options.Logger)
		ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber4, icmpForwarder.HandlePacket)
		ipStack.SetTransportProtocolHandler(icmp.ProtocolNumber6, icmpForwarder.HandlePacket)
		device.icmpForwarder = icmpForwarder
	}
	return device, nil
}

func (d *stackDevice) Start() error {
	return nil
}

func (d *stackDevice) UpdateConfiguration(configuration Configuration) error {
	d.stateAccess.Lock()
	defer d.stateAccess.Unlock()
	if d.logRouteOptions && hasRouteOptions(configuration.Routes) {
		d.options.Logger.Debug("OpenVPN route gateway and metric options are not representable by the gVisor stack device; routes are installed by prefix")
	}
	if configuration.MTU != 0 {
		d.options.MTU = configuration.MTU
		d.endpoint.mtu.Store(configuration.MTU)
	}
	previousAddresses := d.options.Configuration.Address
	d.options.Configuration = configuration
	return d.updateAddresses(previousAddresses, configuration.Address)
}

func (d *stackDevice) updateAddresses(previousAddresses []netip.Prefix, addresses []netip.Prefix) error {
	for _, prefix := range previousAddresses {
		if slices.Contains(addresses, prefix) {
			continue
		}
		gErr := d.stack.RemoveAddress(tun.DefaultNIC, tun.AddressFromAddr(prefix.Addr()))
		if gErr != nil {
			return E.New("remove local address ", prefix, ": ", gErr.String())
		}
	}
	for _, prefix := range addresses {
		if slices.Contains(previousAddresses, prefix) {
			continue
		}
		protocolAddress := tcpip.ProtocolAddress{
			AddressWithPrefix: tcpip.AddressWithPrefix{
				Address:   tun.AddressFromAddr(prefix.Addr()),
				PrefixLen: prefix.Bits(),
			},
		}
		if prefix.Addr().Is4() {
			protocolAddress.Protocol = ipv4.ProtocolNumber
		} else {
			protocolAddress.Protocol = ipv6.ProtocolNumber
		}
		gErr := d.stack.AddProtocolAddress(tun.DefaultNIC, protocolAddress, stack.AddressProperties{})
		if gErr != nil {
			return E.New("add local address ", prefix, ": ", gErr.String())
		}
	}
	d.inet4Address, d.inet6Address = firstAddresses(addresses)
	return nil
}

func (d *stackDevice) WriteInboundBuffers(packetBuffers []*buf.Buffer) error {
	return d.processInboundBuffers(packetBuffers, d.writeBuffers)
}

func (d *stackDevice) writeBuffers(packetBuffers []*buf.Buffer) error {
	networkProtocols := make([]tcpip.NetworkProtocolNumber, 0, len(packetBuffers))
	stackPacketBuffers := make([]*stack.PacketBuffer, 0, len(packetBuffers))
	var packetErr error
	for _, packetBuffer := range packetBuffers {
		packet := packetBuffer.Bytes()
		var networkProtocol tcpip.NetworkProtocolNumber
		switch header.IPVersion(packet) {
		case header.IPv4Version:
			networkProtocol = header.IPv4ProtocolNumber
		case header.IPv6Version:
			networkProtocol = header.IPv6ProtocolNumber
		default:
			if packetErr == nil {
				packetErr = E.New("invalid IP packet")
			}
			continue
		}
		networkProtocols = append(networkProtocols, networkProtocol)
		stackPacketBuffers = append(stackPacketBuffers, stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(packet),
		}))
	}
	d.endpoint.deliverNetworkPackets(networkProtocols, stackPacketBuffers)
	for _, packetBuffer := range stackPacketBuffers {
		packetBuffer.DecRef()
	}
	return packetErr
}

func (d *stackDevice) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if destination.IsIPv6() && d.blockIPv6Enabled() {
		return nil, E.New("IPv6 blocked by pushed OpenVPN block-ipv6")
	}
	inet4Address, inet6Address := d.PortAddresses()
	address := tcpip.FullAddress{
		NIC:  tun.DefaultNIC,
		Port: destination.Port,
		Addr: tun.AddressFromAddr(destination.Addr),
	}
	bind := tcpip.FullAddress{
		NIC: tun.DefaultNIC,
	}
	var networkProtocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		if !inet4Address.IsValid() {
			return nil, E.New("missing IPv4 local address")
		}
		networkProtocol = header.IPv4ProtocolNumber
		bind.Addr = tun.AddressFromAddr(inet4Address)
	} else {
		if !inet6Address.IsValid() {
			return nil, E.New("missing IPv6 local address")
		}
		networkProtocol = header.IPv6ProtocolNumber
		bind.Addr = tun.AddressFromAddr(inet6Address)
	}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		return gonet.DialTCPWithBind(ctx, d.stack, bind, address, networkProtocol)
	case N.NetworkUDP:
		return gonet.DialUDP(d.stack, &bind, &address, networkProtocol)
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (d *stackDevice) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if destination.IsIPv6() && d.blockIPv6Enabled() {
		return nil, E.New("IPv6 blocked by pushed OpenVPN block-ipv6")
	}
	inet4Address, inet6Address := d.PortAddresses()
	bind := tcpip.FullAddress{
		NIC: tun.DefaultNIC,
	}
	var networkProtocol tcpip.NetworkProtocolNumber
	if destination.IsIPv4() {
		if !inet4Address.IsValid() {
			return nil, E.New("missing IPv4 local address")
		}
		networkProtocol = header.IPv4ProtocolNumber
		bind.Addr = tun.AddressFromAddr(inet4Address)
	} else {
		if !inet6Address.IsValid() {
			return nil, E.New("missing IPv6 local address")
		}
		networkProtocol = header.IPv6ProtocolNumber
		bind.Addr = tun.AddressFromAddr(inet6Address)
	}
	return gonet.DialUDP(d.stack, &bind, nil, networkProtocol)
}

func (d *stackDevice) blockIPv6Enabled() bool {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return d.options.Configuration.BlockIPv6
}

func (d *stackDevice) PortAddresses() (netip.Addr, netip.Addr) {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return d.inet4Address, d.inet6Address
}

func (d *stackDevice) PortMTU() uint32 {
	d.stateAccess.RLock()
	defer d.stateAccess.RUnlock()
	return d.options.MTU
}

func (d *stackDevice) Close() error {
	d.closeOnce.Do(func() {
		close(d.endpoint.done)
		if d.icmpForwarder != nil {
			d.icmpForwarder.Close()
		}
		d.stack.Close()
		for _, endpoint := range d.stack.CleanupEndpoints() {
			endpoint.Abort()
		}
		d.stack.Wait()
	})
	return nil
}

type stackEndpoint struct {
	device           *stackDevice
	mtu              atomic.Uint32
	done             chan struct{}
	dispatcherAccess sync.RWMutex
	dispatcher       stack.NetworkDispatcher
}

func (e *stackEndpoint) MTU() uint32 {
	return e.mtu.Load()
}

func (e *stackEndpoint) SetMTU(mtu uint32) {
	e.mtu.Store(mtu)
}

func (e *stackEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (e *stackEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (e *stackEndpoint) SetLinkAddress(addr tcpip.LinkAddress) {
}

func (e *stackEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityRXChecksumOffload
}

func (e *stackEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.dispatcherAccess.Lock()
	defer e.dispatcherAccess.Unlock()
	e.dispatcher = dispatcher
}

func (e *stackEndpoint) IsAttached() bool {
	e.dispatcherAccess.RLock()
	defer e.dispatcherAccess.RUnlock()
	return e.dispatcher != nil
}

func (e *stackEndpoint) deliverNetworkPackets(networkProtocols []tcpip.NetworkProtocolNumber, packetBuffers []*stack.PacketBuffer) {
	e.dispatcherAccess.RLock()
	defer e.dispatcherAccess.RUnlock()
	if e.dispatcher == nil {
		return
	}
	for i, packetBuffer := range packetBuffers {
		e.dispatcher.DeliverNetworkPacket(networkProtocols[i], packetBuffer)
	}
}

func (e *stackEndpoint) Wait() {
}

func (e *stackEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (e *stackEndpoint) AddHeader(packetBuffer *stack.PacketBuffer) {
}

func (e *stackEndpoint) ParseHeader(packetBuffer *stack.PacketBuffer) bool {
	return true
}

func (e *stackEndpoint) WritePackets(list stack.PacketBufferList) (int, tcpip.Error) {
	packetBuffers := make([]*buf.Buffer, 0, list.Len())
	for _, packetBuffer := range list.AsSlice() {
		packetSlices := packetBuffer.AsSlices()
		packetLength := 0
		for _, packetSlice := range packetSlices {
			packetLength += len(packetSlice)
		}
		outboundBuffer := buf.NewSize(PacketHeadroom + packetLength + systemDevicePacketRearSpace)
		outboundBuffer.Resize(PacketHeadroom, 0)
		for _, packetSlice := range packetSlices {
			_, _ = outboundBuffer.Write(packetSlice)
		}
		packetBuffers = append(packetBuffers, outboundBuffer)
	}
	err := e.device.writeOutbound(packetBuffers)
	if err != nil {
		return 0, &tcpip.ErrClosedForSend{}
	}
	return list.Len(), nil
}

func (e *stackEndpoint) Close() {
}

func (e *stackEndpoint) SetOnCloseAction(action func()) {
}
