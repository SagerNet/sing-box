package v2rayhttpupgrade

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	sHttp "github.com/sagernet/sing/protocol/http"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx        context.Context
	tlsConfig  tls.ServerConfig
	handler    adapter.V2RayServerTransportHandler
	httpServer *http.Server
	host       string
	path       string
	headers    http.Header
}

func NewServer(ctx context.Context, options option.V2RayHTTPUpgradeOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	server := &Server{
		ctx:       ctx,
		tlsConfig: tlsConfig,
		handler:   handler,
		host:      options.Host,
		path:      options.Path,
		headers:   options.Headers.Build(),
	}
	if !strings.HasPrefix(server.path, "/") {
		server.path = "/" + server.path
	}
	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: C.TCPTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		TLSNextProto: make(map[string]func(*http.Server, *tls.STDConn, http.Handler)),
	}
	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	host := request.Host
	if len(s.host) > 0 && host != s.host {
		s.invalidRequest(writer, request, http.StatusBadRequest, E.New("bad host: ", host))
		return
	}
	if !strings.HasPrefix(request.URL.Path, s.path) {
		s.invalidRequest(writer, request, http.StatusNotFound, E.New("bad path: ", request.URL.Path))
		return
	}
	if request.Method != http.MethodGet {
		s.invalidRequest(writer, request, http.StatusNotFound, E.New("bad method: ", request.Method))
		return
	}
	if !strings.EqualFold(request.Header.Get("Connection"), "upgrade") {
		s.invalidRequest(writer, request, http.StatusNotFound, E.New("not a upgrade request"))
		return
	}
	if !strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
		s.invalidRequest(writer, request, http.StatusNotFound, E.New("not a websocket request"))
		return
	}
	if request.Header.Get("Sec-WebSocket-Key") != "" {
		s.invalidRequest(writer, request, http.StatusNotFound, E.New("real websocket request received"))
		return
	}
	hijacker, canHijack := writer.(http.Hijacker)
	if !canHijack {
		s.invalidRequest(writer, request, http.StatusInternalServerError, E.New("invalid connection, maybe HTTP/2"))
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		s.invalidRequest(writer, request, http.StatusInternalServerError, E.Cause(err, "hijack failed"))
		return
	}
	response := &http.Response{
		StatusCode: 101,
		Header:     s.headers.Clone(),
	}
	response.Header.Set("Connection", "upgrade")
	response.Header.Set("Upgrade", "websocket")
	err = response.Write(conn)
	if err != nil {
		s.invalidRequest(writer, request, http.StatusInternalServerError, E.Cause(err, "write response failed"))
		return
	}
	var metadata M.Metadata
	metadata.Source = sHttp.SourceAddress(request)
	s.handler.NewConnection(request.Context(), conn, metadata)
}

func (s *Server) invalidRequest(writer http.ResponseWriter, request *http.Request, statusCode int, err error) {
	if statusCode > 0 {
		writer.WriteHeader(statusCode)
	}
	s.handler.NewError(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func (s *Server) Serve(listener net.Listener) error {
	if s.tlsConfig != nil {
		if len(s.tlsConfig.NextProtos()) == 0 {
			s.tlsConfig.SetNextProtos([]string{"http/1.1"})
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
