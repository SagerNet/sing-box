package inbound

import (
	"context"
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"

	""golang.org/x/sys/unix""

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat"
)

type TProxy struct {
	myInboundAdapter
	udpNat *udpnat.Service[netip.AddrPort]
}

func NewTProxy(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TProxyInboundOptions) *TProxy {
	tproxy := &TProxy{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeTProxy,
			network:       options.Network.Build(),
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}
	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}
	tproxy.connHandler = tproxy
	tproxy.oobPacketHandler = tproxy
	tproxy.udpNat = udpnat.New[netip.AddrPort](int64(udpTimeout.Seconds()), tproxy.upstreamContextHandler())
	tproxy.packetUpstream = tproxy.udpNat
	return tproxy
}

func (t *TProxy) Start() error {
	err := t.myInboundAdapter.Start()
	if err != nil {
		return err
	}
	if t.tcpListener != nil {
		err = control.Conn(common.MustCast[syscall.Conn](t.tcpListener), func(fd uintptr) error {
			return redir.TProxy(fd, M.SocksaddrFromNet(t.tcpListener.Addr()).Addr.Is6())
		})
		if err != nil {
			return E.Cause(err, "configure tproxy TCP listener")
		}
	}
	if t.udpConn != nil {
		err = control.Conn(t.udpConn, func(fd uintptr) error {
			return redir.TProxy(fd, M.SocksaddrFromNet(t.udpConn.LocalAddr()).Addr.Is6())
		})
		if err != nil {
			return E.Cause(err, "configure tproxy UDP listener")
		}
	}
	return nil
}

func (t *TProxy) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	metadata.Destination = M.SocksaddrFromNet(conn.LocalAddr()).Unwrap()
	return t.newConnection(ctx, conn, metadata)
}

func (t *TProxy) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, oob []byte, metadata adapter.InboundContext) error {
	destination, err := redir.GetOriginalDestinationFromOOB(oob)
	if err != nil {
		return E.Cause(err, "get tproxy destination")
	}
	metadata.Destination = M.SocksaddrFromNetIP(destination).Unwrap()
	t.udpNat.NewContextPacket(ctx, metadata.Source.AddrPort(), buffer, adapter.UpstreamMetadata(metadata), func(natConn N.PacketConn) (context.Context, N.PacketWriter) {
		return adapter.WithContext(log.ContextWithNewID(ctx), &metadata), &tproxyPacketWriter{ctx: ctx, source: natConn, destination: metadata.Destination}
	})
	return nil
}

type tproxyPacketWriter struct {
	ctx         context.Context
	source      N.PacketConn
	destination M.Socksaddr
	conn        *net.UDPConn
}

func (w *tproxyPacketWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	conn := w.conn
	if w.destination == destination && conn != nil {
		_, err := conn.WriteToUDPAddrPort(buffer.Bytes(), M.AddrPortFromNet(w.source.LocalAddr()))
		if err != nil {
			w.conn = nil
		}
		return err
	}
	
	var laddr = destination.String()
	localAddr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		return err
	}

	var raddr = w.source.LocalAddr().String()
	remoteAddr, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		return err
	}

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	err = unix.SetsockoptInt(int(fd), unix.SOL_IP, unix.IP_TRANSPARENT, 1)
	if err != nil {
		return err
	}
	err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	if err != nil {
		return err
	}
	err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	if err != nil {
		return err
	}

	sockaddr := &unix.SockaddrInet4{Port: localAddr.Port}
	copy(sockaddr.Addr[:], localAddr.IP.To4())
	err = unix.Bind(fd, sockaddr)
	if err != nil {
		return err
	}

	remoteSockaddr := &unix.SockaddrInet4{Port: remoteAddr.Port}
	copy(remoteSockaddr.Addr[:], remoteAddr.IP.To4())
	err = unix.Connect(fd, remoteSockaddr)
	if err != nil {
		return err
	}

	file := os.NewFile(uintptr(fd), "")
	fileConn, err := net.FileConn(file)
	if err != nil {
		return err
	}
	file.Close()

	var udpConn = fileConn.(*net.UDPConn)
	
	if w.destination == destination {
		w.conn = udpConn
	} else {
		defer udpConn.Close()
	}
	return common.Error(udpConn.WriteToUDPAddrPort(buffer.Bytes(), M.AddrPortFromNet(w.source.LocalAddr())))
}

func (w *tproxyPacketWriter) Close() error {
	return common.Close(common.PtrOrNil(w.conn))
}
