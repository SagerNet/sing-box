package naive

import (
	"context"
	"io"
	"math/rand"
	"net"
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2rayhttp"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHttp "github.com/sagernet/sing/protocol/http"
)

var ConfigureHTTP3ListenerFunc func(listener *listener.Listener, handler http.Handler, tlsConfig tls.ServerConfig, logger logger.Logger) (io.Closer, error)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.NaiveInboundOptions](registry, C.TypeNaive, NewInbound)
}

type Inbound struct {
	inbound.Adapter
	ctx              context.Context
	router           adapter.ConnectionRouterEx
	logger           logger.ContextLogger
	listener         *listener.Listener
	network          []string
	networkIsDefault bool
	authenticator    *auth.Authenticator
	tlsConfig        tls.ServerConfig
	httpServer       *http.Server
	h3Server         io.Closer
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.NaiveInboundOptions) (adapter.Inbound, error) {
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeNaive, tag),
		ctx:     ctx,
		router:  uot.NewRouter(router, logger),
		logger:  logger,
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Listen:  options.ListenOptions,
		}),
		networkIsDefault: options.Network == "",
		network:          options.Network.Build(),
		authenticator:    auth.NewAuthenticator(options.Users),
	}
	if common.Contains(inbound.network, N.NetworkUDP) {
		if options.TLS == nil || !options.TLS.Enabled {
			return nil, E.New("TLS is required for QUIC server")
		}
	}
	if len(options.Users) == 0 {
		return nil, E.New("missing users")
	}
	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		inbound.tlsConfig = tlsConfig
	}
	return inbound, nil
}

func (n *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	var tlsConfig *tls.STDConfig
	if n.tlsConfig != nil {
		err := n.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
		tlsConfig, err = n.tlsConfig.STDConfig()
		if err != nil {
			return err
		}
	}
	if common.Contains(n.network, N.NetworkTCP) {
		tcpListener, err := n.listener.ListenTCP()
		if err != nil {
			return err
		}
		n.httpServer = &http.Server{
			Handler:   n,
			TLSConfig: tlsConfig,
			BaseContext: func(listener net.Listener) context.Context {
				return n.ctx
			},
		}
		go func() {
			var sErr error
			if tlsConfig != nil {
				sErr = n.httpServer.ServeTLS(tcpListener, "", "")
			} else {
				sErr = n.httpServer.Serve(tcpListener)
			}
			if sErr != nil && !E.IsClosedOrCanceled(sErr) {
				n.logger.Error("http server serve error: ", sErr)
			}
		}()
	}

	if common.Contains(n.network, N.NetworkUDP) {
		http3Server, err := ConfigureHTTP3ListenerFunc(n.listener, n, n.tlsConfig, n.logger)
		if err == nil {
			n.h3Server = http3Server
		} else if len(n.network) > 1 {
			n.logger.Warn(E.Cause(err, "naive http3 disabled"))
		} else {
			return err
		}
	}

	return nil
}

func (n *Inbound) Close() error {
	return common.Close(
		&n.listener,
		common.PtrOrNil(n.httpServer),
		n.h3Server,
		n.tlsConfig,
	)
}

func (n *Inbound) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := log.ContextWithNewID(request.Context())
	if request.Method != "CONNECT" {
		rejectHTTP(writer, http.StatusBadRequest)
		n.badRequest(ctx, request, E.New("not CONNECT request"))
		return
	} else if request.Header.Get("Padding") == "" {
		rejectHTTP(writer, http.StatusBadRequest)
		n.badRequest(ctx, request, E.New("missing naive padding"))
		return
	}
	userName, password, authOk := sHttp.ParseBasicAuth(request.Header.Get("Proxy-Authorization"))
	if authOk {
		authOk = n.authenticator.Verify(userName, password)
	}
	if !authOk {
		rejectHTTP(writer, http.StatusProxyAuthRequired)
		n.badRequest(ctx, request, E.New("authorization failed"))
		return
	}
	writer.Header().Set("Padding", generateNaivePaddingHeader())
	writer.WriteHeader(http.StatusOK)
	writer.(http.Flusher).Flush()

	hostPort := request.URL.Host
	if hostPort == "" {
		hostPort = request.Host
	}
	source := sHttp.SourceAddress(request)
	destination := M.ParseSocksaddr(hostPort).Unwrap()

	if hijacker, isHijacker := writer.(http.Hijacker); isHijacker {
		conn, _, err := hijacker.Hijack()
		if err != nil {
			n.badRequest(ctx, request, E.New("hijack failed"))
			return
		}
		n.newConnection(ctx, false, &naiveH1Conn{Conn: conn}, userName, source, destination)
	} else {
		n.newConnection(ctx, true, &naiveH2Conn{reader: request.Body, writer: writer, flusher: writer.(http.Flusher)}, userName, source, destination)
	}
}

func (n *Inbound) newConnection(ctx context.Context, waitForClose bool, conn net.Conn, userName string, source M.Socksaddr, destination M.Socksaddr) {
	if userName != "" {
		n.logger.InfoContext(ctx, "[", userName, "] inbound connection from ", source)
		n.logger.InfoContext(ctx, "[", userName, "] inbound connection to ", destination)
	} else {
		n.logger.InfoContext(ctx, "inbound connection from ", source)
		n.logger.InfoContext(ctx, "inbound connection to ", destination)
	}
	var metadata adapter.InboundContext
	metadata.Inbound = n.Tag()
	metadata.InboundType = n.Type()
	//nolint:staticcheck
	metadata.InboundDetour = n.listener.ListenOptions().Detour
	//nolint:staticcheck
	metadata.InboundOptions = n.listener.ListenOptions().InboundOptions
	metadata.Source = source
	metadata.Destination = destination
	metadata.OriginDestination = M.SocksaddrFromNet(conn.LocalAddr()).Unwrap()
	metadata.User = userName
	if !waitForClose {
		n.router.RouteConnectionEx(ctx, conn, metadata, nil)
	} else {
		done := make(chan struct{})
		wrapper := v2rayhttp.NewHTTP2Wrapper(conn)
		n.router.RouteConnectionEx(ctx, conn, metadata, N.OnceClose(func(it error) {
			close(done)
		}))
		<-done
		wrapper.CloseWrapper()
	}
}

func (n *Inbound) badRequest(ctx context.Context, request *http.Request, err error) {
	n.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", request.RemoteAddr))
}

func rejectHTTP(writer http.ResponseWriter, statusCode int) {
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		writer.WriteHeader(statusCode)
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		writer.WriteHeader(statusCode)
		return
	}
	if tcpConn, isTCP := common.Cast[*net.TCPConn](conn); isTCP {
		tcpConn.SetLinger(0)
	}
	conn.Close()
}

func generateNaivePaddingHeader() string {
	paddingLen := rand.Intn(32) + 30
	padding := make([]byte, paddingLen)
	bits := rand.Uint64()
	for i := 0; i < 16; i++ {
		// Codes that won't be Huffman coded.
		padding[i] = "!#$()+<>?@[]^`{}"[bits&15]
		bits >>= 4
	}
	for i := 16; i < paddingLen; i++ {
		padding[i] = '~'
	}
	return string(padding)
}
