package inbound

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/wsc"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = &WSC{}
var _ adapter.InjectableInbound = &WSC{}

var _ adapter.WSCServerTransportHandler = &wscTransportHandler{}

var _ wsc.Authenticator = &CustomAuthenticator{}

type WSC struct {
	myInboundAdapter
	server    adapter.WSCServerTransport
	tlsConfig tls.ServerConfig
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
			network:       []string{network.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}
	server, err := wsc.NewServer(wsc.ServerConfig{
		Ctx:     ctx,
		Logger:  logger,
		Handler: (*wscTransportHandler)(inbound),
		Authenticator: &CustomAuthenticator{
			id:     0,
			logger: logger,
		},
		Router:                     router,
		MaxConnectionPerUser:       options.MaxConnectionPerUser,
		UsageReportTrafficInterval: options.UsageTraffic.Traffic,
		UsageReportTimeInterval:    time.Duration(options.UsageTraffic.Time),
	})
	if err != nil {
		return nil, err
	}
	if options.TLS != nil {
		inbound.tlsConfig, err = tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
	}
	inbound.server = server
	return inbound, nil
}

func (wsc *WSC) Close() error {
	return common.Close(&wsc.myInboundAdapter, wsc.tlsConfig, wsc.server)
}

func (wsc *WSC) Start() error {
	tcpListener, err := wsc.ListenTCP()
	if err != nil {
		return err
	}
	go func() {
		sErr := wsc.server.Serve(tcpListener)
		if sErr != nil && !exceptions.IsClosedOrCanceled(sErr) && !errors.Is(sErr, http.ErrServerClosed) {
			wsc.logger.Error("wsc server serve error: ", sErr)
		}
	}()
	return nil
}

func (wsc *WSC) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	wsc.routeTCP(ctx, conn, metadata)
	return nil
}

func (wsc *WSC) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata adapter.InboundContext) error {
	return wsc.myInboundAdapter.newPacketConnection(ctx, conn, metadata)
}

func (wsc *WSC) Inject(conn net.Conn, metadata adapter.InboundContext) error {
	wsc.injectTCP(conn, metadata)
	return nil
}

func (wsc *WSC) NewError(ctx context.Context, err error) {
	wsc.myInboundAdapter.NewError(ctx, err)
}

func (handler *wscTransportHandler) NewConnection(ctx context.Context, conn net.Conn, metadata metadata.Metadata) error {
	return (*WSC)(handler).NewConnection(ctx, conn, adapter.InboundContext{
		Source:      metadata.Source,
		Destination: metadata.Destination,
	})
}

func (handler *wscTransportHandler) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata metadata.Metadata) error {
	return (*WSC)(handler).NewPacketConnection(ctx, conn, adapter.InboundContext{
		Source:      metadata.Source,
		Destination: metadata.Destination,
	})
}

func (handler *wscTransportHandler) NewError(ctx context.Context, err error) {
	(*WSC)(handler).NewError(ctx, err)
}

func (auth *CustomAuthenticator) Authenticate(ctx context.Context, params wsc.AuthenticateParams) (wsc.AuthenticateResult, error) {
	auth.id++
	return wsc.AuthenticateResult{
		ID:      int64(auth.id),
		Rate:    math.MaxInt64,
		MaxConn: 60,
	}, nil
}

func (auth *CustomAuthenticator) ReportUsage(ctx context.Context, params wsc.ReportUsageParams) (wsc.ReportUsageResult, error) {
	auth.logger.Debug("Reporting usage : ", params.ID, " | ", params.UsedTraffic)
	return wsc.ReportUsageResult{}, nil
}
