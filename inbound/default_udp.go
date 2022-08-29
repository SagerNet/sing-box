package inbound

import (
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func (a *myInboundAdapter) ListenUDP() (net.PacketConn, error) {
	bindAddr := M.SocksaddrFrom(netip.Addr(a.listenOptions.Listen), a.listenOptions.ListenPort)
	udpConn, err := net.ListenUDP(M.NetworkFromNetAddr(N.NetworkUDP, bindAddr.Addr), bindAddr.UDPAddr())
	if err != nil {
		return nil, err
	}
	a.udpConn = udpConn
	a.udpAddr = bindAddr
	a.logger.Info("udp server started at ", udpConn.LocalAddr())
	return udpConn, err
}

func (a *myInboundAdapter) loopUDPIn() {
	defer close(a.packetOutboundClosed)
	_buffer := buf.StackNewPacket()
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	buffer.IncRef()
	defer buffer.DecRef()
	packetService := (*myInboundPacketAdapter)(a)
	for {
		buffer.Reset()
		n, addr, err := a.udpConn.ReadFromUDPAddrPort(buffer.FreeBytes())
		if err != nil {
			return
		}
		buffer.Truncate(n)
		var metadata adapter.InboundContext
		metadata.Inbound = a.tag
		metadata.InboundType = a.protocol
		metadata.SniffEnabled = a.listenOptions.SniffEnabled
		metadata.SniffOverrideDestination = a.listenOptions.SniffOverrideDestination
		metadata.DomainStrategy = dns.DomainStrategy(a.listenOptions.DomainStrategy)
		metadata.Source = M.SocksaddrFromNetIP(addr)
		metadata.OriginDestination = a.udpAddr
		err = a.packetHandler.NewPacket(a.ctx, packetService, buffer, metadata)
		if err != nil {
			a.newError(E.Cause(err, "process packet from ", metadata.Source))
		}
	}
}

func (a *myInboundAdapter) loopUDPOOBIn() {
	defer close(a.packetOutboundClosed)
	_buffer := buf.StackNewPacket()
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	buffer.IncRef()
	defer buffer.DecRef()
	packetService := (*myInboundPacketAdapter)(a)
	oob := make([]byte, 1024)
	for {
		buffer.Reset()
		n, oobN, _, addr, err := a.udpConn.ReadMsgUDPAddrPort(buffer.FreeBytes(), oob)
		if err != nil {
			return
		}
		buffer.Truncate(n)
		var metadata adapter.InboundContext
		metadata.Inbound = a.tag
		metadata.InboundType = a.protocol
		metadata.SniffEnabled = a.listenOptions.SniffEnabled
		metadata.SniffOverrideDestination = a.listenOptions.SniffOverrideDestination
		metadata.DomainStrategy = dns.DomainStrategy(a.listenOptions.DomainStrategy)
		metadata.Source = M.SocksaddrFromNetIP(addr)
		metadata.OriginDestination = a.udpAddr
		err = a.oobPacketHandler.NewPacket(a.ctx, packetService, buffer, oob[:oobN], metadata)
		if err != nil {
			a.newError(E.Cause(err, "process packet from ", metadata.Source))
		}
	}
}

func (a *myInboundAdapter) loopUDPInThreadSafe() {
	defer close(a.packetOutboundClosed)
	packetService := (*myInboundPacketAdapter)(a)
	for {
		buffer := buf.NewPacket()
		n, addr, err := a.udpConn.ReadFromUDPAddrPort(buffer.FreeBytes())
		if err != nil {
			buffer.Release()
			return
		}
		buffer.Truncate(n)
		var metadata adapter.InboundContext
		metadata.Inbound = a.tag
		metadata.InboundType = a.protocol
		metadata.SniffEnabled = a.listenOptions.SniffEnabled
		metadata.SniffOverrideDestination = a.listenOptions.SniffOverrideDestination
		metadata.DomainStrategy = dns.DomainStrategy(a.listenOptions.DomainStrategy)
		metadata.Source = M.SocksaddrFromNetIP(addr)
		metadata.OriginDestination = a.udpAddr
		err = a.packetHandler.NewPacket(a.ctx, packetService, buffer, metadata)
		if err != nil {
			buffer.Release()
			a.newError(E.Cause(err, "process packet from ", metadata.Source))
		}
	}
}

func (a *myInboundAdapter) loopUDPOOBInThreadSafe() {
	defer close(a.packetOutboundClosed)
	packetService := (*myInboundPacketAdapter)(a)
	oob := make([]byte, 1024)
	for {
		buffer := buf.NewPacket()
		n, oobN, _, addr, err := a.udpConn.ReadMsgUDPAddrPort(buffer.FreeBytes(), oob)
		if err != nil {
			buffer.Release()
			return
		}
		buffer.Truncate(n)
		var metadata adapter.InboundContext
		metadata.Inbound = a.tag
		metadata.InboundType = a.protocol
		metadata.SniffEnabled = a.listenOptions.SniffEnabled
		metadata.SniffOverrideDestination = a.listenOptions.SniffOverrideDestination
		metadata.DomainStrategy = dns.DomainStrategy(a.listenOptions.DomainStrategy)
		metadata.Source = M.SocksaddrFromNetIP(addr)
		metadata.OriginDestination = a.udpAddr
		err = a.oobPacketHandler.NewPacket(a.ctx, packetService, buffer, oob[:oobN], metadata)
		if err != nil {
			buffer.Release()
			a.newError(E.Cause(err, "process packet from ", metadata.Source))
		}
	}
}

func (a *myInboundAdapter) loopUDPOut() {
	for {
		select {
		case packet := <-a.packetOutbound:
			err := a.writePacket(packet.buffer, packet.destination)
			if err != nil && !E.IsClosed(err) {
				a.newError(E.New("write back udp: ", err))
			}
			continue
		case <-a.packetOutboundClosed:
		}
		for {
			select {
			case packet := <-a.packetOutbound:
				packet.buffer.Release()
			default:
				return
			}
		}
	}
}

func (a *myInboundAdapter) writePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	if destination.IsFqdn() {
		udpAddr, err := net.ResolveUDPAddr(N.NetworkUDP, destination.String())
		if err != nil {
			return err
		}
		return common.Error(a.udpConn.WriteTo(buffer.Bytes(), udpAddr))
	}
	return common.Error(a.udpConn.WriteToUDPAddrPort(buffer.Bytes(), destination.AddrPort()))
}

type myInboundPacketAdapter myInboundAdapter

func (s *myInboundPacketAdapter) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	n, addr, err := s.udpConn.ReadFromUDPAddrPort(buffer.FreeBytes())
	if err != nil {
		return M.Socksaddr{}, err
	}
	buffer.Truncate(n)
	return M.SocksaddrFromNetIP(addr), nil
}

func (s *myInboundPacketAdapter) WriteIsThreadUnsafe() {
}

type myInboundPacket struct {
	buffer      *buf.Buffer
	destination M.Socksaddr
}

func (s *myInboundPacketAdapter) Upstream() any {
	return s.udpConn
}

func (s *myInboundPacketAdapter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	s.packetAccess.RLock()
	defer s.packetAccess.RUnlock()

	select {
	case <-s.packetOutboundClosed:
		return os.ErrClosed
	default:
	}

	s.packetOutbound <- &myInboundPacket{buffer, destination}
	return nil
}

func (s *myInboundPacketAdapter) Close() error {
	return s.udpConn.Close()
}

func (s *myInboundPacketAdapter) LocalAddr() net.Addr {
	return s.udpConn.LocalAddr()
}

func (s *myInboundPacketAdapter) SetDeadline(t time.Time) error {
	return s.udpConn.SetDeadline(t)
}

func (s *myInboundPacketAdapter) SetReadDeadline(t time.Time) error {
	return s.udpConn.SetReadDeadline(t)
}

func (s *myInboundPacketAdapter) SetWriteDeadline(t time.Time) error {
	return s.udpConn.SetWriteDeadline(t)
}
