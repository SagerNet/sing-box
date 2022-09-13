package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/settings"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = (*myInboundAdapter)(nil)

type myInboundAdapter struct {
	protocol         string
	network          []string
	ctx              context.Context
	router           adapter.Router
	logger           log.ContextLogger
	tag              string
	listenOptions    option.ListenOptions
	connHandler      adapter.ConnectionHandler
	packetHandler    adapter.PacketHandler
	oobPacketHandler adapter.OOBPacketHandler
	packetUpstream   any

	// http mixed

	setSystemProxy   bool
	clearSystemProxy func() error

	// internal

	tcpListener          net.Listener
	udpConn              *net.UDPConn
	udpAddr              M.Socksaddr
	packetOutboundClosed chan struct{}
	packetOutbound       chan *myInboundPacket
}

func (a *myInboundAdapter) Type() string {
	return a.protocol
}

func (a *myInboundAdapter) Tag() string {
	return a.tag
}

func (a *myInboundAdapter) Network() []string {
	return a.network
}

func (a *myInboundAdapter) Start() error {
	var err error
	if common.Contains(a.network, N.NetworkTCP) {
		_, err = a.ListenTCP()
		if err != nil {
			return err
		}
		go a.loopTCPIn()
	}
	if common.Contains(a.network, N.NetworkUDP) {
		_, err = a.ListenUDP()
		if err != nil {
			return err
		}
		a.packetOutboundClosed = make(chan struct{})
		a.packetOutbound = make(chan *myInboundPacket)
		if a.oobPacketHandler != nil {
			if _, threadUnsafeHandler := common.Cast[N.ThreadUnsafeWriter](a.packetUpstream); !threadUnsafeHandler {
				go a.loopUDPOOBIn()
			} else {
				go a.loopUDPOOBInThreadSafe()
			}
		} else {
			if _, threadUnsafeHandler := common.Cast[N.ThreadUnsafeWriter](a.packetUpstream); !threadUnsafeHandler {
				go a.loopUDPIn()
			} else {
				go a.loopUDPInThreadSafe()
			}
			go a.loopUDPOut()
		}
	}
	if a.setSystemProxy {
		a.clearSystemProxy, err = settings.SetSystemProxy(a.router, M.SocksaddrFromNet(a.tcpListener.Addr()).Port, a.protocol == C.TypeMixed)
		if err != nil {
			return E.Cause(err, "set system proxy")
		}
	}
	return nil
}

func (a *myInboundAdapter) Close() error {
	var err error
	if a.clearSystemProxy != nil {
		err = a.clearSystemProxy()
	}
	return E.Errors(err, common.Close(
		a.tcpListener,
		common.PtrOrNil(a.udpConn),
	))
}

func (a *myInboundAdapter) upstreamHandler(metadata adapter.InboundContext) adapter.UpstreamHandlerAdapter {
	return adapter.NewUpstreamHandler(metadata, a.newConnection, a.streamPacketConnection, a)
}

func (a *myInboundAdapter) upstreamContextHandler() adapter.UpstreamHandlerAdapter {
	return adapter.NewUpstreamContextHandler(a.newConnection, a.newPacketConnection, a)
}

func (a *myInboundAdapter) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	a.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	return a.router.RouteConnection(ctx, conn, metadata)
}

func (a *myInboundAdapter) streamPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	a.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	return a.router.RoutePacketConnection(ctx, conn, metadata)
}

func (a *myInboundAdapter) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	a.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	a.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	return a.router.RoutePacketConnection(ctx, conn, metadata)
}

func (a *myInboundAdapter) createMetadata(conn net.Conn, metadata adapter.InboundContext) adapter.InboundContext {
	metadata.Inbound = a.tag
	metadata.InboundType = a.protocol
	metadata.InboundDetour = a.listenOptions.Detour
	metadata.SniffEnabled = a.listenOptions.SniffEnabled
	metadata.SniffOverrideDestination = a.listenOptions.SniffOverrideDestination
	metadata.DomainStrategy = dns.DomainStrategy(a.listenOptions.DomainStrategy)
	if !metadata.Source.IsValid() {
		metadata.Source = M.SocksaddrFromNet(conn.RemoteAddr())
	}
	if !metadata.Destination.IsValid() {
		metadata.Destination = M.SocksaddrFromNet(conn.LocalAddr())
	}
	if tcpConn, isTCP := common.Cast[*net.TCPConn](conn); isTCP {
		metadata.OriginDestination = M.SocksaddrFromNet(tcpConn.LocalAddr())
	}
	return metadata
}

func (a *myInboundAdapter) newError(err error) {
	a.logger.Error(err)
}

func (a *myInboundAdapter) NewError(ctx context.Context, err error) {
	NewError(a.logger, ctx, err)
}

func NewError(logger log.ContextLogger, ctx context.Context, err error) {
	common.Close(err)
	if E.IsClosedOrCanceled(err) {
		logger.DebugContext(ctx, "connection closed: ", err)
		return
	}
	logger.ErrorContext(ctx, err)
}
