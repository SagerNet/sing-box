package v2rayapi

import (
	"errors"
	"net"
	"net/http"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/experimental"
	"github.com/jobberrt/sing-box/log"
	"github.com/jobberrt/sing-box/option"
	"github.com/sagernet/sing/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	experimental.RegisterV2RayServerConstructor(NewServer)
}

var _ adapter.V2RayServer = (*Server)(nil)

type Server struct {
	logger       log.Logger
	listen       string
	tcpListener  net.Listener
	grpcServer   *grpc.Server
	statsService *StatsService
}

func NewServer(logger log.Logger, options option.V2RayAPIOptions) (adapter.V2RayServer, error) {
	grpcServer := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	statsService := NewStatsService(common.PtrValueOrDefault(options.Stats))
	if statsService != nil {
		RegisterStatsServiceServer(grpcServer, statsService)
	}
	server := &Server{
		logger:       logger,
		listen:       options.Listen,
		grpcServer:   grpcServer,
		statsService: statsService,
	}
	return server, nil
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.listen)
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
