package direct

import (
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/udpnat"
	"github.com/sagernet/sing-box/config"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.InboundHandler = (*Inbound)(nil)

type Inbound struct {
	router              adapter.Router
	logger              log.Logger
	network             []string
	udpNat              *udpnat.Service[netip.AddrPort]
	overrideOption      int
	overrideDestination M.Socksaddr
}

func NewInbound(router adapter.Router, logger log.Logger, options *config.DirectInboundOptions) (inbound *Inbound) {
	inbound = &Inbound{
		router:  router,
		logger:  logger,
		network: options.Network.Build(),
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
	inbound.udpNat = udpnat.New[netip.AddrPort](options.UDPTimeout, inbound)
	return
}

func (d *Inbound) Type() string {
	return C.TypeDirect
}

func (d *Inbound) Network() []string {
	return d.network
}

func (d *Inbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	switch d.overrideOption {
	case 0:
		metadata.Destination = d.overrideDestination
	case 1:
		destination := d.overrideDestination
		destination.Port = metadata.Destination.Port
		metadata.Destination = destination
	case 2:
		metadata.Destination.Port = d.overrideDestination.Port
	}
	d.logger.WithContext(ctx).Info("inbound connection to ", metadata.Destination)
	return d.router.RouteConnection(ctx, conn, metadata)
}

func (d *Inbound) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
	switch d.overrideOption {
	case 0:
		metadata.Destination = d.overrideDestination
	case 1:
		destination := d.overrideDestination
		destination.Port = metadata.Destination.Port
		metadata.Destination = destination
	case 2:
		metadata.Destination.Port = d.overrideDestination.Port
	}
	d.udpNat.NewPacketDirect(ctx, metadata.Source, conn, buffer, metadata)
	return nil
}

func (d *Inbound) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	d.logger.WithContext(ctx).Info("inbound packet connection to ", metadata.Destination)
	return d.router.RoutePacketConnection(ctx, conn, metadata)
}

func (d *Inbound) NewError(ctx context.Context, err error) {
	d.logger.WithContext(ctx).Error(err)
}
