package v2raygrpc

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/common/tls"
	"github.com/jobberrt/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"google.golang.org/grpc"
	gM "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx     context.Context
	handler N.TCPConnectionHandler
	server  *grpc.Server
}

func NewServer(ctx context.Context, options option.V2RayGRPCOptions, tlsConfig tls.ServerConfig, handler N.TCPConnectionHandler) (*Server, error) {
	var serverOptions []grpc.ServerOption
	if tlsConfig != nil {
		tlsConfig.SetNextProtos([]string{"h2"})
		serverOptions = append(serverOptions, grpc.Creds(NewTLSTransportCredentials(tlsConfig)))
	}
	server := &Server{ctx, handler, grpc.NewServer(serverOptions...)}
	RegisterGunServiceCustomNameServer(server.server, server, options.ServiceName)
	return server, nil
}

func (s *Server) Tun(server GunService_TunServer) error {
	ctx, cancel := context.WithCancel(s.ctx)
	conn := NewGRPCConn(server, cancel)
	var metadata M.Metadata
	if remotePeer, loaded := peer.FromContext(server.Context()); loaded {
		metadata.Source = M.SocksaddrFromNet(remotePeer.Addr)
	}
	if grpcMetadata, loaded := gM.FromIncomingContext(server.Context()); loaded {
		forwardFrom := strings.Join(grpcMetadata.Get("X-Forwarded-For"), ",")
		if forwardFrom != "" {
			for _, from := range strings.Split(forwardFrom, ",") {
				originAddr := M.ParseSocksaddr(from)
				if originAddr.IsValid() {
					metadata.Source = originAddr.Unwrap()
				}
			}
		}
	}
	go s.handler.NewConnection(ctx, conn, metadata)
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
