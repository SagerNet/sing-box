package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = &WSC{}
var _ adapter.InjectableInbound = &WSC{}

type WSC struct {
	myInboundAdapter
}

func NewWSC(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.WSCInboundOptions) (*WSC, error) {
	wsc := &WSC{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeWSC,
			network:       []string{network.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}
	wsc.connHandler = wsc
	return wsc, nil
}

func (wsc *WSC) Close() error {
	return wsc.myInboundAdapter.Close()
}

func (wsc *WSC) Start() error {
	return wsc.myInboundAdapter.Start()
}

func (wsc *WSC) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	wsc.injectTCP(conn, metadata)
	return nil
}

func (wsc *WSC) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext) error {
	wsc.myInboundAdapter.NewError(ctx, network.ErrUnknownNetwork)
	conn.Close()
	return network.ErrUnknownNetwork
}

func (wsc *WSC) Inject(conn net.Conn, metadata adapter.InboundContext) error {
	wsc.injectTCP(conn, metadata)
	return nil
}

func (wsc *WSC) NewError(ctx context.Context, err error) {
	wsc.myInboundAdapter.NewError(ctx, err)
}
