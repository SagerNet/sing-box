//go:build with_mtproto

package inbound

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/mtproto"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/replay"
)

var (
	_ adapter.Inbound           = (*MTProto)(nil)
	_ adapter.InjectableInbound = (*MTProto)(nil)
)

type MTProto struct {
	myInboundAdapter
	secret      mtproto.Secret
	replayCache replay.Filter
}

func NewMTProto(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.MTProtoInboundOptions) (*MTProto, error) {
	inbound := &MTProto{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeMTProto,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		replayCache: replay.NewSimple(time.Minute),
	}
	inbound.connHandler = inbound
	var err error
	inbound.secret, err = mtproto.ParseSecret(options.Secret)
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

func (m *MTProto) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	fakeTLSConn, err := mtproto.FakeTLSHandshake(ctx, conn, m.secret, m.replayCache)
	if err != nil {
		return err
	}
	dc, err := mtproto.Obfs2ClientHandshake(m.secret.Key[:], fakeTLSConn)
	if err != nil {
		return err
	}
	if !mtproto.AddressPool.IsValidDC(dc) {
		return E.New("unknown DC: ", dc)
	}
	dcAddr := mtproto.AddressPool.GetV4(dc)

	metadata.Protocol = "mtproto"
	metadata.Destination = dcAddr[0]

	return m.router.RouteConnection(ctx, fakeTLSConn, metadata)
}

func (m *MTProto) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}
