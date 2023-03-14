package v2raywebsocket

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/common/tls"
	C "github.com/jobberrt/sing-box/constant"
	"github.com/jobberrt/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHttp "github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/websocket"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx                 context.Context
	handler             N.TCPConnectionHandler
	errorHandler        E.Handler
	httpServer          *http.Server
	path                string
	maxEarlyData        uint32
	earlyDataHeaderName string
}

func NewServer(ctx context.Context, options option.V2RayWebsocketOptions, tlsConfig tls.ServerConfig, handler N.TCPConnectionHandler, errorHandler E.Handler) (*Server, error) {
	server := &Server{
		ctx:                 ctx,
		handler:             handler,
		errorHandler:        errorHandler,
		path:                options.Path,
		maxEarlyData:        options.MaxEarlyData,
		earlyDataHeaderName: options.EarlyDataHeaderName,
	}
	if !strings.HasPrefix(server.path, "/") {
		server.path = "/" + server.path
	}
	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: C.TCPTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
	}
	if tlsConfig != nil {
		stdConfig, err := tlsConfig.Config()
		if err != nil {
			return nil, err
		}
		server.httpServer.TLSConfig = stdConfig
	}
	return server, nil
}

var upgrader = websocket.Upgrader{
	HandshakeTimeout: C.TCPTimeout,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if s.maxEarlyData == 0 || s.earlyDataHeaderName != "" {
		if request.URL.Path != s.path {
			writer.WriteHeader(http.StatusNotFound)
			s.badRequest(request, E.New("bad path: ", request.URL.Path))
			return
		}
	}
	var (
		earlyData []byte
		err       error
		conn      net.Conn
	)
	if s.earlyDataHeaderName == "" {
		if strings.HasPrefix(request.URL.RequestURI(), s.path) {
			earlyDataStr := request.URL.RequestURI()[len(s.path):]
			earlyData, err = base64.RawURLEncoding.DecodeString(earlyDataStr)
		} else {
			writer.WriteHeader(http.StatusNotFound)
			s.badRequest(request, E.New("bad path: ", request.URL.Path))
			return
		}
	} else {
		earlyDataStr := request.Header.Get(s.earlyDataHeaderName)
		if earlyDataStr != "" {
			earlyData, err = base64.RawURLEncoding.DecodeString(earlyDataStr)
		}
	}
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		s.badRequest(request, E.Cause(err, "decode early data"))
		return
	}
	wsConn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		s.badRequest(request, E.Cause(err, "upgrade websocket connection"))
		return
	}
	var metadata M.Metadata
	metadata.Source = sHttp.SourceAddress(request)
	conn = NewServerConn(wsConn, metadata.Source.TCPAddr())
	if len(earlyData) > 0 {
		conn = bufio.NewCachedConn(conn, buf.As(earlyData))
	}
	s.handler.NewConnection(request.Context(), conn, metadata)
}

func (s *Server) badRequest(request *http.Request, err error) {
	s.errorHandler.NewError(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
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
