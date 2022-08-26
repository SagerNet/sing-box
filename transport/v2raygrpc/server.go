package v2raygrpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHttp "github.com/sagernet/sing/protocol/http"

	"golang.org/x/net/http2"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx          context.Context
	canceler     context.CancelFunc
	handler      N.TCPConnectionHandler
	errorHandler E.Handler
	h2Opts       *http2.ServeConnOpts
	h2Server     *http2.Server
	path         string
	tlsConfig    *tls.Config
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func NewServer(ctx context.Context, options option.V2RayGRPCOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler, errorHandler E.Handler) *Server {
	server := &Server{
		handler:      handler,
		errorHandler: errorHandler,
		path:         fmt.Sprintf("/%s/Tun", url.QueryEscape(options.ServiceName)),
		tlsConfig:    tlsConfig,
		h2Server:     &http2.Server{},
	}
	server.ctx, server.canceler = context.WithCancel(ctx)
	if !common.Contains(tlsConfig.NextProtos, http2.NextProtoTLS) {
		tlsConfig.NextProtos = append(tlsConfig.NextProtos, http2.NextProtoTLS)
	}
	server.h2Opts = &http2.ServeConnOpts{
		Context: ctx,
		Handler: server,
		BaseConfig: &http.Server{
			ReadHeaderTimeout: C.TCPTimeout,
			MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
			Handler:           server,
		},
	}
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != s.path {
		writer.WriteHeader(http.StatusNotFound)
		s.badRequest(request, E.New("bad path: ", request.URL.Path))
		return
	}
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusNotFound)
		s.badRequest(request, E.New("bad method: ", request.Method))
		return
	}
	if ct := request.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/grpc") {
		writer.WriteHeader(http.StatusNotFound)
		s.badRequest(request, E.New("bad content type: ", ct))
		return
	}

	writer.Header().Set("Content-Type", "application/grpc")
	writer.Header().Set("TE", "trailers")

	writer.WriteHeader(http.StatusOK)
	//if f, ok := writer.(http.Flusher); ok {
	//	f.Flush()
	//}
	var metadata M.Metadata
	metadata.Source = sHttp.SourceAddress(request)
	conn := newGunConn(request.Body, writer, request.Body)
	s.handler.NewConnection(request.Context(), conn, metadata)
}

func (s *Server) badRequest(request *http.Request, err error) {
	s.errorHandler.NewError(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Serve(listener net.Listener) error {
	tlsEnabled := s.tlsConfig != nil
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		if tlsEnabled {
			tlsConn := tls.Server(conn, s.tlsConfig.Clone())
			err = tlsConn.HandshakeContext(s.ctx)
			if err != nil {
				continue
			}
			conn = tlsConn
		}
		go s.h2Server.ServeConn(conn, s.h2Opts)
	}
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	return os.ErrInvalid
}

func (s *Server) Close() error {
	s.canceler()
	return nil
}
