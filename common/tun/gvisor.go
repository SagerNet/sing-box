package tun

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const defaultNIC tcpip.NICID = 1

var _ adapter.Service = (*GVisorTun)(nil)

type GVisorTun struct {
	ctx     context.Context
	tunFd   uintptr
	tunMtu  uint32
	handler Handler
	stack   *stack.Stack
}

func NewGVisor(ctx context.Context, tunFd uintptr, tunMtu uint32, handler Handler) *GVisorTun {
	return &GVisorTun{
		ctx:     ctx,
		tunFd:   tunFd,
		tunMtu:  tunMtu,
		handler: handler,
	}
}

func (t *GVisorTun) Start() error {
	linkEndpoint, err := NewEndpoint(t.tunFd, t.tunMtu)
	if err != nil {
		return err
	}
	ipStack := stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
			icmp.NewProtocol6,
		},
	})
	tErr := ipStack.CreateNIC(defaultNIC, linkEndpoint)
	if tErr != nil {
		return E.New("create nic: ", tErr)
	}
	ipStack.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: defaultNIC},
		{Destination: header.IPv6EmptySubnet, NIC: defaultNIC},
	})
	ipStack.SetSpoofing(defaultNIC, true)
	ipStack.SetPromiscuousMode(defaultNIC, true)
	bufSize := 20 * 1024
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpip.TCPReceiveBufferSizeRangeOption{
		Min:     1,
		Default: bufSize,
		Max:     bufSize,
	})
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpip.TCPSendBufferSizeRangeOption{
		Min:     1,
		Default: bufSize,
		Max:     bufSize,
	})
	sOpt := tcpip.TCPSACKEnabled(true)
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &sOpt)
	mOpt := tcpip.TCPModerateReceiveBufferOption(true)
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &mOpt)
	tcpForwarder := tcp.NewForwarder(ipStack, 0, 1024, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		endpoint, err := r.CreateEndpoint(&wq)
		if err != nil {
			r.Complete(true)
			return
		}
		r.Complete(false)
		endpoint.SocketOptions().SetKeepAlive(true)
		tcpConn := gonet.NewTCPConn(&wq, endpoint)
		lAddr := tcpConn.RemoteAddr()
		rAddr := tcpConn.LocalAddr()
		if lAddr == nil || rAddr == nil {
			tcpConn.Close()
			return
		}
		go func() {
			var metadata M.Metadata
			metadata.Source = M.SocksaddrFromNet(lAddr)
			metadata.Destination = M.SocksaddrFromNet(rAddr)
			ctx := log.ContextWithID(t.ctx)
			hErr := t.handler.NewConnection(ctx, tcpConn, metadata)
			if hErr != nil {
				t.handler.NewError(ctx, hErr)
			}
		}()
	})
	ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber, func(id stack.TransportEndpointID, buffer *stack.PacketBuffer) bool {
		return tcpForwarder.HandlePacket(id, buffer)
	})
	udpForwarder := udp.NewForwarder(ipStack, func(request *udp.ForwarderRequest) {
		var wq waiter.Queue
		endpoint, err := request.CreateEndpoint(&wq)
		if err != nil {
			return
		}
		udpConn := gonet.NewUDPConn(ipStack, &wq, endpoint)
		lAddr := udpConn.RemoteAddr()
		rAddr := udpConn.LocalAddr()
		if lAddr == nil || rAddr == nil {
			udpConn.Close()
			return
		}
		go func() {
			var metadata M.Metadata
			metadata.Source = M.SocksaddrFromNet(lAddr)
			metadata.Destination = M.SocksaddrFromNet(rAddr)
			ctx := log.ContextWithID(t.ctx)
			hErr := t.handler.NewPacketConnection(ctx, bufio.NewPacketConn(&bufio.UnbindPacketConn{ExtendedConn: bufio.NewExtendedConn(udpConn), Addr: M.SocksaddrFromNet(rAddr)}), metadata)
			if hErr != nil {
				t.handler.NewError(ctx, hErr)
			}
		}()
	})
	ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)
	t.stack = ipStack
	return nil
}

func (t *GVisorTun) Close() error {
	return common.Close(
		common.PtrOrNil(t.stack),
	)
}
