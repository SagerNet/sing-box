package outbound

import (
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
)

type ProxyListener struct {
	ctx           context.Context
	logger        log.ContextLogger
	dialer        N.Dialer
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

// TODO: migrate to new api
//
//nolint:staticcheck
func (l *ProxyListener) accept(ctx context.Context, conn *net.TCPConn) error {
	return socks.HandleConnection(ctx, conn, l.authenticator, l, M.Metadata{})
}

func (l *ProxyListener) NewConnection(ctx context.Context, conn net.Conn, upstreamMetadata M.Metadata) error {
	var metadata adapter.InboundContext
	metadata.Network = N.NetworkTCP
	metadata.Destination = upstreamMetadata.Destination
	l.logger.InfoContext(ctx, "proxy connection to ", metadata.Destination)
	return NewConnection(ctx, l.dialer, conn, metadata)
}

func (l *ProxyListener) NewPacketConnection(ctx context.Context, conn N.PacketConn, upstreamMetadata M.Metadata) error {
	var metadata adapter.InboundContext
	metadata.Network = N.NetworkUDP
	metadata.Destination = upstreamMetadata.Destination
	l.logger.InfoContext(ctx, "proxy packet connection to ", metadata.Destination)
	return NewPacketConnection(ctx, l.dialer, conn, metadata)
}
