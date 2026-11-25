package redirect

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/redir"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterTProxy(registry *inbound.Registry) {
	inbound.Register[option.TProxyInboundOptions](registry, C.TypeTProxy, NewTProxy)
}

type TProxy struct {
	inbound.Adapter
	ctx      context.Context
	router   adapter.Router
	logger   log.ContextLogger
	listener *listener.Listener
	udpNat   *tun.UDPNat
}

func NewTProxy(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TProxyInboundOptions) (adapter.Inbound, error) {
	tproxy := &TProxy{
		Adapter: inbound.NewAdapter(C.TypeTProxy, tag),
		ctx:     ctx,
		router:  router,
		logger:  logger,
	}
	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}
	networkManager := service.FromContext[adapter.NetworkManager](ctx)
	tproxy.udpNat = tun.NewUDPNat(tun.UDPNatOptions{
		Handler:         tproxy,
		Prepare:         tproxy.preparePacketConnection,
		Timeout:         udpTimeout,
		Mapping:         tun.NATMapping(options.UDPMapping),
		Filtering:       tun.NATFiltering(options.UDPFiltering),
		MaxSize:         options.UDPNATMax,
		InterfaceFinder: networkManager.InterfaceFinder(),
	})
	tproxy.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           options.Network.Build(),
		Listen:            options.ListenOptions,
		ConnectionHandler: tproxy,
		OOBPacketHandler:  tproxy,
		TProxy:            true,
	})
	return tproxy, nil
}

func (t *TProxy) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := t.udpNat.Start()
	if err != nil {
		return err
	}
	scope.Add(t.udpNat.Close)
	err = t.listener.Start()
	if err != nil {
		return err
	}
	scope.Add(t.listener.Close)
	return nil
}

func (t *TProxy) InterfaceUpdated(ctx context.Context) {
	t.udpNat.Purge()
}

func (t *TProxy) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Inbound = t.Tag()
	metadata.InboundType = t.Type()
	metadata.Destination = M.SocksaddrFromNet(conn.LocalAddr()).Unwrap()
	t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	t.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (t *TProxy) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	t.logger.InfoContext(ctx, "inbound packet connection from ", source)
	t.logger.InfoContext(ctx, "inbound packet connection to ", destination)
	var metadata adapter.InboundContext
	metadata.Inbound = t.Tag()
	metadata.InboundType = t.Type()
	metadata.Source = source
	metadata.Destination = destination
	metadata.OriginDestination = t.listener.UDPAddr()
	t.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func (t *TProxy) NewPacket(buffer *buf.Buffer, oob []byte, source M.Socksaddr) {
	destination, err := redir.GetOriginalDestinationFromOOB(oob)
	if err != nil {
		t.logger.Warn("process packet from ", source, ": get tproxy destination: ", err)
		return
	}
	t.udpNat.NewPacket([][]byte{buffer.Bytes()}, source, M.SocksaddrFromNetIP(destination), nil)
}

func (t *TProxy) preparePacketConnection(source M.Socksaddr, destination M.Socksaddr, userData any) (bool, context.Context, N.PacketWriter, N.CloseHandlerFunc) {
	ctx := log.ContextWithNewID(t.ctx)
	writer := &tproxyPacketWriter{
		ctx:         ctx,
		listener:    t.listener,
		source:      source.AddrPort(),
		destination: destination,
	}
	return true, ctx, writer, func(it error) {
		writer.close()
	}
}

type tproxyPacketWriter struct {
	ctx         context.Context
	listener    *listener.Listener
	source      netip.AddrPort
	destination M.Socksaddr
	access      sync.Mutex
	conn        *net.UDPConn
	closed      bool
}

func (w *tproxyPacketWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	if destination != w.destination {
		conn, err := w.listenPacket(destination)
		if err != nil {
			return err
		}
		defer conn.Close()
		return common.Error(conn.WriteToUDPAddrPort(buffer.Bytes(), w.source))
	}
	conn, err := w.cachedConn()
	if err != nil {
		return err
	}
	return common.Error(conn.WriteToUDPAddrPort(buffer.Bytes(), w.source))
}

func (w *tproxyPacketWriter) cachedConn() (*net.UDPConn, error) {
	w.access.Lock()
	defer w.access.Unlock()
	if w.closed {
		return nil, net.ErrClosed
	}
	if w.conn == nil {
		conn, err := w.listenPacket(w.destination)
		if err != nil {
			return nil, err
		}
		w.conn = conn
	}
	return w.conn, nil
}

func (w *tproxyPacketWriter) listenPacket(destination M.Socksaddr) (*net.UDPConn, error) {
	var listenConfig net.ListenConfig
	listenConfig.Control = control.Append(listenConfig.Control, control.ReuseAddr())
	listenConfig.Control = control.Append(listenConfig.Control, redir.TProxyWriteBack())
	packetConn, err := w.listener.ListenPacket(listenConfig, w.ctx, "udp", destination.String())
	if err != nil {
		return nil, err
	}
	return packetConn.(*net.UDPConn), nil
}

func (w *tproxyPacketWriter) close() {
	w.access.Lock()
	defer w.access.Unlock()
	w.closed = true
	common.Close(common.PtrOrNil(w.conn))
}
