package v2rayhttp

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/common/tls"
	C "github.com/jobberrt/sing-box/constant"
	"github.com/jobberrt/sing-box/option"
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
	ctx          context.Context
	handler      N.TCPConnectionHandler
	errorHandler E.Handler
	httpServer   *http.Server
	h2Server     *http2.Server
	h2cHandler   http.Handler
	host         []string
	path         string
	method       string
	headers      http.Header
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func NewServer(ctx context.Context, options option.V2RayHTTPOptions, tlsConfig tls.ServerConfig, handler N.TCPConnectionHandler, errorHandler E.Handler) (*Server, error) {
	server := &Server{
		ctx:          ctx,
		handler:      handler,
		errorHandler: errorHandler,
		h2Server:     new(http2.Server),
		host:         options.Host,
		path:         options.Path,
		method:       options.Method,
		headers:      make(http.Header),
	}
	if server.method == "" {
		server.method = "PUT"
	}
	if !strings.HasPrefix(server.path, "/") {
		server.path = "/" + server.path
	}
	for key, value := range options.Headers {
		server.headers.Set(key, value)
	}
	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: C.TCPTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
	}
	server.h2cHandler = h2c.NewHandler(server, server.h2Server)
	if tlsConfig != nil {
		stdConfig, err := tlsConfig.Config()
		if err != nil {
			return nil, err
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
	host := request.Host
	if len(s.host) > 0 && !common.Contains(s.host, host) {
		writer.WriteHeader(http.StatusBadRequest)
		s.badRequest(request, E.New("bad host: ", host))
		return
	}
	if !strings.HasPrefix(request.URL.Path, s.path) {
		writer.WriteHeader(http.StatusNotFound)
		s.badRequest(request, E.New("bad path: ", request.URL.Path))
		return
	}
	if request.Method != s.method {
		writer.WriteHeader(http.StatusNotFound)
		s.badRequest(request, E.New("bad method: ", request.Method))
		return
	}

	writer.Header().Set("Cache-Control", "no-store")

	for key, values := range s.headers {
		for _, value := range values {
			writer.Header().Set(key, value)
		}
	}

	writer.WriteHeader(http.StatusOK)
	writer.(http.Flusher).Flush()

	var metadata M.Metadata
	metadata.Source = sHttp.SourceAddress(request)
	if h, ok := writer.(http.Hijacker); ok {
		conn, _, err := h.Hijack()
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			s.badRequest(request, E.Cause(err, "hijack conn"))
			return
		}
		s.handler.NewConnection(request.Context(), conn, metadata)
	} else {
		conn := NewHTTP2Wrapper(&ServerHTTPConn{
			newHTTPConn(request.Body, writer),
			writer.(http.Flusher),
		})
		s.handler.NewConnection(request.Context(), conn, metadata)
		conn.CloseWrapper()
	}
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
