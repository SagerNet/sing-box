package v2raygrpclite

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/common/tls"
	"github.com/jobberrt/sing-box/option"
	"github.com/jobberrt/sing-box/transport/v2rayhttp"
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

func NewServer(ctx context.Context, options option.V2RayGRPCOptions, tlsConfig tls.ServerConfig, handler N.TCPConnectionHandler, errorHandler E.Handler) (*Server, error) {
	server := &Server{
		handler:      handler,
		errorHandler: errorHandler,
		path:         fmt.Sprintf("/%s/Tun", url.QueryEscape(options.ServiceName)),
		h2Server:     new(http2.Server),
	}
	server.httpServer = &http.Server{
		Handler: server,
	}
	server.h2cHandler = h2c.NewHandler(server, server.h2Server)
	if tlsConfig != nil {
		stdConfig, err := tlsConfig.Config()
		if err != nil {
			return nil, err
		}
		if len(stdConfig.NextProtos) == 0 {
			stdConfig.NextProtos = []string{http2.NextProtoTLS}
		}
		server.httpServer.TLSConfig = stdConfig
	}
	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == "PRI" && len(request.Header) == 0 && request.URL.Path == "*" && request.Proto == "HTTP/2.0" {
		s.h2cHandler.ServeHTTP(writer, request)
		return
	}
	if request.URL.Path != s.path {
		request.Write(os.Stdout)
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
	conn := v2rayhttp.NewHTTP2Wrapper(newGunConn(request.Body, writer, writer.(http.Flusher)))
	s.handler.NewConnection(request.Context(), conn, metadata)
	conn.CloseWrapper()
}

func (s *Server) badRequest(request *http.Request, err error) {
	s.errorHandler.NewError(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Serve(listener net.Listener) error {
	fixTLSConfig := s.httpServer.TLSConfig == nil
	err := http2.ConfigureServer(s.httpServer, s.h2Server)
	if err != nil {
		return err
	}
	if fixTLSConfig {
		s.httpServer.TLSConfig = nil
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
