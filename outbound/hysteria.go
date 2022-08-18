//go:build with_quic

package outbound

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/hysteria"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/lucas-clemente/quic-go"
)

const (
	hyMbpsToBps                      = 125000
	hyMinSpeedBPS                    = 16384
	hyDefaultStreamReceiveWindow     = 15728640 // 15 MB/s
	hyDefaultConnectionReceiveWindow = 67108864 // 64 MB/s
	hyDefaultMaxIncomingStreams      = 1024
	hyDefaultALPN                    = "hysteria"
	hyKeepAlivePeriod                = 10 * time.Second
)

var _ adapter.Outbound = (*Hysteria)(nil)

type Hysteria struct {
	myOutboundAdapter
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr
	tlsConfig  *tls.Config
	quicConfig *quic.Config
	authKey    []byte
	xplusKey   []byte
	sendBPS    uint64
	recvBPS    uint64
	connAccess sync.Mutex
	conn       quic.Connection
}

func NewHysteria(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaOutboundOptions) (*Hysteria, error) {
	tlsConfig := &tls.Config{
		ServerName:         options.ServerName,
		InsecureSkipVerify: options.Insecure,
		MinVersion:         tls.VersionTLS13,
	}
	if options.ALPN != "" {
		tlsConfig.NextProtos = []string{options.ALPN}
	} else {
		tlsConfig.NextProtos = []string{hyDefaultALPN}
	}
	var ca []byte
	var err error
	if options.CustomCA != "" {
		ca, err = os.ReadFile(options.CustomCA)
		if err != nil {
			return nil, err
		}
	}
	if options.CustomCAStr != "" {
		ca = []byte(options.CustomCAStr)
	}
	if len(ca) > 0 {
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(ca) {
			return nil, E.New("parse ca failed")
		}
		tlsConfig.RootCAs = cp
	}
	quicConfig := &quic.Config{
		InitialStreamReceiveWindow:     options.ReceiveWindowConn,
		MaxStreamReceiveWindow:         options.ReceiveWindowConn,
		InitialConnectionReceiveWindow: options.ReceiveWindow,
		MaxConnectionReceiveWindow:     options.ReceiveWindow,
		KeepAlivePeriod:                hyKeepAlivePeriod,
		DisablePathMTUDiscovery:        options.DisableMTUDiscovery,
		EnableDatagrams:                true,
	}
	if options.ReceiveWindowConn == 0 {
		quicConfig.InitialStreamReceiveWindow = hyDefaultStreamReceiveWindow
		quicConfig.MaxStreamReceiveWindow = hyDefaultStreamReceiveWindow
	}
	if options.ReceiveWindow == 0 {
		quicConfig.InitialConnectionReceiveWindow = hyDefaultConnectionReceiveWindow
		quicConfig.MaxConnectionReceiveWindow = hyDefaultConnectionReceiveWindow
	}
	if quicConfig.MaxIncomingStreams == 0 {
		quicConfig.MaxIncomingStreams = hyDefaultMaxIncomingStreams
	}
	var auth []byte
	if len(options.Auth) > 0 {
		auth = options.Auth
	} else {
		auth = []byte(options.AuthString)
	}
	var xplus []byte
	if options.Obfs != "" {
		xplus = []byte(options.Obfs)
	}
	var up, down uint64
	if len(options.Up) > 0 {
		up = hysteria.StringToBps(options.Up)
		if up == 0 {
			return nil, E.New("invalid up speed format: ", options.Up)
		}
	} else {
		up = uint64(options.UpMbps) * hyMbpsToBps
	}
	if len(options.Down) > 0 {
		down = hysteria.StringToBps(options.Down)
		if down == 0 {
			return nil, E.New("invalid down speed format: ", options.Down)
		}
	} else {
		down = uint64(options.DownMbps) * hyMbpsToBps
	}
	if up < hyMinSpeedBPS {
		return nil, E.New("invalid up speed")
	}
	if down < hyMinSpeedBPS {
		return nil, E.New("invalid down speed")
	}
	return &Hysteria{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeHysteria,
			network:  options.Network.Build(),
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		ctx:        ctx,
		dialer:     dialer.NewOutbound(router, options.OutboundDialerOptions),
		serverAddr: options.ServerOptions.Build(),
		tlsConfig:  tlsConfig,
		quicConfig: quicConfig,
		authKey:    auth,
		xplusKey:   xplus,
		sendBPS:    up,
		recvBPS:    down,
	}, nil
}

func (h *Hysteria) offer(ctx context.Context) (quic.Connection, error) {
	conn := h.conn
	if conn != nil && !common.Done(conn.Context()) {
		return conn, nil
	}
	h.connAccess.Lock()
	defer h.connAccess.Unlock()
	conn = h.conn
	if conn != nil && !common.Done(conn.Context()) {
		return conn, nil
	}
	conn, err := h.offerNew(ctx)
	if err != nil {
		return nil, err
	}
	h.conn = conn
	return conn, nil
}

func (h *Hysteria) offerNew(ctx context.Context) (quic.Connection, error) {
	udpConn, err := h.dialer.DialContext(h.ctx, "udp", h.serverAddr)
	if err != nil {
		return nil, err
	}
	var packetConn net.PacketConn
	packetConn = bufio.NewUnbindPacketConn(udpConn)
	if h.xplusKey != nil {
		packetConn = hysteria.NewXPlusPacketConn(packetConn, h.xplusKey)
	}
	packetConn = &hysteria.WrapPacketConn{PacketConn: packetConn}
	quicConn, err := quic.Dial(packetConn, udpConn.RemoteAddr(), h.serverAddr.AddrString(), h.tlsConfig, h.quicConfig)
	if err != nil {
		packetConn.Close()
		return nil, err
	}
	controlStream, err := quicConn.OpenStreamSync(ctx)
	if err != nil {
		packetConn.Close()
		return nil, err
	}
	err = hysteria.WriteClientHello(controlStream, hysteria.ClientHello{
		SendBPS: h.sendBPS,
		RecvBPS: h.recvBPS,
		Auth:    h.authKey,
	})
	if err != nil {
		return nil, E.Cause(err, "write hysteria client hello")
	}
	serverHello, err := hysteria.ReadServerHello(controlStream)
	if err != nil {
		return nil, err
	}
	if !serverHello.OK {
		return nil, E.New("remote error: ", serverHello.Message)
	}
	// TODO: set congestion control
	return quicConn, nil
}

func (h *Hysteria) Close() error {
	h.connAccess.Lock()
	defer h.connAccess.Unlock()
	if h.conn != nil {
		h.conn.CloseWithError(0, "")
	}
	return nil
}

func (h *Hysteria) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	conn, err := h.offer(ctx)
	if err != nil {
		return nil, err
	}
	stream, err := conn.OpenStream()
	if err != nil {
		return nil, err
	}
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		return hysteria.NewClientConn(stream, destination), nil
	default:
		return nil, E.New("unsupported network: ", network)
	}
}

func (h *Hysteria) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (h *Hysteria) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *Hysteria) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}
