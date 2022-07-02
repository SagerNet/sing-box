package inbound

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat"
)

var _ adapter.Inbound = (*Direct)(nil)

type Direct struct {
	myInboundAdapter
	udpNat              *udpnat.Service[netip.AddrPort]
	overrideOption      int
	overrideDestination M.Socksaddr
}

func NewDirect(ctx context.Context, router adapter.Router, logger log.Logger, tag string, options *option.DirectInboundOptions) *Direct {
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
	inbound.udpNat = udpnat.New[netip.AddrPort](options.UDPTimeout, inbound.upstreamContextHandler())
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
	d.logger.WithContext(ctx).Info("inbound connection to ", metadata.Destination)
	return d.router.RouteConnection(ctx, conn, metadata)
}

func (d *Direct) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
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
	var upstreamMetadata M.Metadata
	upstreamMetadata.Source = metadata.Source
	upstreamMetadata.Destination = metadata.Destination
	d.udpNat.NewPacketDirect(&adapter.MetadataContext{Context: log.ContextWithID(ctx), Metadata: metadata}, metadata.Source.AddrPort(), conn, buffer, upstreamMetadata)
	return nil
}
