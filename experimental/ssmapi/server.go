package ssmapi

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"github.com/go-chi/chi/v5"
)

func init() {
	experimental.RegisterSSMServerConstructor(NewServer)
}

var _ adapter.SSMServer = (*Server)(nil)

type Server struct {
	router      adapter.Router
	logger      log.Logger
	httpServer  *http.Server
	tcpListener net.Listener

	nodes          []Node
	userManager    *UserManager
	trafficManager *TrafficManager
}

type Node interface {
	Protocol() string
	ID() string
	Shadowsocks() ShadowsocksNodeObject
	Object() any
	Tag() string
	UpdateUsers(users []string, uPSKs []string) error
}

func NewServer(router adapter.Router, logger log.Logger, options option.SSMAPIOptions) (adapter.SSMServer, error) {
	chiRouter := chi.NewRouter()
	server := &Server{
		router: router,
		logger: logger,
		httpServer: &http.Server{
			Addr:    options.Listen,
			Handler: chiRouter,
		},
		nodes: make([]Node, 0, len(options.Nodes)),
	}
	for i, nodeOptions := range options.Nodes {
		switch nodeOptions.Type {
		case C.TypeShadowsocks:
			ssOptions := nodeOptions.ShadowsocksOptions
			inbound, loaded := router.Inbound(ssOptions.Inbound)
			if !loaded {
				return nil, E.New("parse SSM node[", i, "]: inbound", ssOptions.Inbound, "not found")
			}
			ssInbound, isSS := inbound.(adapter.ManagedShadowsocksServer)
			if !isSS {
				return nil, E.New("parse SSM node[", i, "]: inbound", ssOptions.Inbound, "is not a shadowsocks inbound")
			}
			node := &ShadowsocksNode{
				ssOptions,
				ssInbound,
			}
			server.nodes = append(server.nodes, node)
		}
	}
	server.trafficManager = NewTrafficManager(server.nodes)
	server.userManager = NewUserManager(server.nodes, server.trafficManager)
	listenPrefix := options.ListenPrefix
	if !strings.HasPrefix(listenPrefix, "/") {
		listenPrefix = "/" + listenPrefix
	}
	chiRouter.Route(listenPrefix+"server/v1", server.setupRoutes)
	return server, nil
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}
	s.logger.Info("ssm-api started at ", listener.Addr())
	s.tcpListener = listener
	go func() {
		err = s.httpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("ssm-api serve error: ", err)
		}
	}()
	return nil
}

func (s *Server) Close() error {
	return common.Close(
		common.PtrOrNil(s.httpServer),
		s.tcpListener,
		s.trafficManager,
	)
}

func (s *Server) RoutedConnection(metadata adapter.InboundContext, conn net.Conn) net.Conn {
	return s.trafficManager.RoutedConnection(metadata, conn)
}

func (s *Server) RoutedPacketConnection(metadata adapter.InboundContext, conn N.PacketConn) N.PacketConn {
	return s.trafficManager.RoutedPacketConnection(metadata, conn)
}
