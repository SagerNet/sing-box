package hysteria2

import (
	"context"
	"errors"
	"net"
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
	sHTTP "github.com/sagernet/sing/protocol/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"golang.org/x/net/http2"
)

func RegisterRealmService(registry *boxService.Registry) {
	boxService.Register[option.HysteriaRealmServiceOptions](registry, C.TypeHysteriaRealm, NewRealmService)
}

type RealmService struct {
	boxService.Adapter
	ctx        context.Context
	cancel     context.CancelFunc
	logger     log.ContextLogger
	listener   *listener.Listener
	tlsConfig  tls.ServerConfig
	httpServer *http.Server
	server     *server
}

func NewRealmService(ctx context.Context, logger log.ContextLogger, tag string, options option.HysteriaRealmServiceOptions) (adapter.Service, error) {
	if len(options.Users) == 0 {
		return nil, E.New("missing users")
	}
	tokenMap := make(map[string]*realmUser, len(options.Users))
	for i, user := range options.Users {
		if user.Name == "" {
			return nil, E.New("missing name for user[", i, "]")
		}
		if user.Token == "" {
			return nil, E.New("missing token for user[", i, "]")
		}
		tokenMap[user.Token] = &realmUser{
			name:      user.Name,
			maxRealms: user.MaxRealms,
		}
	}
	server := newServer(logger, tokenMap)
	ctx, cancel := context.WithCancel(ctx)
	chiRouter := chi.NewRouter()
	chiRouter.Use(middleware.RequestSize(maxRequestBodyBytes))
	chiRouter.Use(func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.DebugContext(r.Context(), r.Method, " ", r.RequestURI, " ", sHTTP.SourceAddress(r))
			handler.ServeHTTP(w, r)
		})
	})
	chiRouter.Route("/v1/{id}", func(r chi.Router) {
		r.Use(validateRealmID)
		r.With(server.authUser).Post("/", server.handleRegister)
		r.With(server.authSession).Delete("/", server.handleDeregister)
		r.With(server.authSession).Get("/events", server.handleEvents)
		r.With(server.authSession).Post("/heartbeat", server.handleHeartbeat)
		r.With(server.authUser).Post("/connect", server.handleConnect)
		r.With(server.authSession).Post("/connects/{nonce}", server.handleConnectResponse)
	})
	chiRouter.NotFound(func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, render.M{"error": "not_found", "message": "unknown path"})
	})
	chiRouter.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, http.StatusMethodNotAllowed)
		render.JSON(w, r, render.M{"error": "bad_request", "message": "method not allowed"})
	})
	s := &RealmService{
		Adapter: boxService.NewAdapter(C.TypeHysteriaRealm, tag),
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		httpServer: &http.Server{
			Handler: chiRouter,
			ConnContext: func(ctx context.Context, _ net.Conn) context.Context {
				return log.ContextWithNewID(ctx)
			},
		},
		server: server,
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

func (s *RealmService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if s.tlsConfig != nil {
		err := s.tlsConfig.Start()
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

func (s *RealmService) Close() error {
	s.cancel()
	err := common.Close(common.PtrOrNil(s.httpServer))
	s.server.closeAll()
	return E.Errors(err, common.Close(
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	))
}
