//go:build with_quic

package v2rayquic

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/qtls"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/hysteria"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx          context.Context
	tlsConfig    tls.ServerConfig
	quicConfig   *quic.Config
	handler      adapter.V2RayServerTransportHandler
	udpListener  net.PacketConn
	quicListener qtls.QUICListener
}

func NewServer(ctx context.Context, options option.V2RayQUICOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (adapter.V2RayServerTransport, error) {
	quicConfig := &quic.Config{
		DisablePathMTUDiscovery: !C.IsLinux && !C.IsWindows,
	}
	if len(tlsConfig.NextProtos()) == 0 {
		tlsConfig.SetNextProtos([]string{"h2", "http/1.1"})
	}
	server := &Server{
		ctx:        ctx,
		tlsConfig:  tlsConfig,
		quicConfig: quicConfig,
		handler:    handler,
	}
	return server, nil
}

func (s *Server) Network() []string {
	return []string{N.NetworkUDP}
}

func (s *Server) Serve(listener net.Listener) error {
	return os.ErrInvalid
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	quicListener, err := qtls.Listen(listener, s.tlsConfig, s.quicConfig)
	if err != nil {
		return err
	}
	s.udpListener = listener
	s.quicListener = quicListener
	go s.acceptLoop()
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.quicListener.Accept(s.ctx)
		if err != nil {
			return
		}
		go func() {
			hErr := s.streamAcceptLoop(conn)
			if hErr != nil {
				s.handler.NewError(conn.Context(), hErr)
			}
		}()
	}
}

func (s *Server) streamAcceptLoop(conn quic.Connection) error {
	for {
		stream, err := conn.AcceptStream(s.ctx)
		if err != nil {
			return err
		}
		go s.handler.NewConnection(conn.Context(), &hysteria.StreamWrapper{Conn: conn, Stream: stream}, M.Metadata{})
	}
}

func (s *Server) Close() error {
	return common.Close(s.udpListener, s.quicListener)
}
