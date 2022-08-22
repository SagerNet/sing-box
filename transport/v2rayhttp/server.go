package v2rayhttp

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx          context.Context
	handler      N.TCPConnectionHandler
	errorHandler E.Handler
	httpServer   *http.Server
	host         []string
	path         string
	method       string
	headers      http.Header
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func NewServer(ctx context.Context, options option.V2RayHTTPOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler, errorHandler E.Handler) *Server {
	server := &Server{
		ctx:          ctx,
		handler:      handler,
		errorHandler: errorHandler,
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
		TLSConfig:         tlsConfig,
	}
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
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
	if f, ok := writer.(http.Flusher); ok {
		f.Flush()
	}

	if h, ok := writer.(http.Hijacker); ok {
		conn, _, err := h.Hijack()
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			s.badRequest(request, E.Cause(err, "hijack conn"))
			return
		}
		s.handler.NewConnection(request.Context(), conn, M.Metadata{})
	} else {
		conn := &ServerHTTPConn{
			HTTPConn{
				request.Body,
				writer,
			},
			writer.(http.Flusher),
		}
		s.handler.NewConnection(request.Context(), conn, M.Metadata{})
	}
}

func (s *Server) badRequest(request *http.Request, err error) {
	s.errorHandler.NewError(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Serve(listener net.Listener) error {
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
