package wsc

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/sagernet/ws"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing/common/auth"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.WSCInboundOptions](registry, C.TypeWSC, NewInbound)
}

type Inbound struct {
	inbound.Adapter

	router    adapter.ConnectionRouterEx
	logger    logger.ContextLogger
	listener  *listener.Listener
	users     map[string]bool
	tlsConfig tls.ServerConfig
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.WSCInboundOptions) (adapter.Inbound, error) {
	ib := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeWSC, tag),
		router:  uot.NewRouter(router, logger),
		logger:  logger,
		users:   map[string]bool{},
	}

	for _, user := range options.Users {
		_, ok := ib.users[user.Auth]
		if !ok {
			ib.users[user.Auth] = true
		} else {
			return nil, fmt.Errorf("user already exists: %s", user.Auth)
		}
	}

	var err error
	if options.TLS != nil {
		ib.tlsConfig, err = tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
	}

	ib.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           []string{N.NetworkTCP},
		Listen:            options.ListenOptions,
		ConnectionHandler: ib,
	})
	return ib, nil
}

func (in *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if in.tlsConfig != nil {
		if err := in.tlsConfig.Start(); err != nil {
			return E.Cause(err, "create TLS config")
		}
	}
	return in.listener.Start()
}

func (in *Inbound) Close() error {
	return common.Close(in.listener, in.tlsConfig)
}

func (in *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if in.tlsConfig != nil {
		tlsConn, err := tls.ServerHandshake(ctx, conn, in.tlsConfig)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			in.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source, ": TLS handshake"))
			return
		}
		conn = tlsConn
	}

	var requestURI string
	upgrader := ws.Upgrader{
		OnRequest: func(uri []byte) error {
			requestURI = string(uri)
			return nil
		},
	}

	brw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	if _, err := upgrader.Upgrade(brw); err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, E.Cause(err, "websocket upgrade"))
		return
	}
	if err := brw.Flush(); err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, E.Cause(err, "flush handshake"))
		return
	}

	uri, err := url.ParseRequestURI(requestURI)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, E.Cause(err, "parse request uri"))
		return
	}

	query := uri.Query()
	user := query.Get("user")
	network := query.Get("net")
	addr := query.Get("addr")

	if _, ok := in.users[user]; !ok {
		N.CloseOnHandshakeFailure(conn, onClose, E.New("unauthorized user"))
		return
	}

	if network != "" && network != "tcp" && network != N.NetworkTCP {
		N.CloseOnHandshakeFailure(conn, onClose, E.New("only net=tcp supported"))
		return
	}

	destination, err := parseSocksAddr(addr)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, E.Cause(err, "bad addr"))
		return
	}
	metadata.Destination = destination

	wsConn := newWSStreamConn(conn, true)

	if user != "" {
		ctx = auth.ContextWithUser(ctx, user)
	}

	in.router.RouteConnectionEx(ctx, wsConn, metadata, onClose)
}

func parseSocksAddr(addr string) (M.Socksaddr, error) {
	if addr == "" {
		return M.Socksaddr{}, E.New("empty addr")
	}

	raw := addr
	if !strings.Contains(raw, "://") {
		raw = "tcp://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return M.Socksaddr{}, err
	}

	host := u.Hostname()
	portStr := u.Port()
	if host == "" || portStr == "" {
		return M.Socksaddr{}, E.New("missing host or port")
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return M.Socksaddr{}, err
	}

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return M.Socksaddr{Fqdn: host, Port: uint16(port)}, nil
	}

	return M.Socksaddr{Addr: ip, Port: uint16(port)}, nil
}
