package inbound

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat2"
)

var _ adapter.Inbound = (*Direct)(nil)

type Direct struct {
	myInboundAdapter
	udpNat              *udpnat.Service
	overrideOption      int
	overrideDestination M.Socksaddr
}

func NewDirect(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.DirectInboundOptions) *Direct {
	options.UDPFragmentDefault = true
	inbound := &Direct{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeDirect,
			network:       options.Network.Build(),
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
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
	inbound.connHandler = inbound
	inbound.packetHandler = inbound
	return inbound
}

func (d *Direct) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	switch d.overrideOption {
	case 1:
		metadata.Destination = d.overrideDestination
	case 2:
		destination := d.overrideDestination
		destination.Port = metadata.Destination.Port
		metadata.Destination = destination
	case 3:
		metadata.Destination.Port = d.overrideDestination.Port
	}
	d.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	return d.router.RouteConnection(ctx, conn, metadata)
}

func (d *Direct) NewPacketEx(buffer *buf.Buffer, source M.Socksaddr) {
	var destination M.Socksaddr
	switch d.overrideOption {
	case 1:
		destination = d.overrideDestination
	case 2:
		destination = d.overrideDestination
		destination.Port = source.Port
	case 3:
		destination = source
		destination.Port = d.overrideDestination.Port
	}
	d.udpNat.NewPacket([][]byte{buffer.Bytes()}, source, destination, nil)
}

func (d *Direct) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	d.newConnectionEx(ctx, conn, metadata, onClose)
}

func (d *Direct) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	d.newPacketConnectionEx(ctx, conn, d.createPacketMetadataEx(source, destination), onClose)
}

func (d *Direct) preparePacketConnection(source M.Socksaddr, destination M.Socksaddr, userData any) (bool, context.Context, N.PacketWriter, N.CloseHandlerFunc) {
	return true, d.ctx, &directPacketWriter{d.packetConn(), source}, nil
}

type directPacketWriter struct {
	writer N.PacketWriter
	source M.Socksaddr
}

func (w *directPacketWriter) WritePacket(buffer *buf.Buffer, addr M.Socksaddr) error {
	return w.writer.WritePacket(buffer, w.source)
}
