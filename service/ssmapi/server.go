package ssmapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	"github.com/sagernet/sing/service"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/http2"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.SSMAPIServiceOptions](registry, C.TypeSSMAPI, NewService)
}

type Service struct {
	boxService.Adapter
	ctx        context.Context
	logger     log.ContextLogger
	listener   *listener.Listener
	tlsConfig  tls.ServerConfig
	httpServer *http.Server
	traffics   map[string]*TrafficManager
	users      map[string]*UserManager
	cachePath  string
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.SSMAPIServiceOptions) (adapter.Service, error) {
	chiRouter := chi.NewRouter()
	s := &Service{
		Adapter: boxService.NewAdapter(C.TypeSSMAPI, tag),
		ctx:     ctx,
		logger:  logger,
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		httpServer: &http.Server{
			Handler: chiRouter,
		},
		traffics:  make(map[string]*TrafficManager),
		users:     make(map[string]*UserManager),
		cachePath: options.CachePath,
	}
	inboundManager := service.FromContext[adapter.InboundManager](ctx)
	if options.Servers.Size() == 0 {
		return nil, E.New("missing servers")
	}
	for i, entry := range options.Servers.Entries() {
		inbound, loaded := inboundManager.Get(entry.Value)
		if !loaded {
			return nil, E.New("parse SSM server[", i, "]: inbound ", entry.Value, " not found")
		}
		managedServer, isManaged := inbound.(adapter.ManagedSSMServer)
		if !isManaged {
			return nil, E.New("parse SSM server[", i, "]: inbound/", inbound.Type(), "[", inbound.Tag(), "] is not a SSM server")
		}
		traffic := NewTrafficManager()
		managedServer.SetTracker(traffic)
		user := NewUserManager(managedServer, traffic)
		chiRouter.Route(entry.Key, NewAPIServer(logger, traffic, user).Route)
		s.traffics[entry.Key] = traffic
		s.users[entry.Key] = user
	}
	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		s.tlsConfig = tlsConfig
	}
	return s, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := s.loadCache()
	if err != nil {
		s.logger.Error(E.Cause(err, "load cache"))
	}
	if s.tlsConfig != nil {
		err = s.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
	}
	tcpListener, err := s.listener.ListenTCP()
	if err != nil {
		return err
	}
	if s.tlsConfig != nil {
		if !common.Contains(s.tlsConfig.NextProtos(), http2.NextProtoTLS) {
			s.tlsConfig.SetNextProtos(append([]string{"h2"}, s.tlsConfig.NextProtos()...))
		}
		tcpListener = aTLS.NewListener(tcpListener, s.tlsConfig)
	}
	go func() {
		err = s.httpServer.Serve(tcpListener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("serve error: ", err)
		}
	}()
	return nil
}

func (s *Service) Close() error {
	err := s.saveCache()
	if err != nil {
		s.logger.Error(E.Cause(err, "save cache"))
	}
	return common.Close(
		common.PtrOrNil(s.httpServer),
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	)
}
