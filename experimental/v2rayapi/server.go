package v2rayapi

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	experimental.RegisterV2RayServerConstructor(NewServer)
}

var _ adapter.V2RayServer = (*Server)(nil)

type Server struct {
	ctx          context.Context
	logger       log.Logger
	listen       string
	tcpListener  net.Listener
	grpcServer   *grpc.Server
	statsService *StatsService
	tlsConfig    tls.ServerConfig
}

func NewServer(ctx context.Context, logger log.Logger, options option.V2RayAPIOptions) (adapter.V2RayServer, error) {
	grpcServer := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	statsService := NewStatsService(common.PtrValueOrDefault(options.Stats))
	if statsService != nil {
		RegisterStatsServiceServer(grpcServer, statsService)
	}
	var tlsConfig tls.ServerConfig
	if options.TLS != nil {
		var err error
		tlsConfig, err = tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
	}
	server := &Server{
		ctx:          ctx,
		logger:       logger,
		listen:       options.Listen,
		grpcServer:   grpcServer,
		statsService: statsService,
		tlsConfig:    tlsConfig,
	}
	return server, nil
}

func (s *Server) Start() error {
	if s.tlsConfig != nil {
		err := s.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
	}
	listener, err := tls.NewListener(s.ctx, s.listen, s.tlsConfig)
	if err != nil {
		return err
	}
	s.logger.Info("grpc server started at ", listener.Addr())
	s.tcpListener = listener
	go func() {
		err = s.grpcServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error(err)
		}
	}()
	return nil
}

func (s *Server) Close() error {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
	return common.Close(
		common.PtrOrNil(s.grpcServer),
		s.tcpListener,
	)
}

func (s *Server) StatsService() adapter.V2RayStatsService {
	return s.statsService
}
