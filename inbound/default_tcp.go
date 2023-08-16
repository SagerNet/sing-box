package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/proxyproto"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func (a *myInboundAdapter) ListenTCP() (net.Listener, error) {
	var err error
	bindAddr := M.SocksaddrFrom(a.listenOptions.Listen.Build(), a.listenOptions.ListenPort)
	var tcpListener net.Listener
	var listenConfig net.ListenConfig
	if a.listenOptions.TCPMultiPath {
		if !go121Available {
			return nil, E.New("MultiPath TCP requires go1.21, please recompile your binary.")
		}
		setMultiPathTCP(&listenConfig)
	}
	if a.listenOptions.TCPFastOpen {
		if !go120Available {
			return nil, E.New("TCP Fast Open requires go1.20, please recompile your binary.")
		}
		tcpListener, err = listenTFO(listenConfig, a.ctx, M.NetworkFromNetAddr(N.NetworkTCP, bindAddr.Addr), bindAddr.String())
	} else {
		tcpListener, err = listenConfig.Listen(a.ctx, M.NetworkFromNetAddr(N.NetworkTCP, bindAddr.Addr), bindAddr.String())
	}
	if err == nil {
		a.logger.Info("tcp server started at ", tcpListener.Addr())
	}
	if a.listenOptions.ProxyProtocol {
		a.logger.Debug("proxy protocol enabled")
		tcpListener = &proxyproto.Listener{Listener: tcpListener, AcceptNoHeader: a.listenOptions.ProxyProtocolAcceptNoHeader}
	}
	a.tcpListener = tcpListener
	return tcpListener, err
}

func (a *myInboundAdapter) loopTCPIn() {
	tcpListener := a.tcpListener
	for {
		conn, err := tcpListener.Accept()
		if err != nil {
			//goland:noinspection GoDeprecation
			//nolint:staticcheck
			if netError, isNetError := err.(net.Error); isNetError && netError.Temporary() {
				a.logger.Error(err)
				continue
			}
			if a.inShutdown.Load() && E.IsClosed(err) {
				return
			}
			a.tcpListener.Close()
			a.logger.Error("serve error: ", err)
			continue
		}
		go a.injectTCP(conn, adapter.InboundContext{})
	}
}

func (a *myInboundAdapter) injectTCP(conn net.Conn, metadata adapter.InboundContext) {
	ctx := log.ContextWithNewID(a.ctx)
	metadata = a.createMetadata(conn, metadata)
	a.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	hErr := a.connHandler.NewConnection(ctx, conn, metadata)
	if hErr != nil {
		conn.Close()
		a.NewError(ctx, E.Cause(hErr, "process connection from ", metadata.Source))
	}
}

func (a *myInboundAdapter) routeTCP(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) {
	a.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	hErr := a.newConnection(ctx, conn, metadata)
	if hErr != nil {
		conn.Close()
		a.NewError(ctx, E.Cause(hErr, "process connection from ", metadata.Source))
	}
}
