package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/settings"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/atomic"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = (*myInboundAdapter)(nil)

type myInboundAdapter struct {
	protocol         string
	network          []string
	ctx              context.Context
	router           adapter.ConnectionRouterEx
	logger           log.ContextLogger
	tag              string
	listenOptions    option.ListenOptions
	connHandler      adapter.ConnectionHandlerEx
	packetHandler    adapter.PacketHandlerEx
	oobPacketHandler adapter.OOBPacketHandlerEx
	packetUpstream   any

	// http mixed

	setSystemProxy bool
	systemProxy    settings.SystemProxy

	// internal

	tcpListener          net.Listener
	udpConn              *net.UDPConn
	udpAddr              M.Socksaddr
	packetOutboundClosed chan struct{}
	packetOutbound       chan *myInboundPacket

	inShutdown atomic.Bool
}

func (a *myInboundAdapter) Type() string {
	return a.protocol
}

func (a *myInboundAdapter) Tag() string {
	return a.tag
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
		listenPort := M.SocksaddrFromNet(a.tcpListener.Addr()).Port
		var listenAddrString string
		listenAddr := a.listenOptions.Listen.Build()
		if listenAddr.IsUnspecified() {
			listenAddrString = "127.0.0.1"
		} else {
			listenAddrString = listenAddr.String()
		}
		var systemProxy settings.SystemProxy
		systemProxy, err = settings.NewSystemProxy(a.ctx, M.ParseSocksaddrHostPort(listenAddrString, listenPort), a.protocol == C.TypeMixed)
		if err != nil {
			return E.Cause(err, "initialize system proxy")
		}
		err = systemProxy.Enable()
		if err != nil {
			return E.Cause(err, "set system proxy")
		}
		a.systemProxy = systemProxy
	}
	return nil
}

func (a *myInboundAdapter) Close() error {
	a.inShutdown.Store(true)
	var err error
	if a.systemProxy != nil && a.systemProxy.IsEnabled() {
		err = a.systemProxy.Disable()
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

func (a *myInboundAdapter) upstreamHandlerEx(metadata adapter.InboundContext) adapter.UpstreamHandlerAdapterEx {
	return adapter.NewUpstreamHandlerEx(metadata, a.newConnectionEx, a.streamPacketConnectionEx)
}

func (a *myInboundAdapter) upstreamContextHandlerEx() adapter.UpstreamHandlerAdapterEx {
	return adapter.NewUpstreamContextHandlerEx(a.newConnectionEx, a.newPacketConnectionEx)
}

func (a *myInboundAdapter) newConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	a.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	a.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (a *myInboundAdapter) newPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	ctx = log.ContextWithNewID(ctx)
	a.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	a.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	a.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func (a *myInboundAdapter) streamPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	a.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	a.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func (a *myInboundAdapter) createMetadata(conn net.Conn, metadata adapter.InboundContext) adapter.InboundContext {
	metadata.Inbound = a.tag
	metadata.InboundType = a.protocol
	metadata.InboundDetour = a.listenOptions.Detour
	metadata.InboundOptions = a.listenOptions.InboundOptions
	if !metadata.Source.IsValid() {
		metadata.Source = M.SocksaddrFromNet(conn.RemoteAddr()).Unwrap()
	}
	if !metadata.Destination.IsValid() {
		metadata.Destination = M.SocksaddrFromNet(conn.LocalAddr()).Unwrap()
	}
	if tcpConn, isTCP := common.Cast[*net.TCPConn](conn); isTCP {
		metadata.OriginDestination = M.SocksaddrFromNet(tcpConn.LocalAddr()).Unwrap()
	}
	return metadata
}

// Deprecated: don't use
func (a *myInboundAdapter) newError(err error) {
	a.logger.Error(err)
}

// Deprecated: don't use
func (a *myInboundAdapter) NewError(ctx context.Context, err error) {
	NewError(a.logger, ctx, err)
}

// Deprecated: don't use
func NewError(logger log.ContextLogger, ctx context.Context, err error) {
	common.Close(err)
	if E.IsClosedOrCanceled(err) {
		logger.DebugContext(ctx, "connection closed: ", err)
		return
	}
	logger.ErrorContext(ctx, err)
}
