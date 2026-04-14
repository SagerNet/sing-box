package proxybridge

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
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/service"
)

type Bridge struct {
	ctx           context.Context
	logger        logger.ContextLogger
	tag           string
	dialer        N.Dialer
	connection    adapter.ConnectionManager
	tcpListener   *net.TCPListener
	username      string
	password      string
	authenticator *auth.Authenticator
}

func New(ctx context.Context, logger logger.ContextLogger, tag string, dialer N.Dialer) (*Bridge, error) {
	username := randomHex(16)
	password := randomHex(16)
	tcpListener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return nil, err
	}
	bridge := &Bridge{
		ctx:           ctx,
		logger:        logger,
		tag:           tag,
		dialer:        dialer,
		connection:    service.FromContext[adapter.ConnectionManager](ctx),
		tcpListener:   tcpListener,
		username:      username,
		password:      password,
		authenticator: auth.NewAuthenticator([]auth.User{{Username: username, Password: password}}),
	}
	go bridge.acceptLoop()
	return bridge, nil
}

func randomHex(size int) string {
	raw := make([]byte, size)
	rand.Read(raw)
	return hex.EncodeToString(raw)
}

func (b *Bridge) Port() uint16 {
	return M.SocksaddrFromNet(b.tcpListener.Addr()).Port
}

func (b *Bridge) Username() string {
	return b.username
}

func (b *Bridge) Password() string {
	return b.password
}

func (b *Bridge) Close() error {
	return common.Close(b.tcpListener)
}

func (b *Bridge) acceptLoop() {
	for {
		tcpConn, err := b.tcpListener.AcceptTCP()
		if err != nil {
			return
		}
		ctx := log.ContextWithNewID(b.ctx)
		go func() {
			hErr := socks.HandleConnectionEx(ctx, tcpConn, std_bufio.NewReader(tcpConn), b.authenticator, b, nil, 0, M.SocksaddrFromNet(tcpConn.RemoteAddr()), nil)
			if hErr == nil {
				return
			}
			if E.IsClosedOrCanceled(hErr) {
				b.logger.DebugContext(ctx, E.Cause(hErr, b.tag, " connection closed"))
				return
			}
			b.logger.ErrorContext(ctx, E.Cause(hErr, b.tag))
		}()
	}
}

func (b *Bridge) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Source = source
	metadata.Destination = destination
	metadata.Network = N.NetworkTCP
	b.logger.InfoContext(ctx, b.tag, " connection to ", metadata.Destination)
	b.connection.NewConnection(ctx, b.dialer, conn, metadata, onClose)
}

func (b *Bridge) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, onClose N.CloseHandlerFunc) {
	var metadata adapter.InboundContext
	metadata.Source = source
	metadata.Destination = destination
	metadata.Network = N.NetworkUDP
	b.logger.InfoContext(ctx, b.tag, " packet connection to ", metadata.Destination)
	b.connection.NewPacketConnection(ctx, b.dialer, conn, metadata, onClose)
}
