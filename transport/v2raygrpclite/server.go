package v2raygrpclite

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
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHttp "github.com/sagernet/sing/protocol/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	handler      N.TCPConnectionHandler
	errorHandler E.Handler
	httpServer   *http.Server
	h2Server     *http2.Server
	h2cHandler   http.Handler
	path         string
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func NewServer(ctx context.Context, options option.V2RayGRPCOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler, errorHandler E.Handler) *Server {
	server := &Server{
		handler:      handler,
		errorHandler: errorHandler,
		path:         fmt.Sprintf("/%s/Tun", url.QueryEscape(options.ServiceName)),
		h2Server:     new(http2.Server),
	}
	if tlsConfig != nil {
		if !common.Contains(tlsConfig.NextProtos, "h2") {
			tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2")
		}
	}
	server.httpServer = &http.Server{
		Handler:   server,
		TLSConfig: tlsConfig,
	}
	server.h2cHandler = h2c.NewHandler(server, server.h2Server)
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == "PRI" && len(request.Header) == 0 && request.URL.Path == "*" && request.Proto == "HTTP/2.0" {
		s.h2cHandler.ServeHTTP(writer, request)
		return
	}
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
	var metadata M.Metadata
	metadata.Source = sHttp.SourceAddress(request)
	conn := newGunConn(request.Body, writer, writer.(http.Flusher))
	s.handler.NewConnection(request.Context(), conn, metadata)
}

func (s *Server) badRequest(request *http.Request, err error) {
	s.errorHandler.NewError(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Serve(listener net.Listener) error {
	err := http2.ConfigureServer(s.httpServer, s.h2Server)
	if err != nil {
		return err
	}
	if s.httpServer.TLSConfig == nil {
		return s.httpServer.Serve(listener)
	} else {
		return s.httpServer.ServeTLS(listener, "", "")
	}
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	return os.ErrInvalid
}

func (s *Server) Close() error {
	return common.Close(common.PtrOrNil(s.httpServer))
}
