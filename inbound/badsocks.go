package inbound

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks2"
	"github.com/sagernet/sing-shadowsocks2/badsocks"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Inbound           = (*Badsocks)(nil)
	_ adapter.InjectableInbound = (*Badsocks)(nil)
)

type Badsocks struct {
	myInboundAdapter
	service shadowsocks.Service
}

func NewBadsocks(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.BadsocksInboundOptions) (*Badsocks, error) {
	inbound := &Badsocks{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeBadsocks,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}
	service, err := badsocks.NewService(ctx, badsocks.MethodName, shadowsocks.ServiceOptions{
		Password: options.Password,
		Handler:  inbound.upstreamContextHandler(),
	})
	if err != nil {
		return nil, err
	}
	inbound.service = service
	inbound.connHandler = inbound
	return inbound, err
}

func (h *Badsocks) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return h.service.NewConnection(adapter.WithContext(log.ContextWithNewID(ctx), &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (h *Badsocks) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}
