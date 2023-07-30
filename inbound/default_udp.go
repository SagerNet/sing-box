package inbound

import (
	"net"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/unix"
)

func (a *myInboundAdapter) ListenUDP() (net.PacketConn, error) {
	var bindAddr M.Socksaddr
	var udpFragment bool
	if a.listenOptions.UDPFragment != nil {
		udpFragment = *a.listenOptions.UDPFragment
	} else {
		udpFragment = a.listenOptions.UDPFragmentDefault
	}
	var udpConn net.PacketConn
	var err error
	a.initFds()
	if a.udpFd != 0 {
		if !udpFragment {
			if err := unix.SetsockoptInt(a.udpFd, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO); err != nil {
				a.logger.Warn("SETSOCKOPT IP_MTU_DISCOVER IP_PMTUDISC_DO: ", err)
			}
			sa, err := unix.Getsockname(a.udpFd)
			if err == nil {
				if _, isIpv6 := sa.(*unix.SockaddrInet6); isIpv6 {
					if err := unix.SetsockoptInt(a.udpFd, unix.IPPROTO_IPV6, unix.IPV6_MTU_DISCOVER, unix.IP_PMTUDISC_DO); err != nil {
						a.logger.Warn("SETSOCKOPT IPV6_MTU_DISCOVER IP_PMTUDISC_DO", err)
					}
				}
			} else {
				a.logger.Error("fd:", a.udpFd, " get addr error: ", err)
			}
		}
		f := os.NewFile(uintptr(a.udpFd), "")
		udpConn, err = net.FilePacketConn(f)
		f.Close()
		bindAddr = M.SocksaddrFromNet(udpConn.LocalAddr())
	} else {
		var lc net.ListenConfig
		bindAddr = M.SocksaddrFrom(a.listenOptions.Listen.Build(), a.listenOptions.ListenPort)
		if !udpFragment {
			lc.Control = control.Append(lc.Control, control.DisableUDPFragment())
		}
		udpConn, err = lc.ListenPacket(a.ctx, M.NetworkFromNetAddr(N.NetworkUDP, bindAddr.Addr), bindAddr.String())
	}
	if err != nil {
		return nil, err
	}
	a.udpConn = udpConn.(*net.UDPConn)
	a.udpAddr = bindAddr
	a.logger.Info("udp server started at ", udpConn.LocalAddr())
	return udpConn, err
}

func (a *myInboundAdapter) loopUDPIn() {
	defer close(a.packetOutboundClosed)
	buffer := buf.NewPacket()
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
		metadata.InboundOptions = a.listenOptions.InboundOptions
		metadata.Source = M.SocksaddrFromNetIP(addr).Unwrap()
		metadata.OriginDestination = a.udpAddr
		err = a.packetHandler.NewPacket(a.ctx, packetService, buffer, metadata)
		if err != nil {
			a.newError(E.Cause(err, "process packet from ", metadata.Source))
		}
	}
}

func (a *myInboundAdapter) loopUDPOOBIn() {
	defer close(a.packetOutboundClosed)
	buffer := buf.NewPacket()
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
		metadata.InboundOptions = a.listenOptions.InboundOptions
		metadata.Source = M.SocksaddrFromNetIP(addr).Unwrap()
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
		metadata.InboundOptions = a.listenOptions.InboundOptions
		metadata.Source = M.SocksaddrFromNetIP(addr).Unwrap()
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
		metadata.InboundOptions = a.listenOptions.InboundOptions
		metadata.Source = M.SocksaddrFromNetIP(addr).Unwrap()
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
	select {
	case s.packetOutbound <- &myInboundPacket{buffer, destination}:
		return nil
	case <-s.packetOutboundClosed:
		return os.ErrClosed
	}
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
