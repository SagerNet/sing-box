package v2raygrpclite

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2rayhttp"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	sHttp "github.com/sagernet/sing/protocol/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	tlsConfig    tls.ServerConfig
	handler      adapter.V2RayServerTransportHandler
	errorHandler E.Handler
	httpServer   *http.Server
	h2Server     *http2.Server
	h2cHandler   http.Handler
	path         string
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func NewServer(ctx context.Context, options option.V2RayGRPCOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	server := &Server{
		tlsConfig: tlsConfig,
		handler:   handler,
		path:      fmt.Sprintf("/%s/Tun", url.QueryEscape(options.ServiceName)),
		h2Server:  new(http2.Server),
	}
	server.httpServer = &http.Server{
		Handler: server,
	}
	server.h2cHandler = h2c.NewHandler(server, server.h2Server)
	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == "PRI" && len(request.Header) == 0 && request.URL.Path == "*" && request.Proto == "HTTP/2.0" {
		s.h2cHandler.ServeHTTP(writer, request)
		return
	}
	if request.URL.Path != s.path {
		s.fallbackRequest(request.Context(), writer, request, http.StatusNotFound, E.New("bad path: ", request.URL.Path))
		return
	}
	if request.Method != http.MethodPost {
		s.fallbackRequest(request.Context(), writer, request, http.StatusNotFound, E.New("bad method: ", request.Method))
		return
	}
	if ct := request.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/grpc") {
		s.fallbackRequest(request.Context(), writer, request, http.StatusNotFound, E.New("bad content type: ", ct))
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

func (s *Server) fallbackRequest(ctx context.Context, writer http.ResponseWriter, request *http.Request, statusCode int, err error) {
	conn := v2rayhttp.NewHTTPConn(request.Body, writer)
	fErr := s.handler.FallbackConnection(ctx, &conn, M.Metadata{})
	if fErr == nil {
		return
	} else if fErr == os.ErrInvalid {
		fErr = nil
	}
	writer.WriteHeader(statusCode)
	s.handler.NewError(request.Context(), E.Cause(E.Errors(err, E.Cause(fErr, "fallback connection")), "process connection from ", request.RemoteAddr))
}

func (s *Server) Serve(listener net.Listener) error {
	if s.tlsConfig != nil {
		if len(s.tlsConfig.NextProtos()) == 0 {
			s.tlsConfig.SetNextProtos([]string{http2.NextProtoTLS})
		}
		listener = aTLS.NewListener(listener, s.tlsConfig)
	}
	return s.httpServer.Serve(listener)
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	return os.ErrInvalid
}

func (s *Server) Close() error {
	return common.Close(common.PtrOrNil(s.httpServer))
}
