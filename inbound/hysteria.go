//go:build with_quic

package inbound

import (
	"bytes"
	"context"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/hysteria"
	dns "github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/lucas-clemente/quic-go"
)

var _ adapter.Inbound = (*Hysteria)(nil)

type Hysteria struct {
	ctx           context.Context
	router        adapter.Router
	logger        log.ContextLogger
	tag           string
	listenOptions option.ListenOptions
	quicConfig    *quic.Config
	tlsConfig     *TLSConfig
	authKey       []byte
	xplusKey      []byte
	sendBPS       uint64
	recvBPS       uint64
	listener      quic.Listener
}

func NewHysteria(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaInboundOptions) (*Hysteria, error) {
	quicConfig := &quic.Config{
		InitialStreamReceiveWindow:     options.ReceiveWindowConn,
		MaxStreamReceiveWindow:         options.ReceiveWindowConn,
		InitialConnectionReceiveWindow: options.ReceiveWindowClient,
		MaxConnectionReceiveWindow:     options.ReceiveWindowClient,
		MaxIncomingStreams:             int64(options.MaxConnClient),
		KeepAlivePeriod:                hysteria.KeepAlivePeriod,
		DisablePathMTUDiscovery:        options.DisableMTUDiscovery,
		EnableDatagrams:                true,
	}
	if options.ReceiveWindowConn == 0 {
		quicConfig.InitialStreamReceiveWindow = hysteria.DefaultStreamReceiveWindow
		quicConfig.MaxStreamReceiveWindow = hysteria.DefaultStreamReceiveWindow
	}
	if options.ReceiveWindowClient == 0 {
		quicConfig.InitialConnectionReceiveWindow = hysteria.DefaultConnectionReceiveWindow
		quicConfig.MaxConnectionReceiveWindow = hysteria.DefaultConnectionReceiveWindow
	}
	if quicConfig.MaxIncomingStreams == 0 {
		quicConfig.MaxIncomingStreams = hysteria.DefaultMaxIncomingStreams
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
		up = uint64(options.UpMbps) * hysteria.MbpsToBps
	}
	if len(options.Down) > 0 {
		down = hysteria.StringToBps(options.Down)
		if down == 0 {
			return nil, E.New("invalid down speed format: ", options.Down)
		}
	} else {
		down = uint64(options.DownMbps) * hysteria.MbpsToBps
	}
	if up < hysteria.MinSpeedBPS {
		return nil, E.New("invalid up speed")
	}
	if down < hysteria.MinSpeedBPS {
		return nil, E.New("invalid down speed")
	}
	inbound := &Hysteria{
		ctx:           ctx,
		router:        router,
		logger:        logger,
		tag:           tag,
		quicConfig:    quicConfig,
		listenOptions: options.ListenOptions,
		authKey:       auth,
		xplusKey:      xplus,
		sendBPS:       up,
		recvBPS:       down,
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, ErrTLSRequired
	}
	if len(options.TLS.ALPN) == 0 {
		options.TLS.ALPN = []string{hysteria.DefaultALPN}
	}
	tlsConfig, err := NewTLSConfig(logger, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	inbound.tlsConfig = tlsConfig
	return inbound, nil
}

func (h *Hysteria) Type() string {
	return C.TypeHysteria
}

func (h *Hysteria) Tag() string {
	return h.tag
}

func (h *Hysteria) Start() error {
	listenAddr := M.SocksaddrFrom(netip.Addr(h.listenOptions.Listen), h.listenOptions.ListenPort)
	var packetConn net.PacketConn
	var err error
	packetConn, err = net.ListenUDP(M.NetworkFromNetAddr("udp", listenAddr.Addr), listenAddr.UDPAddr())
	if err != nil {
		return err
	}
	if len(h.xplusKey) > 0 {
		packetConn = hysteria.NewXPlusPacketConn(packetConn, h.xplusKey)
	}
	err = h.tlsConfig.Start()
	if err != nil {
		return err
	}
	listener, err := quic.Listen(packetConn, h.tlsConfig.Config(), h.quicConfig)
	if err != nil {
		return err
	}
	h.listener = listener
	go h.acceptLoop()
	return nil
}

func (h *Hysteria) acceptLoop() {
	for {
		ctx := log.ContextWithNewID(h.ctx)
		conn, err := h.listener.Accept(ctx)
		if err != nil {
			return
		}
		go func() {
			hErr := h.accept(ctx, conn)
			if hErr != nil {
				conn.CloseWithError(0, "")
				NewError(h.logger, ctx, E.Cause(hErr, "process connection from ", conn.RemoteAddr()))
			}
		}()
	}
}

func (h *Hysteria) accept(ctx context.Context, conn quic.Connection) error {
	controlStream, err := conn.AcceptStream(ctx)
	if err != nil {
		return err
	}
	clientHello, err := hysteria.ReadClientHello(controlStream)
	if err != nil {
		return err
	}
	if !bytes.Equal(clientHello.Auth, h.authKey) {
		err = hysteria.WriteServerHello(controlStream, hysteria.ServerHello{
			Message: "wrong password",
		})
		return E.Errors(E.New("wrong password: ", string(clientHello.Auth)), err)
	}
	if clientHello.SendBPS == 0 || clientHello.RecvBPS == 0 {
		return E.New("invalid rate from client")
	}
	serverSendBPS, serverRecvBPS := clientHello.RecvBPS, clientHello.SendBPS
	if h.sendBPS > 0 && serverSendBPS > h.sendBPS {
		serverSendBPS = h.sendBPS
	}
	if h.recvBPS > 0 && serverRecvBPS > h.recvBPS {
		serverRecvBPS = h.recvBPS
	}
	err = hysteria.WriteServerHello(controlStream, hysteria.ServerHello{
		OK:      true,
		SendBPS: serverSendBPS,
		RecvBPS: serverRecvBPS,
	})
	if err != nil {
		return err
	}
	// TODO: set congestion control
	go h.udpRecvLoop(conn)
	var stream quic.Stream
	for {
		stream, err = conn.AcceptStream(ctx)
		if err != nil {
			return err
		}
		hErr := h.acceptStream(ctx, conn, stream)
		if hErr != nil {
			stream.Close()
			NewError(h.logger, ctx, E.Cause(hErr, "process stream from ", conn.RemoteAddr()))
		}
	}
}

func (h *Hysteria) udpRecvLoop(conn quic.Connection) {
}

func (h *Hysteria) acceptStream(ctx context.Context, conn quic.Connection, stream quic.Stream) error {
	request, err := hysteria.ReadClientRequest(stream)
	if err != nil {
		return err
	}
	if request.UDP {
		err = hysteria.WriteServerResponse(stream, hysteria.ServerResponse{
			Message: "unsupported",
		}, nil)
		if err != nil {
			return err
		}
		stream.Close()
		return nil
	}
	var metadata adapter.InboundContext
	metadata.Inbound = h.tag
	metadata.InboundType = C.TypeHysteria
	metadata.SniffEnabled = h.listenOptions.SniffEnabled
	metadata.SniffOverrideDestination = h.listenOptions.SniffOverrideDestination
	metadata.DomainStrategy = dns.DomainStrategy(h.listenOptions.DomainStrategy)
	metadata.Network = N.NetworkTCP
	metadata.Source = M.SocksaddrFromNet(conn.RemoteAddr())
	metadata.Destination = M.ParseSocksaddrHostPort(request.Host, request.Port)
	return h.router.RouteConnection(ctx, hysteria.NewServerConn(stream, metadata.Destination), metadata)
}

func (h *Hysteria) Close() error {
	return common.Close(
		h.listener,
		common.PtrOrNil(h.tlsConfig),
	)
}
