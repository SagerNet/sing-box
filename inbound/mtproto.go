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
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/replay"
)

var (
	_ adapter.Inbound           = (*MTProto)(nil)
	_ adapter.InjectableInbound = (*MTProto)(nil)
)

type MTProto struct {
	myInboundAdapter
	userList    []string
	secretList  []*mtproto.Secret
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
		// testDataCenter: options.TestDataCenter,
	}
	inbound.connHandler = inbound
	err := inbound.UpdateUsers(common.MapIndexed(options.Users, func(index int, user option.MTProtoUser) string {
		return user.Name
	}), common.Map(options.Users, func(user option.MTProtoUser) string {
		return user.Secret
	}))
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

func (m *MTProto) UpdateUsers(userList []string, secretTextList []string) error {
	secretList := make([]*mtproto.Secret, len(secretTextList))
	for i, secretText := range secretTextList {
		userName := userList[i]
		if userName == "" {
			userName = F.ToString(i)
		}
		if secretText == "" {
			return E.New("missing secret for user ", userName)
		}
		secret, err := mtproto.ParseSecret(secretText)
		if err != nil {
			return E.Cause(err, "parse user ", userName)
		}
		secretList[i] = secret
	}
	m.userList = userList
	m.secretList = secretList
	return nil
}

func (m *MTProto) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	secretIndex, fakeTLSConn, err := mtproto.FakeTLSHandshake(ctx, conn, m.secretList, m.replayCache)
	if err != nil {
		return err
	}
	dataCenter, err := mtproto.Obfs2ClientHandshake(m.secretList[secretIndex].Key[:], fakeTLSConn)
	if err != nil {
		return err
	}

	userName := m.userList[secretIndex]
	if userName == "" {
		userName = F.ToString(secretIndex)
	}
	m.logger.InfoContext(ctx, "[", userName, "] inbound connection to Telegram DC ", dataCenter)
	metadata.Protocol = "mtproto"
	metadata.Destination = M.Socksaddr{
		Fqdn: mtproto.DataCenterName(dataCenter) + ".telegram.sing-box.arpa",
		Port: 443,
	}
	serverAddress := mtproto.ProductionDataCenterAddress[dataCenter]
	if len(serverAddress) == 0 {
		m.logger.Debug("unknown data center: ", dataCenter)
	}
	metadata.DestinationAddresses = serverAddress
	return m.router.RouteConnection(ctx, fakeTLSConn, metadata)
}

func (m *MTProto) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}
