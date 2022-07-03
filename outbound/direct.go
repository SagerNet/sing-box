package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound/dialer"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*Direct)(nil)

type Direct struct {
	myOutboundAdapter
	overrideOption      int
	overrideDestination M.Socksaddr
}

func NewDirect(router adapter.Router, logger log.Logger, tag string, options option.DirectOutboundOptions) *Direct {
	outbound := &Direct{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeDirect,
			logger:   logger,
			tag:      tag,
			dialer:   dialer.New(router, options.DialerOptions),
		},
	}
	if options.OverrideAddress != "" && options.OverridePort != 0 {
		outbound.overrideOption = 1
		outbound.overrideDestination = M.ParseSocksaddrHostPort(options.OverrideAddress, options.OverridePort)
	} else if options.OverrideAddress != "" {
		outbound.overrideOption = 2
		outbound.overrideDestination = M.ParseSocksaddrHostPort(options.OverrideAddress, options.OverridePort)
	} else if options.OverridePort != 0 {
		outbound.overrideOption = 3
		outbound.overrideDestination = M.Socksaddr{Port: options.OverridePort}
	}
	return outbound
}

func (d *Direct) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch d.overrideOption {
	case 1:
		destination = d.overrideDestination
	case 2:
		newDestination := d.overrideDestination
		newDestination.Port = destination.Port
		destination = newDestination
	case 3:
		destination.Port = d.overrideDestination.Port
	}
	switch network {
	case C.NetworkTCP:
		d.logger.WithContext(ctx).Info("outbound connection to ", destination)
	case C.NetworkUDP:
		d.logger.WithContext(ctx).Info("outbound packet connection to ", destination)
	}
	return d.dialer.DialContext(ctx, network, destination)
}

func (d *Direct) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	d.logger.WithContext(ctx).Info("outbound packet connection")
	return d.dialer.ListenPacket(ctx, destination)
}

func (d *Direct) NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	outConn, err := d.DialContext(ctx, C.NetworkTCP, destination)
	if err != nil {
		return err
	}
	return bufio.CopyConn(ctx, conn, outConn)
}

func (d *Direct) NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error {
	outConn, err := d.ListenPacket(ctx, destination)
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outConn))
}
