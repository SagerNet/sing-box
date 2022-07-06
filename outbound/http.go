package outbound

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/http"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound/dialer"
)

var _ adapter.Outbound = (*HTTP)(nil)

type HTTP struct {
	myOutboundAdapter
	client *http.Client
}

func NewHTTP(router adapter.Router, logger log.Logger, tag string, options option.HTTPOutboundOptions) *HTTP {
	return &HTTP{
		myOutboundAdapter{
			protocol: C.TypeHTTP,
			logger:   logger,
			tag:      tag,
			network:  []string{C.NetworkTCP},
		},
		http.NewClient(dialer.New(router, options.DialerOptions), M.ParseSocksaddrHostPort(options.Server, options.ServerPort), options.Username, options.Password),
	}
}

func (h *HTTP) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	h.logger.WithContext(ctx).Info("outbound connection to ", destination)
	return h.client.DialContext(ctx, network, destination)
}

func (h *HTTP) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (h *HTTP) NewConnection(ctx context.Context, conn net.Conn, destination M.Socksaddr) error {
	outConn, err := h.DialContext(ctx, C.NetworkTCP, destination)
	if err != nil {
		return err
	}
	return bufio.CopyConn(ctx, conn, outConn)
}

func (h *HTTP) NewPacketConnection(ctx context.Context, conn N.PacketConn, destination M.Socksaddr) error {
	return os.ErrInvalid
}
