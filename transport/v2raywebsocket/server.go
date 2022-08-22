package v2raywebsocket

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/gorilla/websocket"
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

func NewServer(ctx context.Context, options option.V2RayWebsocketOptions, tlsConfig *tls.Config, handler N.TCPConnectionHandler, errorHandler E.Handler) *Server {
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
		TLSConfig:         tlsConfig,
	}
	return server
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
	var remoteAddr net.Addr
	forwardFrom := request.Header.Get("X-Forwarded-For")
	if forwardFrom != "" {
		for _, from := range strings.Split(forwardFrom, ",") {
			originAddr, err := netip.ParseAddr(from)
			if err == nil {
				remoteAddr = M.SocksaddrFrom(originAddr, 0).TCPAddr()
				break
			}
		}
	}
	conn = &WebsocketConn{
		Conn:       wsConn,
		remoteAddr: remoteAddr,
	}
	if len(earlyData) > 0 {
		conn = bufio.NewCachedConn(conn, buf.As(earlyData))
	}
	s.handler.NewConnection(request.Context(), conn, M.Metadata{})
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
