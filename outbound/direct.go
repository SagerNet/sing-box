package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

var _ adapter.Outbound = (*Direct)(nil)

type Direct struct {
	myOutboundAdapter
	dialer              N.Dialer
	overrideOption      int
	overrideDestination M.Socksaddr
}

func NewDirect(router adapter.Router, logger log.Logger, tag string, options option.DirectOutboundOptions) *Direct {
	outbound := &Direct{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeDirect,
			logger:   logger,
			tag:      tag,
			network:  []string{C.NetworkTCP, C.NetworkUDP},
		},
		dialer: dialer.New(router, options.DialerOptions),
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

func (h *Direct) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	switch h.overrideOption {
	case 1:
		destination = h.overrideDestination
	case 2:
		newDestination := h.overrideDestination
		newDestination.Port = destination.Port
		destination = newDestination
	case 3:
		destination.Port = h.overrideDestination.Port
	}
	switch network {
	case C.NetworkTCP:
		h.logger.WithContext(ctx).Info("outbound connection to ", destination)
	case C.NetworkUDP:
		h.logger.WithContext(ctx).Info("outbound packet connection to ", destination)
	}
	return h.dialer.DialContext(ctx, network, destination)
}

func (h *Direct) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	h.logger.WithContext(ctx).Info("outbound packet connection")
	return h.dialer.ListenPacket(ctx, destination)
}

func (h *Direct) NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	outConn, err := h.DialContext(ctx, C.NetworkTCP, destination)
	if err != nil {
		return err
	}
	return bufio.CopyConn(ctx, conn, outConn)
}

func (h *Direct) NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error {
	outConn, err := h.ListenPacket(ctx, destination)
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(outConn))
}
