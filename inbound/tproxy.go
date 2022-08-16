package inbound

import (
	"context"
	"net"
	"net/netip"

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
	var udpTimeout int64
	if options.UDPTimeout != 0 {
		udpTimeout = options.UDPTimeout
	} else {
		udpTimeout = int64(C.UDPTimeout.Seconds())
	}
	tproxy.connHandler = tproxy
	tproxy.oobPacketHandler = tproxy
	tproxy.udpNat = udpnat.New[netip.AddrPort](udpTimeout, tproxy.upstreamContextHandler())
	tproxy.packetUpstream = tproxy.udpNat
	return tproxy
}

func (t *TProxy) Start() error {
	err := t.myInboundAdapter.Start()
	if err != nil {
		return err
	}
	if t.tcpListener != nil {
		err = control.Conn(t.tcpListener, func(fd uintptr) error {
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
	metadata.Destination = M.SocksaddrFromNet(conn.LocalAddr())
	return t.newConnection(ctx, conn, metadata)
}

func (t *TProxy) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, oob []byte, metadata adapter.InboundContext) error {
	destination, err := redir.GetOriginalDestinationFromOOB(oob)
	if err != nil {
		return E.Cause(err, "get tproxy destination")
	}
	metadata.Destination = M.SocksaddrFromNetIP(destination)
	t.udpNat.NewContextPacket(ctx, metadata.Source.AddrPort(), buffer, adapter.UpstreamMetadata(metadata), func(natConn N.PacketConn) (context.Context, N.PacketWriter) {
		return adapter.WithContext(log.ContextWithNewID(ctx), &metadata), &tproxyPacketWriter{source: natConn}
	})
	return nil
}

type tproxyPacketWriter struct {
	source      N.PacketConn
	destination M.Socksaddr
	conn        *net.UDPConn
}

func (w *tproxyPacketWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	var udpConn *net.UDPConn
	if w.destination == destination {
		if w.conn != nil {
			udpConn = w.conn
		}
	}
	if udpConn == nil {
		var err error
		udpConn, err = redir.DialUDP(destination.UDPAddr(), M.SocksaddrFromNet(w.source.LocalAddr()).UDPAddr())
		if err != nil {
			return E.Cause(err, "tproxy udp write back")
		}
		if w.destination == destination {
			w.conn = udpConn
		} else {
			defer udpConn.Close()
		}
	}
	return common.Error(udpConn.Write(buffer.Bytes()))
}

func (w *tproxyPacketWriter) Close() error {
	return common.Close(common.PtrOrNil(w.conn))
}
