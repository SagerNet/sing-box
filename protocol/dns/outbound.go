package dns

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.StubOptions](registry, C.TypeDNS, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	router adapter.DNSRouter
	logger logger.ContextLogger
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.StubOptions) (adapter.Outbound, error) {
	return &Outbound{
		Adapter: outbound.NewAdapter(C.TypeDNS, tag, []string{N.NetworkTCP, N.NetworkUDP}, nil),
		router:  service.FromContext[adapter.DNSRouter](ctx),
		logger:  logger,
	}, nil
}

func (d *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, os.ErrInvalid
}

func (d *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (d *Outbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	metadata.Destination = M.Socksaddr{}
	for {
		conn.SetReadDeadline(time.Now().Add(C.DNSTimeout))
		err := HandleStreamDNSRequest(ctx, d.router, conn, metadata)
		if err != nil {
			conn.Close()
			if onClose != nil {
				onClose(err)
			}
			return
		}
	}
}

func (d *Outbound) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	NewDNSPacketConnection(ctx, d.router, conn, nil, metadata)
}
