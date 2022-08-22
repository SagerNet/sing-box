package v2raygrpc

import (
	"context"
	"crypto/tls"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
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

func NewServer(ctx context.Context, options option.V2RayGRPCOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler) *Server {
	var serverOptions []grpc.ServerOption
	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{"h2"}
		serverOptions = append(serverOptions, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	server := &Server{ctx, handler, grpc.NewServer(serverOptions...)}
	RegisterGunServiceCustomNameServer(server.server, server, options.ServiceName)
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

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func (s *Server) Serve(listener net.Listener) error {
	return s.server.Serve(listener)
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	return os.ErrInvalid
}

func (s *Server) Close() error {
	s.server.Stop()
	return nil
}
