package v2raywebsocket

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2rayhttp"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	sHttp "github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/ws"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx                 context.Context
	logger              logger.ContextLogger
	tlsConfig           tls.ServerConfig
	handler             adapter.V2RayServerTransportHandler
	httpServer          *http.Server
	path                string
	maxEarlyData        uint32
	earlyDataHeaderName string
	upgrader            ws.HTTPUpgrader
}

func NewServer(ctx context.Context, logger logger.ContextLogger, options option.V2RayWebsocketOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	server := &Server{
		ctx:                 ctx,
		logger:              logger,
		tlsConfig:           tlsConfig,
		handler:             handler,
		path:                options.Path,
		maxEarlyData:        options.MaxEarlyData,
		earlyDataHeaderName: options.EarlyDataHeaderName,
		upgrader: ws.HTTPUpgrader{
			Timeout: C.TCPTimeout,
			Header:  options.Headers.Build(),
		},
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
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return log.ContextWithNewID(ctx)
		},
	}
	return server, nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if s.maxEarlyData == 0 || s.earlyDataHeaderName != "" {
		if request.URL.Path != s.path {
			s.invalidRequest(writer, request, http.StatusNotFound, E.New("bad path: ", request.URL.Path))
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
			s.invalidRequest(writer, request, http.StatusNotFound, E.New("bad path: ", request.URL.Path))
			return
		}
	} else {
		if request.URL.Path != s.path {
			s.invalidRequest(writer, request, http.StatusNotFound, E.New("bad path: ", request.URL.Path))
			return
		}
		earlyDataStr := request.Header.Get(s.earlyDataHeaderName)
		if earlyDataStr != "" {
			earlyData, err = base64.RawURLEncoding.DecodeString(earlyDataStr)
		}
	}
	if err != nil {
		s.invalidRequest(writer, request, http.StatusBadRequest, E.Cause(err, "decode early data"))
		return
	}
	wsConn, _, _, err := ws.UpgradeHTTP(request, writer)
	if err != nil {
		s.invalidRequest(writer, request, 0, E.Cause(err, "upgrade websocket connection"))
		return
	}
	source := sHttp.SourceAddress(request)
	conn = NewConn(wsConn, source, ws.StateServerSide)
	if len(earlyData) > 0 {
		conn = bufio.NewCachedConn(conn, buf.As(earlyData))
	}
	s.handler.NewConnectionEx(v2rayhttp.DupContext(request.Context()), conn, source, M.Socksaddr{}, nil)
}

func (s *Server) invalidRequest(writer http.ResponseWriter, request *http.Request, statusCode int, err error) {
	if statusCode > 0 {
		writer.WriteHeader(statusCode)
	}
	s.logger.ErrorContext(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func (s *Server) Serve(listener net.Listener) error {
	if s.tlsConfig != nil {
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
