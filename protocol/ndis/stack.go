//go:build windows

package ndis

import (
	"context"
	"net/netip"
	"time"

	"github.com/sagernet/gvisor/pkg/buffer"
	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/header"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/tcp"
	"github.com/sagernet/gvisor/pkg/tcpip/transport/udp"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/debug"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"github.com/wiresock/ndisapi-go"
	"github.com/wiresock/ndisapi-go/driver"
	"go4.org/netipx"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type Stack struct {
	ctx                    context.Context
	logger                 logger.ContextLogger
	network                adapter.NetworkManager
	trackerIn              conntrack.Tracker
	trackerOut             conntrack.Tracker
	api                    *ndisapi.NdisApi
	handler                tun.Handler
	udpTimeout             time.Duration
	filter                 *driver.QueuedPacketFilter
	stack                  *stack.Stack
	endpoint               *ndisEndpoint
	routeAddress           []netip.Prefix
	routeExcludeAddress    []netip.Prefix
	routeAddressSet        []*netipx.IPSet
	routeExcludeAddressSet []*netipx.IPSet
	currentInterface       *control.Interface
}

func (s *Stack) Start() error {
	err := s.start(s.network.InterfaceMonitor().DefaultInterface())
	if err != nil {
		return err
	}
	s.network.InterfaceMonitor().RegisterCallback(s.updateDefaultInterface)
	return nil
}

func (s *Stack) updateDefaultInterface(defaultInterface *control.Interface, flags int) {
	if s.currentInterface.Equals(*defaultInterface) {
		return
	}
	err := s.start(defaultInterface)
	if err != nil {
		s.logger.Error(E.Cause(err, "reconfigure NDIS at: ", defaultInterface.Name))
	}
}

func (s *Stack) start(defaultInterface *control.Interface) error {
	_ = s.Close()
	adapters, err := s.api.GetTcpipBoundAdaptersInfo()
	if err != nil {
		return err
	}
	if defaultInterface != nil {
		for index := 0; index < int(adapters.AdapterCount); index++ {
			name := s.api.ConvertWindows2000AdapterName(string(adapters.AdapterNameList[index][:]))
			if name != defaultInterface.Name {
				continue
			}
			s.filter, err = driver.NewQueuedPacketFilter(s.api, adapters, nil, s.processOut)
			if err != nil {
				return err
			}
			address := tcpip.LinkAddress(adapters.CurrentAddress[index][:])
			mtu := uint32(adapters.MTU[index])
			endpoint := &ndisEndpoint{
				filter:  s.filter,
				mtu:     mtu,
				address: address,
			}
			s.stack, err = tun.NewGVisorStack(endpoint)
			if err != nil {
				s.filter = nil
				return err
			}
			s.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tun.NewTCPForwarder(s.ctx, s.stack, s.handler).HandlePacket)
			s.stack.SetTransportProtocolHandler(udp.ProtocolNumber, tun.NewUDPForwarder(s.ctx, s.stack, s.handler, s.udpTimeout).HandlePacket)
			err = s.filter.StartFilter(index)
			if err != nil {
				s.filter = nil
				s.stack.Close()
				s.stack = nil
				return err
			}
			s.endpoint = endpoint
			s.logger.Info("started at ", defaultInterface.Name)
			break
		}
	}
	s.currentInterface = defaultInterface
	return nil
}

func (s *Stack) Close() error {
	if s.filter != nil {
		s.filter.StopFilter()
		s.filter.Close()
		s.filter = nil
	}
	if s.stack != nil {
		s.stack.Close()
		for _, endpoint := range s.stack.CleanupEndpoints() {
			endpoint.Abort()
		}
		s.stack = nil
	}
	return nil
}

func (s *Stack) processOut(handle ndisapi.Handle, packet *ndisapi.IntermediateBuffer) ndisapi.FilterAction {
	if packet.Length < header.EthernetMinimumSize {
		return ndisapi.FilterActionPass
	}
	if s.endpoint.dispatcher == nil || s.filterPacket(packet.Buffer[:packet.Length]) {
		return ndisapi.FilterActionPass
	}
	packetBuffer := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(packet.Buffer[:packet.Length]),
	})
	_, ok := packetBuffer.LinkHeader().Consume(header.EthernetMinimumSize)
	if !ok {
		packetBuffer.DecRef()
		return ndisapi.FilterActionPass
	}
	ethHdr := header.Ethernet(packetBuffer.LinkHeader().Slice())
	destinationAddress := ethHdr.DestinationAddress()
	if destinationAddress == header.EthernetBroadcastAddress {
		packetBuffer.PktType = tcpip.PacketBroadcast
	} else if header.IsMulticastEthernetAddress(destinationAddress) {
		packetBuffer.PktType = tcpip.PacketMulticast
	} else if destinationAddress == s.endpoint.address {
		packetBuffer.PktType = tcpip.PacketHost
	} else {
		packetBuffer.PktType = tcpip.PacketOtherHost
	}
	s.endpoint.dispatcher.DeliverNetworkPacket(ethHdr.Type(), packetBuffer)
	packetBuffer.DecRef()
	return ndisapi.FilterActionDrop
}

func (s *Stack) filterPacket(packet []byte) bool {
	var ipHdr header.Network
	switch header.IPVersion(packet[header.EthernetMinimumSize:]) {
	case ipv4.Version:
		ipHdr = header.IPv4(packet[header.EthernetMinimumSize:])
	case ipv6.Version:
		ipHdr = header.IPv6(packet[header.EthernetMinimumSize:])
	default:
		return true
	}
	sourceAddr := tun.AddrFromAddress(ipHdr.SourceAddress())
	destinationAddr := tun.AddrFromAddress(ipHdr.DestinationAddress())
	if !destinationAddr.IsGlobalUnicast() {
		return true
	}
	var (
		transportProtocol tcpip.TransportProtocolNumber
		transportHdr      header.Transport
	)
	switch ipHdr.TransportProtocol() {
	case tcp.ProtocolNumber:
		transportProtocol = header.TCPProtocolNumber
		transportHdr = header.TCP(ipHdr.Payload())
	case udp.ProtocolNumber:
		transportProtocol = header.UDPProtocolNumber
		transportHdr = header.UDP(ipHdr.Payload())
	default:
		return false
	}
	source := netip.AddrPortFrom(sourceAddr, transportHdr.SourcePort())
	destination := netip.AddrPortFrom(destinationAddr, transportHdr.DestinationPort())
	if transportProtocol == header.TCPProtocolNumber {
		if s.trackerIn.CheckConn(source, destination) {
			if debug.Enabled {
				s.logger.Trace("fall exists TCP ", source, " ", destination)
			}
			return false
		}
	} else {
		if s.trackerIn.CheckPacketConn(source) {
			if debug.Enabled {
				s.logger.Trace("fall exists UDP ", source, " ", destination)
			}
		}
	}
	if len(s.routeAddress) > 0 {
		var match bool
		for _, route := range s.routeAddress {
			if route.Contains(destinationAddr) {
				match = true
			}
		}
		if !match {
			return true
		}
	}
	if len(s.routeAddressSet) > 0 {
		var match bool
		for _, ipSet := range s.routeAddressSet {
			if ipSet.Contains(destinationAddr) {
				match = true
			}
		}
		if !match {
			return true
		}
	}
	if len(s.routeExcludeAddress) > 0 {
		for _, address := range s.routeExcludeAddress {
			if address.Contains(destinationAddr) {
				return true
			}
		}
	}
	if len(s.routeExcludeAddressSet) > 0 {
		for _, ipSet := range s.routeAddressSet {
			if ipSet.Contains(destinationAddr) {
				return true
			}
		}
	}
	if s.trackerOut.CheckDestination(destination) {
		if debug.Enabled {
			s.logger.Trace("passing pending ", source, " ", destination)
		}
		return true
	}
	if transportProtocol == header.TCPProtocolNumber {
		if s.trackerOut.CheckConn(source, destination) {
			if debug.Enabled {
				s.logger.Trace("passing TCP ", source, " ", destination)
			}
			return true
		}
	} else {
		if s.trackerOut.CheckPacketConn(source) {
			if debug.Enabled {
				s.logger.Trace("passing UDP ", source, " ", destination)
			}
		}
	}
	if debug.Enabled {
		s.logger.Trace("fall ", source, " ", destination)
	}
	return false
}
