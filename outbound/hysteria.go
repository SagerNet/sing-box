//go:build with_quic

package outbound

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/congestion"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/qtls"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/hysteria"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound                = (*Hysteria)(nil)
	_ adapter.InterfaceUpdateListener = (*Hysteria)(nil)
)

type Hysteria struct {
	myOutboundAdapter
	ctx          context.Context
	dialer       N.Dialer
	serverAddr   M.Socksaddr
	tlsConfig    tls.Config
	quicConfig   *quic.Config
	authKey      []byte
	xplusKey     []byte
	sendBPS      uint64
	recvBPS      uint64
	connAccess   sync.Mutex
	conn         quic.Connection
	rawConn      net.Conn
	udpAccess    sync.RWMutex
	udpSessions  map[uint32]chan *hysteria.UDPMessage
	udpDefragger hysteria.Defragger
}

func NewHysteria(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HysteriaOutboundOptions) (*Hysteria, error) {
	options.UDPFragmentDefault = true
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	tlsConfig, err := tls.NewClient(ctx, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	if len(tlsConfig.NextProtos()) == 0 {
		tlsConfig.SetNextProtos([]string{hysteria.DefaultALPN})
	}
	quicConfig := &quic.Config{
		InitialStreamReceiveWindow:     options.ReceiveWindowConn,
		MaxStreamReceiveWindow:         options.ReceiveWindowConn,
		InitialConnectionReceiveWindow: options.ReceiveWindow,
		MaxConnectionReceiveWindow:     options.ReceiveWindow,
		KeepAlivePeriod:                hysteria.KeepAlivePeriod,
		DisablePathMTUDiscovery:        options.DisableMTUDiscovery,
		EnableDatagrams:                true,
	}
	if options.ReceiveWindowConn == 0 {
		quicConfig.InitialStreamReceiveWindow = hysteria.DefaultStreamReceiveWindow
		quicConfig.MaxStreamReceiveWindow = hysteria.DefaultStreamReceiveWindow
	}
	if options.ReceiveWindow == 0 {
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
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	return &Hysteria{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeHysteria,
			network:      options.Network.Build(),
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
		ctx:        ctx,
		dialer:     outboundDialer,
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
	h.udpAccess.Lock()
	defer h.udpAccess.Unlock()
	conn = h.conn
	if conn != nil && !common.Done(conn.Context()) {
		return conn, nil
	}
	common.Close(h.rawConn)
	conn, err := h.offerNew(ctx)
	if err != nil {
		return nil, err
	}
	if common.Contains(h.network, N.NetworkUDP) {
		for _, session := range h.udpSessions {
			close(session)
		}
		h.udpSessions = make(map[uint32]chan *hysteria.UDPMessage)
		h.udpDefragger = hysteria.Defragger{}
		go h.udpRecvLoop(conn)
	}
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
	packetConn = &hysteria.PacketConnWrapper{PacketConn: packetConn}
	quicConn, err := qtls.Dial(h.ctx, packetConn, udpConn.RemoteAddr(), h.tlsConfig, h.quicConfig)
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
		packetConn.Close()
		return nil, err
	}
	serverHello, err := hysteria.ReadServerHello(controlStream)
	if err != nil {
		packetConn.Close()
		return nil, err
	}
	if !serverHello.OK {
		packetConn.Close()
		return nil, E.New("remote error: ", serverHello.Message)
	}
	quicConn.SetCongestionControl(hysteria.NewBrutalSender(congestion.ByteCount(serverHello.RecvBPS)))
	h.conn = quicConn
	h.rawConn = udpConn
	return quicConn, nil
}

func (h *Hysteria) udpRecvLoop(conn quic.Connection) {
	for {
		packet, err := conn.ReceiveMessage(h.ctx)
		if err != nil {
			return
		}
		message, err := hysteria.ParseUDPMessage(packet)
		if err != nil {
			h.logger.Error("parse udp message: ", err)
			continue
		}
		dfMsg := h.udpDefragger.Feed(message)
		if dfMsg == nil {
			continue
		}
		h.udpAccess.RLock()
		ch, ok := h.udpSessions[dfMsg.SessionID]
		if ok {
			select {
			case ch <- dfMsg:
				// OK
			default:
				// Silently drop the message when the channel is full
			}
		}
		h.udpAccess.RUnlock()
	}
}

func (h *Hysteria) InterfaceUpdated() {
	h.Close()
	return
}

func (h *Hysteria) Close() error {
	h.connAccess.Lock()
	defer h.connAccess.Unlock()
	h.udpAccess.Lock()
	defer h.udpAccess.Unlock()
	if h.conn != nil {
		h.conn.CloseWithError(0, "")
		h.rawConn.Close()
	}
	for _, session := range h.udpSessions {
		close(session)
	}
	h.udpSessions = make(map[uint32]chan *hysteria.UDPMessage)
	return nil
}

func (h *Hysteria) open(ctx context.Context, reconnect bool) (quic.Connection, quic.Stream, error) {
	conn, err := h.offer(ctx)
	if err != nil {
		if nErr, ok := err.(net.Error); ok && !nErr.Temporary() && reconnect {
			return h.open(ctx, false)
		}
		return nil, nil, err
	}
	stream, err := conn.OpenStream()
	if err != nil {
		if nErr, ok := err.(net.Error); ok && !nErr.Temporary() && reconnect {
			return h.open(ctx, false)
		}
		return nil, nil, err
	}
	return conn, &hysteria.StreamWrapper{Stream: stream}, nil
}

func (h *Hysteria) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		_, stream, err := h.open(ctx, true)
		if err != nil {
			return nil, err
		}
		err = hysteria.WriteClientRequest(stream, hysteria.ClientRequest{
			Host: destination.AddrString(),
			Port: destination.Port,
		})
		if err != nil {
			stream.Close()
			return nil, err
		}
		return hysteria.NewConn(stream, destination, true), nil
	case N.NetworkUDP:
		conn, err := h.ListenPacket(ctx, destination)
		if err != nil {
			return nil, err
		}
		return conn.(*hysteria.PacketConn), nil
	default:
		return nil, E.New("unsupported network: ", network)
	}
}

func (h *Hysteria) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	conn, stream, err := h.open(ctx, true)
	if err != nil {
		return nil, err
	}
	err = hysteria.WriteClientRequest(stream, hysteria.ClientRequest{
		UDP:  true,
		Host: destination.AddrString(),
		Port: destination.Port,
	})
	if err != nil {
		stream.Close()
		return nil, err
	}
	var response *hysteria.ServerResponse
	response, err = hysteria.ReadServerResponse(stream)
	if err != nil {
		stream.Close()
		return nil, err
	}
	if !response.OK {
		stream.Close()
		return nil, E.New("remote error: ", response.Message)
	}
	h.udpAccess.Lock()
	nCh := make(chan *hysteria.UDPMessage, 1024)
	h.udpSessions[response.UDPSessionID] = nCh
	h.udpAccess.Unlock()
	packetConn := hysteria.NewPacketConn(conn, stream, response.UDPSessionID, destination, nCh, common.Closer(func() error {
		h.udpAccess.Lock()
		if ch, ok := h.udpSessions[response.UDPSessionID]; ok {
			close(ch)
			delete(h.udpSessions, response.UDPSessionID)
		}
		h.udpAccess.Unlock()
		return nil
	}))
	go packetConn.Hold()
	return packetConn, nil
}

func (h *Hysteria) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *Hysteria) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}
