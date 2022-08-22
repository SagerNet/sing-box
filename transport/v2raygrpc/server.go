package v2raygrpc

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx     context.Context
	handler N.TCPConnectionHandler
	server  *grpc.Server
}

func NewServer(ctx context.Context, serviceName string, tlsConfig *tls.Config, handler N.TCPConnectionHandler) *Server {
	var serverOptions []grpc.ServerOption
	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{"h2"}
		serverOptions = append(serverOptions, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	server := &Server{ctx, handler, grpc.NewServer(serverOptions...)}
	RegisterGunServiceCustomNameServer(server.server, server, serviceName)
	return server
}

func (s *Server) Tun(server GunService_TunServer) error {
	ctx, cancel := context.WithCancel(s.ctx)
	conn := NewGRPCConn(server, cancel)
	go s.handler.NewConnection(ctx, conn, M.Metadata{})
	<-ctx.Done()
	return nil
}

func (s *Server) mustEmbedUnimplementedGunServiceServer() {
}

func (s *Server) Serve(listener net.Listener) error {
	return s.server.Serve(listener)
}

func (s *Server) Close() error {
	s.server.Stop()
	return nil
}
