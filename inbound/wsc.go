package inbound

import (
	"context"
	"math"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/mux"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-box/transport/wsc"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = &WSC{}
var _ adapter.InjectableInbound = &WSC{}

var _ adapter.V2RayServerTransportHandler = &wscTransportHandler{}

var _ wsc.Authenticator = &CustomAuthenticator{}

type WSC struct {
	myInboundAdapter
	service   *wsc.Service
	tlsConfig tls.ServerConfig
	transport adapter.V2RayServerTransport
}

type wscTransportHandler WSC

type CustomAuthenticator struct {
	id     uint64
	logger logger.ContextLogger
}

func NewWSC(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.WSCInboundOptions) (*WSC, error) {
	inbound := &WSC{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeWSC,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}

	var err error

	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}
	if options.Transport != nil {
		inbound.transport, err = v2ray.NewServerTransport(ctx, common.PtrValueOrDefault(options.Transport), inbound.tlsConfig, (*wscTransportHandler)(inbound))
		if err != nil {
			return nil, err
		}
	}

	inbound.router, err = mux.NewRouterWithOptions(inbound.router, logger, common.PtrValueOrDefault(options.Multiplex))
	if err != nil {
		return nil, err
	}

	inbound.service, err = wsc.NewService(wsc.ServiceConfig{
		Handler: adapter.NewUpstreamContextHandler(inbound.newConnection, inbound.newPacketConnection, inbound),
		Logger:  logger,
		Router:  router,
		Authenticator: &CustomAuthenticator{
			id:     0,
			logger: logger,
		},
		MaxConnectionPerUser:       options.MaxConnectionPerUser,
		UsageReportTrafficInterval: options.UsageTraffic.Traffic,
		UsageReportTimeInterval:    time.Duration(options.UsageTraffic.Time),
	})
	if err != nil {
		return nil, err
	}

	inbound.connHandler = inbound

	return inbound, nil
}

func (wsc *WSC) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	var err error
	if wsc.tlsConfig != nil && wsc.transport == nil {
		conn, err = tls.ServerHandshake(ctx, conn, wsc.tlsConfig)
		if err != nil {
			return err
		}
	}
	return wsc.service.NewConnection(adapter.WithContext(ctx, &metadata), conn, adapter.UpstreamMetadata(metadata))
}

func (wsc *WSC) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}

func (wsc *WSC) Close() error {
	return common.Close(&wsc.myInboundAdapter, wsc.tlsConfig, wsc.transport)
}

func (wsc *WSC) Start() error {
	if wsc.tlsConfig != nil {
		err := wsc.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
	}
	if wsc.transport == nil {
		return wsc.myInboundAdapter.Start()
	}

	if common.Contains(wsc.transport.Network(), N.NetworkTCP) {
		tcpListener, err := wsc.myInboundAdapter.ListenTCP()
		if err != nil {
			return err
		}
		go func() {
			sErr := wsc.transport.Serve(tcpListener)
			if sErr != nil && !E.IsClosed(sErr) {
				wsc.logger.Error("transport serve error: ", sErr)
			}
		}()
	}

	if common.Contains(wsc.transport.Network(), N.NetworkUDP) {
		udpConn, err := wsc.myInboundAdapter.ListenUDP()
		if err != nil {
			return err
		}
		go func() {
			sErr := wsc.transport.ServePacket(udpConn)
			if sErr != nil && !E.IsClosed(sErr) {
				wsc.logger.Error("transport serve error: ", sErr)
			}
		}()
	}

	return nil
}

func (wsc *WSC) newTransportConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	wsc.injectTCP(conn, metadata)
	return nil
}

func (wsc *WSC) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return wsc.router.RoutePacketConnection(ctx, conn, metadata)
}

func (handler *wscTransportHandler) NewConnection(ctx context.Context, conn net.Conn, metadata metadata.Metadata) error {
	return (*WSC)(handler).newTransportConnection(ctx, conn, adapter.InboundContext{
		Source:      metadata.Source,
		Destination: metadata.Destination,
	})
}

func (auth *CustomAuthenticator) Authenticate(ctx context.Context, params wsc.AuthenticateParams) (wsc.AuthenticateResult, error) {
	auth.id++
	return wsc.AuthenticateResult{
		ID:      int64(auth.id),
		Rate:    math.MaxInt64,
		MaxConn: params.MaxConn,
	}, nil
}

func (auth *CustomAuthenticator) ReportUsage(ctx context.Context, params wsc.ReportUsageParams) (wsc.ReportUsageResult, error) {
	auth.logger.Debug("Reporting usage : ", params.ID, " | ", params.UsedTraffic)
	return wsc.ReportUsageResult{}, nil
}
