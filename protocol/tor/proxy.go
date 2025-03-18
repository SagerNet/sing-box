package tor

import (
	std_bufio "bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/service"
)

type ProxyListener struct {
	ctx           context.Context
	logger        log.ContextLogger
	dialer        N.Dialer
	connection    adapter.ConnectionManager
	tcpListener   *net.TCPListener
	username      string
	password      string
	authenticator *auth.Authenticator
}

func NewProxyListener(ctx context.Context, logger log.ContextLogger, dialer N.Dialer) *ProxyListener {
	var usernameB [64]byte
	var passwordB [64]byte
	rand.Read(usernameB[:])
	rand.Read(passwordB[:])
	username := hex.EncodeToString(usernameB[:])
	password := hex.EncodeToString(passwordB[:])
	return &ProxyListener{
		ctx:           ctx,
		logger:        logger,
		dialer:        dialer,
		connection:    service.FromContext[adapter.ConnectionManager](ctx),
		authenticator: auth.NewAuthenticator([]auth.User{{Username: username, Password: password}}),
		username:      username,
		password:      password,
	}
}

func (l *ProxyListener) Start() error {
	tcpListener, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP: net.IPv4(127, 0, 0, 1),
	})
	if err != nil {
		return err
	}
	l.tcpListener = tcpListener
	go l.acceptLoop()
	return nil
}

func (l *ProxyListener) Port() uint16 {
	if l.tcpListener == nil {
		panic("start listener first")
	}
	return M.SocksaddrFromNet(l.tcpListener.Addr()).Port
}

func (l *ProxyListener) Username() string {
	return l.username
}

func (l *ProxyListener) Password() string {
	return l.password
}

func (l *ProxyListener) Close() error {
	return common.Close(l.tcpListener)
}

func (l *ProxyListener) acceptLoop() {
	for {
		tcpConn, err := l.tcpListener.AcceptTCP()
		if err != nil {
			return
		}
		ctx := log.ContextWithNewID(l.ctx)
		go func() {
			hErr := l.accept(ctx, tcpConn)
			if hErr != nil {
				if E.IsClosedOrCanceled(hErr) {
					l.logger.DebugContext(ctx, E.Cause(hErr, "proxy connection closed"))
					return
				}
				l.logger.ErrorContext(ctx, E.Cause(hErr, "proxy"))
			}
		}()
	}
}

func (l *ProxyListener) accept(ctx context.Context, conn *net.TCPConn) error {
	return socks.HandleConnectionEx(ctx, conn, std_bufio.NewReader(conn), l.authenticator, l, nil, M.SocksaddrFromNet(conn.RemoteAddr()), nil)
}

func (l *ProxyListener) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Source = source
	metadata.Destination = destination
	metadata.Network = N.NetworkTCP
	l.logger.InfoContext(ctx, "proxy connection to ", metadata.Destination)
	l.connection.NewConnection(ctx, l.dialer, conn, metadata, onClose)
}

func (l *ProxyListener) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Source = source
	metadata.Destination = destination
	metadata.Network = N.NetworkUDP
	l.logger.InfoContext(ctx, "proxy packet connection to ", metadata.Destination)
	l.connection.NewPacketConnection(ctx, l.dialer, conn, metadata, onClose)
}
