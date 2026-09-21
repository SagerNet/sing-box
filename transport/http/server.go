package http

import (
	"context"
	"io"
	"math"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

const (
	maxHeaderBytes      = 1 << 20
	idleTimeout         = 60 * time.Second
	maxDiscardBodyBytes = 256 << 10
	discardBodyTimeout  = 5 * time.Second
	realm               = "sing-box"
)

var ConfigureHTTP3ListenerFunc func(ctx context.Context, logger logger.Logger, listener *listener.Listener, handler http.Handler, tlsConfig tls.ServerConfig, options option.QUICOptions) (io.Closer, error)

type Handler interface {
	N.TCPConnectionHandlerEx
	N.UDPConnectionHandlerEx
}

type ServerOptions struct {
	Authenticator *auth.Authenticator
	Logger        logger.ContextLogger
	HTTP1         bool
	HTTP2         bool
	HTTP2Options  option.HTTP2Options
	UDP           bool
	Tunnels       map[string]TunnelHandler
}

type Server struct {
	authenticator *auth.Authenticator
	logger        logger.ContextLogger
	http1         bool
	http2Server   *http2.Server
	udp           bool
	tunnels       map[string]TunnelHandler
}

func NewServer(options ServerOptions) *Server {
	server := &Server{
		authenticator: options.Authenticator,
		logger:        options.Logger,
		http1:         options.HTTP1,
		udp:           options.UDP,
		tunnels:       options.Tunnels,
	}
	if options.HTTP2 {
		server.http2Server = &http2.Server{
			IdleTimeout:                  idleTimeout,
			ReadIdleTimeout:              time.Duration(options.HTTP2Options.KeepAlivePeriod),
			PingTimeout:                  time.Duration(options.HTTP2Options.IdleTimeout),
			MaxConcurrentStreams:         uint32(max(options.HTTP2Options.MaxConcurrentStreams, 0)),
			MaxUploadBufferPerConnection: int32(min(options.HTTP2Options.ConnectionReceiveWindow.Value(), math.MaxInt32)),
			MaxUploadBufferPerStream:     int32(min(options.HTTP2Options.StreamReceiveWindow.Value(), math.MaxInt32)),
		}
		if options.HTTP2Options.IdleTimeout > 0 {
			server.http2Server.IdleTimeout = time.Duration(options.HTTP2Options.IdleTimeout)
		}
	}
	return server
}

func (s *Server) ServeConnection(ctx context.Context, conn net.Conn, reader *Reader, handler Handler, source M.Socksaddr, onClose N.CloseHandlerFunc) {
	if s.http2Server != nil {
		conn.SetReadDeadline(time.Now().Add(idleTimeout))
		isHTTP2, err := reader.isHTTP2Preface()
		conn.SetReadDeadline(time.Time{})
		if err != nil {
			s.finishConnection(ctx, conn, source, onClose, E.Cause(err, "peek request"))
			return
		}
		if isHTTP2 {
			s.serveHTTP2(ctx, conn, reader, handler, source, onClose)
			return
		}
	}
	if !s.http1 {
		_, err := conn.Write([]byte("HTTP/1.1 505 HTTP Version Not Supported\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"))
		s.finishConnection(ctx, conn, source, onClose, err)
		return
	}
	connection := &serverConn{
		server:  s,
		ctx:     ctx,
		conn:    conn,
		reader:  reader,
		handler: handler,
		source:  source,
		onClose: onClose,
	}
	connection.serve()
}

func (s *Server) ConfigureTLS(tlsConfig tls.ServerConfig) {
	if len(tlsConfig.NextProtos()) > 0 {
		return
	}
	var nextProtos []string
	if s.http2Server != nil {
		nextProtos = append(nextProtos, http2.NextProtoTLS)
	}
	if s.http1 {
		nextProtos = append(nextProtos, "http/1.1")
	}
	tlsConfig.SetNextProtos(nextProtos)
}

func (s *Server) ListenHTTP3(ctx context.Context, logger logger.Logger, listener *listener.Listener, handler Handler, tlsConfig tls.ServerConfig, options option.QUICOptions) (io.Closer, error) {
	if ConfigureHTTP3ListenerFunc == nil {
		return nil, C.ErrQUICNotIncluded
	}
	if !slices.Contains(tlsConfig.NextProtos(), "h3") {
		tlsConfig.SetNextProtos(append([]string{"h3"}, tlsConfig.NextProtos()...))
	}
	return ConfigureHTTP3ListenerFunc(ctx, logger, listener, &httpHandler{
		server:  s,
		handler: handler,
	}, tlsConfig, options)
}

func (s *Server) finishConnection(ctx context.Context, conn net.Conn, source M.Socksaddr, onClose N.CloseHandlerFunc, err error) {
	conn.Close()
	if err != nil {
		if E.IsClosedOrCanceled(err) || E.IsTimeout(err) {
			s.logger.DebugContext(ctx, "connection closed: ", err)
			err = nil
		} else {
			s.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", source))
		}
	}
	if onClose != nil {
		onClose(err)
	}
}
