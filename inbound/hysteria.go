//go:build with_quic

package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/humanize"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-quic/hysteria"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = (*Hysteria)(nil)

type Hysteria struct {
	myInboundAdapter
	tlsConfig    tls.ServerConfig
	service      *hysteria.Service[int]
	userNameList []string
}

func NewHysteria(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaInboundOptions) (*Hysteria, error) {
	options.UDPFragmentDefault = true
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	inbound := &Hysteria{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeHysteria,
			network:       []string{N.NetworkUDP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		tlsConfig: tlsConfig,
	}
	var sendBps, receiveBps uint64
	if len(options.Up) > 0 {
		sendBps, err = humanize.ParseBytes(options.Up)
		if err != nil {
			return nil, E.Cause(err, "invalid up speed format: ", options.Up)
		}
	} else {
		sendBps = uint64(options.UpMbps) * hysteria.MbpsToBps
	}
	if len(options.Down) > 0 {
		receiveBps, err = humanize.ParseBytes(options.Down)
		if receiveBps == 0 {
			return nil, E.New("invalid down speed format: ", options.Down)
		}
	} else {
		receiveBps = uint64(options.DownMbps) * hysteria.MbpsToBps
	}
	service, err := hysteria.NewService[int](hysteria.ServiceOptions{
		Context:       ctx,
		Logger:        logger,
		SendBPS:       sendBps,
		ReceiveBPS:    receiveBps,
		XPlusPassword: options.Obfs,
		TLSConfig:     tlsConfig,
		Handler:       adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newConnection, inbound.newPacketConnection, nil),

		// Legacy options

		ConnReceiveWindow:   options.ReceiveWindowConn,
		StreamReceiveWindow: options.ReceiveWindowClient,
		MaxIncomingStreams:  int64(options.MaxConnClient),
		DisableMTUDiscovery: options.DisableMTUDiscovery,
	})
	if err != nil {
		return nil, err
	}
	userList := make([]int, 0, len(options.Users))
	userNameList := make([]string, 0, len(options.Users))
	userPasswordList := make([]string, 0, len(options.Users))
	for index, user := range options.Users {
		userList = append(userList, index)
		userNameList = append(userNameList, user.Name)
		var password string
		if user.AuthString != "" {
			password = user.AuthString
		} else {
			password = string(user.Auth)
		}
		userPasswordList = append(userPasswordList, password)
	}
	service.UpdateUsers(userList, userPasswordList)
	inbound.service = service
	inbound.userNameList = userNameList
	return inbound, nil
}

func (h *Hysteria) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	metadata = h.createMetadata(conn, metadata)
	userID, _ := auth.UserFromContext[int](ctx)
	if userName := h.userNameList[userID]; userName != "" {
		metadata.User = userName
		h.logger.InfoContext(ctx, "[", userName, "] inbound connection to ", metadata.Destination)
	} else {
		h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	}
	return h.router.RouteConnection(ctx, conn, metadata)
}

func (h *Hysteria) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	metadata = h.createPacketMetadata(conn, metadata)
	userID, _ := auth.UserFromContext[int](ctx)
	if userName := h.userNameList[userID]; userName != "" {
		metadata.User = userName
		h.logger.InfoContext(ctx, "[", userName, "] inbound packet connection to ", metadata.Destination)
	} else {
		h.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	}
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}

func (h *Hysteria) Start() error {
	if h.tlsConfig != nil {
		err := h.tlsConfig.Start()
		if err != nil {
			return err
		}
	}
	packetConn, err := h.myInboundAdapter.ListenUDP()
	if err != nil {
		return err
	}
	return h.service.Start(packetConn)
}

func (h *Hysteria) Close() error {
	return common.Close(
		&h.myInboundAdapter,
		h.tlsConfig,
		common.PtrOrNil(h.service),
	)
}
