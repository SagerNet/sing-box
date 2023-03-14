package outbound

import (
	"context"
	"net"
	"os"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/common/dialer"
	"github.com/jobberrt/sing-box/common/tls"
	C "github.com/jobberrt/sing-box/constant"
	"github.com/jobberrt/sing-box/log"
	"github.com/jobberrt/sing-box/option"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/http"
)

var _ adapter.Outbound = (*HTTP)(nil)

type HTTP struct {
	myOutboundAdapter
	client *http.Client
}

func NewHTTP(router adapter.Router, logger log.ContextLogger, tag string, options option.HTTPOutboundOptions) (*HTTP, error) {
	detour, err := tls.NewDialerFromOptions(router, dialer.New(router, options.DialerOptions), options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	return &HTTP{
		myOutboundAdapter{
			protocol: C.TypeHTTP,
			network:  []string{N.NetworkTCP},
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		http.NewClient(detour, options.ServerOptions.Build(), options.Username, options.Password),
	}, nil
}

func (h *HTTP) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	h.logger.InfoContext(ctx, "outbound connection to ", destination)
	return h.client.DialContext(ctx, network, destination)
}

func (h *HTTP) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (h *HTTP) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *HTTP) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}
