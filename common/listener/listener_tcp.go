package listener

import (
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/metacubex/tfo-go"
)

func (l *Listener) ListenTCP() (net.Listener, error) {
	var err error
	bindAddr := M.SocksaddrFrom(l.listenOptions.Listen.Build(), l.listenOptions.ListenPort)
	var tcpListener net.Listener
	var listenConfig net.ListenConfig
	if l.listenOptions.TCPKeepAlive >= 0 {
		keepIdle := time.Duration(l.listenOptions.TCPKeepAlive)
		if keepIdle == 0 {
			keepIdle = C.TCPKeepAliveInitial
		}
		keepInterval := time.Duration(l.listenOptions.TCPKeepAliveInterval)
		if keepInterval == 0 {
			keepInterval = C.TCPKeepAliveInterval
		}
		setKeepAliveConfig(&listenConfig, keepIdle, keepInterval)
	}
	if l.listenOptions.TCPMultiPath {
		if !go121Available {
			return nil, E.New("MultiPath TCP requires go1.21, please recompile your binary.")
		}
		setMultiPathTCP(&listenConfig)
	}
	if l.listenOptions.TCPFastOpen {
		var tfoConfig tfo.ListenConfig
		tfoConfig.ListenConfig = listenConfig
		tcpListener, err = tfoConfig.Listen(l.ctx, M.NetworkFromNetAddr(N.NetworkTCP, bindAddr.Addr), bindAddr.String())
	} else {
		tcpListener, err = listenConfig.Listen(l.ctx, M.NetworkFromNetAddr(N.NetworkTCP, bindAddr.Addr), bindAddr.String())
	}
	if err == nil {
		l.logger.Info("tcp server started at ", tcpListener.Addr())
	}
	//nolint:staticcheck
	if l.listenOptions.ProxyProtocol || l.listenOptions.ProxyProtocolAcceptNoHeader {
		return nil, E.New("Proxy Protocol is deprecated and removed in sing-box 1.6.0")
	}
	l.tcpListener = tcpListener
	return tcpListener, err
}

func (l *Listener) loopTCPIn() {
	tcpListener := l.tcpListener
	var metadata adapter.InboundContext
	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			//nolint:staticcheck
			if netError, isNetError := err.(net.Error); isNetError && netError.Temporary() {
				l.logger.Error(err)
				continue
			}
			if l.shutdown.Load() && E.IsClosed(err) {
				return
			}
			l.tcpListener.Close()
			l.logger.Error("tcp listener closed: ", err)
			continue
		}
		//nolint:staticcheck
		metadata.InboundDetour = l.listenOptions.Detour
		//nolint:staticcheck
		metadata.InboundOptions = l.listenOptions.InboundOptions
		metadata.Source = M.SocksaddrFromNet(conn.RemoteAddr()).Unwrap()
		metadata.OriginDestination = M.SocksaddrFromNet(conn.LocalAddr()).Unwrap()
		ctx := log.ContextWithNewID(l.ctx)
		l.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
		go l.connHandler.NewConnectionEx(ctx, conn, metadata, nil)
	}
}
