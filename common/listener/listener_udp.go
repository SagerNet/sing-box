package listener

import (
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func (l *Listener) ListenUDP() (net.PacketConn, error) {
	bindAddr := M.SocksaddrFrom(l.listenOptions.Listen.Build(netip.AddrFrom4([4]byte{127, 0, 0, 1})), l.listenOptions.ListenPort)
	var lc net.ListenConfig
	var udpFragment bool
	if l.listenOptions.UDPFragment != nil {
		udpFragment = *l.listenOptions.UDPFragment
	} else {
		udpFragment = l.listenOptions.UDPFragmentDefault
	}
	if !udpFragment {
		lc.Control = control.Append(lc.Control, control.DisableUDPFragment())
	}
	udpConn, err := lc.ListenPacket(l.ctx, M.NetworkFromNetAddr(N.NetworkUDP, bindAddr.Addr), bindAddr.String())
	if err != nil {
		return nil, err
	}
	l.udpConn = udpConn.(*net.UDPConn)
	l.udpAddr = bindAddr
	l.logger.Info("udp server started at ", udpConn.LocalAddr())
	return udpConn, err
}

func (l *Listener) UDPAddr() M.Socksaddr {
	return l.udpAddr
}

func (l *Listener) PacketWriter() N.PacketWriter {
	return (*packetWriter)(l)
}

func (l *Listener) loopUDPIn() {
	defer close(l.packetOutboundClosed)
	var buffer *buf.Buffer
	if !l.threadUnsafePacketWriter {
		buffer = buf.NewPacket()
		defer buffer.Release()
		buffer.IncRef()
		defer buffer.DecRef()
	}
	if l.oobPacketHandler != nil {
		oob := make([]byte, 1024)
		for {
			if l.threadUnsafePacketWriter {
				buffer = buf.NewPacket()
			} else {
				buffer.Reset()
			}
			n, oobN, _, addr, err := l.udpConn.ReadMsgUDPAddrPort(buffer.FreeBytes(), oob)
			if err != nil {
				if l.threadUnsafePacketWriter {
					buffer.Release()
				}
				if l.shutdown.Load() && E.IsClosed(err) {
					return
				}
				l.udpConn.Close()
				l.logger.Error("udp listener closed: ", err)
				return
			}
			buffer.Truncate(n)
			l.oobPacketHandler.NewPacketEx(buffer, oob[:oobN], M.SocksaddrFromNetIP(addr).Unwrap())
		}
	} else {
		for {
			if l.threadUnsafePacketWriter {
				buffer = buf.NewPacket()
			} else {
				buffer.Reset()
			}
			n, addr, err := l.udpConn.ReadFromUDPAddrPort(buffer.FreeBytes())
			if err != nil {
				if l.threadUnsafePacketWriter {
					buffer.Release()
				}
				if l.shutdown.Load() && E.IsClosed(err) {
					return
				}
				l.udpConn.Close()
				l.logger.Error("udp listener closed: ", err)
				return
			}
			buffer.Truncate(n)
			l.packetHandler.NewPacketEx(buffer, M.SocksaddrFromNetIP(addr).Unwrap())
		}
	}
}

func (l *Listener) loopUDPOut() {
	for {
		select {
		case packet := <-l.packetOutbound:
			destination := packet.Destination.AddrPort()
			_, err := l.udpConn.WriteToUDPAddrPort(packet.Buffer.Bytes(), destination)
			packet.Buffer.Release()
			N.PutPacketBuffer(packet)
			if err != nil {
				if l.shutdown.Load() && E.IsClosed(err) {
					return
				}
				l.udpConn.Close()
				l.logger.Error("udp listener write back: ", destination, ": ", err)
				return
			}
			continue
		case <-l.packetOutboundClosed:
		}
		for {
			select {
			case packet := <-l.packetOutbound:
				packet.Buffer.Release()
				N.PutPacketBuffer(packet)
			default:
				return
			}
		}
	}
}

type packetWriter Listener

func (w *packetWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	packet := N.NewPacketBuffer()
	packet.Buffer = buffer
	packet.Destination = destination
	select {
	case w.packetOutbound <- packet:
		return nil
	default:
		buffer.Release()
		N.PutPacketBuffer(packet)
		if w.shutdown.Load() {
			return os.ErrClosed
		}
		w.logger.Trace("dropped packet to ", destination)
		return nil
	}
}

func (w *packetWriter) WriteIsThreadUnsafe() {
}
