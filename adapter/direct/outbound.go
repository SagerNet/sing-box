package direct

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/config"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*Outbound)(nil)

type Outbound struct {
	tag                 string
	logger              log.Logger
	dialer              N.Dialer
	overrideOption      int
	overrideDestination M.Socksaddr
}

func NewOutbound(tag string, router adapter.Router, logger log.Logger, options *config.DirectOutboundOptions) (outbound *Outbound) {
	outbound = &Outbound{
		tag:    tag,
		logger: logger,
		dialer: dialer.NewDialer(router, options.DialerOptions),
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
	return
}

func (d *Outbound) Type() string {
	return C.TypeDirect
}

func (d *Outbound) Tag() string {
	return d.tag
}

func (d *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch d.overrideOption {
	case 0:
		destination = d.overrideDestination
	case 1:
		newDestination := d.overrideDestination
		newDestination.Port = destination.Port
		destination = newDestination
	case 2:
		destination.Port = d.overrideDestination.Port
	}
	switch network {
	case C.NetworkTCP:
		d.logger.WithContext(ctx).Debug("outbound connection to ", destination)
	case C.NetworkUDP:
		d.logger.WithContext(ctx).Debug("outbound packet connection to ", destination)
	}
	return d.dialer.DialContext(ctx, network, destination)
}

func (d *Outbound) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	d.logger.WithContext(ctx).Debug("outbound packet connection")
	return d.dialer.ListenPacket(ctx)
}

func (d *Outbound) NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	outConn, err := d.DialContext(ctx, "tcp", destination)
	if err != nil {
		return err
	}
	return bufio.CopyConn(ctx, conn, outConn)
}

func (d *Outbound) NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error {
	outConn, err := d.ListenPacket(ctx)
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outConn))
}
