package listener

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/settings"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Listener struct {
	ctx                      context.Context
	logger                   logger.ContextLogger
	network                  []string
	listenOptions            option.ListenOptions
	connHandler              adapter.ConnectionHandlerEx
	packetHandler            adapter.PacketHandlerEx
	oobPacketHandler         adapter.OOBPacketHandlerEx
	threadUnsafePacketWriter bool
	disablePacketOutput      bool
	setSystemProxy           bool
	systemProxySOCKS         bool

	tcpListener          net.Listener
	systemProxy          settings.SystemProxy
	udpConn              *net.UDPConn
	udpAddr              M.Socksaddr
	packetOutbound       chan *N.PacketBuffer
	packetOutboundClosed chan struct{}
	shutdown             atomic.Bool
}

type Options struct {
	Context                  context.Context
	Logger                   logger.ContextLogger
	Network                  []string
	Listen                   option.ListenOptions
	ConnectionHandler        adapter.ConnectionHandlerEx
	PacketHandler            adapter.PacketHandlerEx
	OOBPacketHandler         adapter.OOBPacketHandlerEx
	ThreadUnsafePacketWriter bool
	DisablePacketOutput      bool
	SetSystemProxy           bool
	SystemProxySOCKS         bool
}

func New(
	options Options,
) *Listener {
	return &Listener{
		ctx:                      options.Context,
		logger:                   options.Logger,
		network:                  options.Network,
		listenOptions:            options.Listen,
		connHandler:              options.ConnectionHandler,
		packetHandler:            options.PacketHandler,
		oobPacketHandler:         options.OOBPacketHandler,
		threadUnsafePacketWriter: options.ThreadUnsafePacketWriter,
		disablePacketOutput:      options.DisablePacketOutput,
		setSystemProxy:           options.SetSystemProxy,
		systemProxySOCKS:         options.SystemProxySOCKS,
	}
}

func (l *Listener) Start() error {
	if common.Contains(l.network, N.NetworkTCP) {
		_, err := l.ListenTCP()
		if err != nil {
			return err
		}
		go l.loopTCPIn()
	}
	if common.Contains(l.network, N.NetworkUDP) {
		_, err := l.ListenUDP()
		if err != nil {
			return err
		}
		l.packetOutboundClosed = make(chan struct{})
		l.packetOutbound = make(chan *N.PacketBuffer, 64)
		go l.loopUDPIn()
		if !l.disablePacketOutput {
			go l.loopUDPOut()
		}
	}
	if l.setSystemProxy {
		listenPort := M.SocksaddrFromNet(l.tcpListener.Addr()).Port
		var listenAddrString string
		listenAddr := l.listenOptions.Listen.Build(netip.IPv4Unspecified())
		if listenAddr.IsUnspecified() {
			listenAddrString = "127.0.0.1"
		} else {
			listenAddrString = listenAddr.String()
		}
		systemProxy, err := settings.NewSystemProxy(l.ctx, M.ParseSocksaddrHostPort(listenAddrString, listenPort), l.systemProxySOCKS)
		if err != nil {
			return E.Cause(err, "initialize system proxy")
		}
		err = systemProxy.Enable()
		if err != nil {
			return E.Cause(err, "set system proxy")
		}
		l.systemProxy = systemProxy
	}
	return nil
}

func (l *Listener) Close() error {
	l.shutdown.Store(true)
	var err error
	if l.systemProxy != nil && l.systemProxy.IsEnabled() {
		err = l.systemProxy.Disable()
	}
	return E.Errors(err, common.Close(
		l.tcpListener,
		common.PtrOrNil(l.udpConn),
	))
}

func (l *Listener) TCPListener() net.Listener {
	return l.tcpListener
}

func (l *Listener) UDPConn() *net.UDPConn {
	return l.udpConn
}

func (l *Listener) ListenOptions() option.ListenOptions {
	return l.listenOptions
}
