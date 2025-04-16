package direct

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat2"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.DirectInboundOptions](registry, C.TypeDirect, NewInbound)
}

type Inbound struct {
	inbound.Adapter
	ctx                 context.Context
	router              adapter.ConnectionRouterEx
	logger              log.ContextLogger
	listener            *listener.Listener
	udpNat              *udpnat.Service
	overrideOption      int
	overrideDestination M.Socksaddr
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.DirectInboundOptions) (adapter.Inbound, error) {
	options.UDPFragmentDefault = true
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeDirect, tag),
		ctx:     ctx,
		router:  router,
		logger:  logger,
	}
	if options.OverrideAddress != "" && options.OverridePort != 0 {
		inbound.overrideOption = 1
		inbound.overrideDestination = M.ParseSocksaddrHostPort(options.OverrideAddress, options.OverridePort)
	} else if options.OverrideAddress != "" {
		inbound.overrideOption = 2
		inbound.overrideDestination = M.ParseSocksaddrHostPort(options.OverrideAddress, options.OverridePort)
	} else if options.OverridePort != 0 {
		inbound.overrideOption = 3
		inbound.overrideDestination = M.Socksaddr{Port: options.OverridePort}
	}
	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}
	inbound.udpNat = udpnat.New(inbound, inbound.preparePacketConnection, udpTimeout, false)
	inbound.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           options.Network.Build(),
		Listen:            options.ListenOptions,
		ConnectionHandler: inbound,
		PacketHandler:     inbound,
	})
	return inbound, nil
}

func (i *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return i.listener.Start()
}

func (i *Inbound) Close() error {
	return i.listener.Close()
}

func (i *Inbound) NewPacketEx(buffer *buf.Buffer, source M.Socksaddr) {
	i.udpNat.NewPacket([][]byte{buffer.Bytes()}, source, i.listener.UDPAddr(), nil)
}

func (i *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Inbound = i.Tag()
	metadata.InboundType = i.Type()
	destination := metadata.OriginDestination
	switch i.overrideOption {
	case 1:
		destination = i.overrideDestination
	case 2:
		destination.Addr = i.overrideDestination.Addr
	case 3:
		destination.Port = i.overrideDestination.Port
	}
	metadata.Destination = destination
	if i.overrideOption != 0 {
		i.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	}
	i.router.RouteConnectionEx(ctx, conn, metadata, onClose)
}

func (i *Inbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	i.logger.InfoContext(ctx, "inbound packet connection from ", source)
	var metadata adapter.InboundContext
	metadata.Inbound = i.Tag()
	metadata.InboundType = i.Type()
	//nolint:staticcheck
	metadata.InboundDetour = i.listener.ListenOptions().Detour
	//nolint:staticcheck
	metadata.InboundOptions = i.listener.ListenOptions().InboundOptions
	metadata.Source = source
	destination = i.listener.UDPAddr()
	switch i.overrideOption {
	case 1:
		destination = i.overrideDestination
	case 2:
		destination.Addr = i.overrideDestination.Addr
	case 3:
		destination.Port = i.overrideDestination.Port
	default:
	}
	i.logger.InfoContext(ctx, "inbound packet connection to ", destination)
	metadata.Destination = destination
	if i.overrideOption != 0 {
		conn = bufio.NewDestinationNATPacketConn(bufio.NewNetPacketConn(conn), i.listener.UDPAddr(), destination)
	}
	i.router.RoutePacketConnectionEx(ctx, conn, metadata, onClose)
}

func (i *Inbound) preparePacketConnection(source M.Socksaddr, destination M.Socksaddr, userData any) (bool, context.Context, N.PacketWriter, N.CloseHandlerFunc) {
	return true, log.ContextWithNewID(i.ctx), &directPacketWriter{i.listener.PacketWriter(), source}, nil
}

type directPacketWriter struct {
	writer N.PacketWriter
	source M.Socksaddr
}

func (w *directPacketWriter) WritePacket(buffer *buf.Buffer, addr M.Socksaddr) error {
	return w.writer.WritePacket(buffer, w.source)
}
