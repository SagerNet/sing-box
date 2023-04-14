package v2rayhttp

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
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
	ctx        context.Context
	tlsConfig  tls.ServerConfig
	handler    adapter.V2RayServerTransportHandler
	httpServer *http.Server
	h2Server   *http2.Server
	h2cHandler http.Handler
	host       []string
	path       string
	method     string
	headers    http.Header
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func NewServer(ctx context.Context, options option.V2RayHTTPOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	server := &Server{
		ctx:       ctx,
		tlsConfig: tlsConfig,
		handler:   handler,
		h2Server: &http2.Server{
			IdleTimeout: time.Duration(options.IdleTimeout),
		},
		host:    options.Host,
		path:    options.Path,
		method:  options.Method,
		headers: make(http.Header),
	}
	if server.method == "" {
		server.method = "PUT"
	}
	if !strings.HasPrefix(server.path, "/") {
		server.path = "/" + server.path
	}
	for key, value := range options.Headers {
		server.headers[key] = value
	}
	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: C.TCPTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	server.h2cHandler = h2c.NewHandler(server, server.h2Server)
	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == "PRI" && len(request.Header) == 0 && request.URL.Path == "*" && request.Proto == "HTTP/2.0" {
		s.h2cHandler.ServeHTTP(writer, request)
		return
	}
	host := request.Host
	if len(s.host) > 0 && !common.Contains(s.host, host) {
		s.fallbackRequest(request.Context(), writer, request, http.StatusBadRequest, E.New("bad host: ", host))
		return
	}
	if !strings.HasPrefix(request.URL.Path, s.path) {
		s.fallbackRequest(request.Context(), writer, request, http.StatusNotFound, E.New("bad path: ", request.URL.Path))
		return
	}
	if request.Method != s.method {
		s.fallbackRequest(request.Context(), writer, request, http.StatusNotFound, E.New("bad method: ", request.Method))
		return
	}

	writer.Header().Set("Cache-Control", "no-store")

	for key, values := range s.headers {
		for _, value := range values {
			writer.Header().Set(key, value)
		}
	}

	var metadata M.Metadata
	metadata.Source = sHttp.SourceAddress(request)
	if h, ok := writer.(http.Hijacker); ok {
		var requestBody *buf.Buffer
		if contentLength := int(request.ContentLength); contentLength > 0 {
			requestBody = buf.NewSize(contentLength)
			_, err := requestBody.ReadFullFrom(request.Body, contentLength)
			if err != nil {
				s.fallbackRequest(request.Context(), writer, request, 0, E.Cause(err, "read request"))
				return
			}
		}
		writer.WriteHeader(http.StatusOK)
		writer.(http.Flusher).Flush()
		conn, reader, err := h.Hijack()
		if err != nil {
			s.fallbackRequest(request.Context(), writer, request, 0, E.Cause(err, "hijack conn"))
			return
		}
		if cacheLen := reader.Reader.Buffered(); cacheLen > 0 {
			cache := buf.NewSize(cacheLen)
			_, err = cache.ReadFullFrom(reader.Reader, cacheLen)
			if err != nil {
				s.fallbackRequest(request.Context(), writer, request, 0, E.Cause(err, "read cache"))
				return
			}
			conn = bufio.NewCachedConn(conn, cache)
		}
		if requestBody != nil {
			conn = bufio.NewCachedConn(conn, requestBody)
		}
		s.handler.NewConnection(request.Context(), conn, metadata)
	} else {
		writer.WriteHeader(http.StatusOK)
		conn := NewHTTP2Wrapper(&ServerHTTPConn{
			NewHTTPConn(request.Body, writer),
			writer.(http.Flusher),
		})
		s.handler.NewConnection(request.Context(), conn, metadata)
		conn.CloseWrapper()
	}
}

func (s *Server) fallbackRequest(ctx context.Context, writer http.ResponseWriter, request *http.Request, statusCode int, err error) {
	conn := NewHTTPConn(request.Body, writer)
	fErr := s.handler.FallbackConnection(ctx, &conn, M.Metadata{})
	if fErr == nil {
		return
	} else if fErr == os.ErrInvalid {
		fErr = nil
	}
	if statusCode > 0 {
		writer.WriteHeader(statusCode)
	}
	s.handler.NewError(request.Context(), E.Cause(E.Errors(err, E.Cause(fErr, "fallback connection")), "process connection from ", request.RemoteAddr))
}

func (s *Server) Serve(listener net.Listener) error {
	if s.tlsConfig != nil {
		if len(s.tlsConfig.NextProtos()) == 0 {
			s.tlsConfig.SetNextProtos([]string{http2.NextProtoTLS, "http/1.1"})
		} else if !common.Contains(s.tlsConfig.NextProtos(), http2.NextProtoTLS) {
			s.tlsConfig.SetNextProtos(append([]string{"h2"}, s.tlsConfig.NextProtos()...))
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
