package v2rayquic

import (
	"context"
	"crypto/tls"
	"net"
	"os"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/hysteria"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx          context.Context
	tlsConfig    *tls.Config
	quicConfig   *quic.Config
	handler      N.TCPConnectionHandler
	errorHandler E.Handler
	udpListener  net.PacketConn
	quicListener quic.Listener
}

func NewServer(ctx context.Context, options option.V2RayQUICOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler, errorHandler E.Handler) *Server {
	quicConfig := &quic.Config{
		DisablePathMTUDiscovery: !C.IsLinux && !C.IsWindows,
	}
	if len(tlsConfig.NextProtos) == 0 {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}
	server := &Server{
		ctx:          ctx,
		tlsConfig:    tlsConfig,
		quicConfig:   quicConfig,
		handler:      handler,
		errorHandler: errorHandler,
	}
	return server
}

func (s *Server) Network() []string {
	return []string{N.NetworkUDP}
}

func (s *Server) Serve(listener net.Listener) error {
	return os.ErrInvalid
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	quicListener, err := quic.Listen(listener, s.tlsConfig, s.quicConfig)
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
				s.errorHandler.NewError(conn.Context(), hErr)
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
