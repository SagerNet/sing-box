package hysteria2

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/common/qtls"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/transport/hysteria2/congestion"
	"github.com/sagernet/sing-box/transport/hysteria2/internal/protocol"
	tuicCongestion "github.com/sagernet/sing-box/transport/tuic/congestion"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const (
	defaultStreamReceiveWindow = 8388608                            // 8MB
	defaultConnReceiveWindow   = defaultStreamReceiveWindow * 5 / 2 // 20MB
	defaultMaxIdleTimeout      = 30 * time.Second
	defaultKeepAlivePeriod     = 10 * time.Second
)

type ClientOptions struct {
	Context            context.Context
	Dialer             N.Dialer
	ServerAddress      M.Socksaddr
	SendBPS            uint64
	ReceiveBPS         uint64
	SalamanderPassword string
	Password           string
	TLSConfig          tls.Config
	UDPDisabled        bool
}

type Client struct {
	ctx                context.Context
	dialer             N.Dialer
	serverAddr         M.Socksaddr
	sendBPS            uint64
	receiveBPS         uint64
	salamanderPassword string
	password           string
	tlsConfig          tls.Config
	quicConfig         *quic.Config
	udpDisabled        bool

	connAccess sync.RWMutex
	conn       *clientQUICConnection
}

func NewClient(options ClientOptions) (*Client, error) {
	quicConfig := &quic.Config{
		DisablePathMTUDiscovery:        !(runtime.GOOS == "windows" || runtime.GOOS == "linux" || runtime.GOOS == "android" || runtime.GOOS == "darwin"),
		MaxDatagramFrameSize:           1400,
		EnableDatagrams:                true,
		InitialStreamReceiveWindow:     defaultStreamReceiveWindow,
		MaxStreamReceiveWindow:         defaultStreamReceiveWindow,
		InitialConnectionReceiveWindow: defaultConnReceiveWindow,
		MaxConnectionReceiveWindow:     defaultConnReceiveWindow,
		MaxIdleTimeout:                 defaultMaxIdleTimeout,
		KeepAlivePeriod:                defaultKeepAlivePeriod,
	}
	return &Client{
		ctx:                options.Context,
		dialer:             options.Dialer,
		serverAddr:         options.ServerAddress,
		sendBPS:            options.SendBPS,
		receiveBPS:         options.ReceiveBPS,
		salamanderPassword: options.SalamanderPassword,
		password:           options.Password,
		tlsConfig:          options.TLSConfig,
		quicConfig:         quicConfig,
		udpDisabled:        options.UDPDisabled,
	}, nil
}

func (c *Client) offer(ctx context.Context) (*clientQUICConnection, error) {
	conn := c.conn
	if conn != nil && conn.active() {
		return conn, nil
	}
	c.connAccess.Lock()
	defer c.connAccess.Unlock()
	conn = c.conn
	if conn != nil && conn.active() {
		return conn, nil
	}
	conn, err := c.offerNew(ctx)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *Client) offerNew(ctx context.Context) (*clientQUICConnection, error) {
	udpConn, err := c.dialer.DialContext(ctx, "udp", c.serverAddr)
	if err != nil {
		return nil, err
	}
	var packetConn net.PacketConn
	packetConn = bufio.NewUnbindPacketConn(udpConn)
	if c.salamanderPassword != "" {
		packetConn = NewSalamanderConn(packetConn, []byte(c.salamanderPassword))
	}
	var quicConn quic.EarlyConnection
	http3Transport, err := qtls.CreateTransport(packetConn, &quicConn, c.serverAddr, c.tlsConfig, c.quicConfig, true)
	if err != nil {
		udpConn.Close()
		return nil, err
	}
	request := &http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "https",
			Host:   protocol.URLHost,
			Path:   protocol.URLPath,
		},
		Header: make(http.Header),
	}
	protocol.AuthRequestToHeader(request.Header, protocol.AuthRequest{Auth: c.password, Rx: c.receiveBPS})
	response, err := http3Transport.RoundTrip(request)
	if err != nil {
		if quicConn != nil {
			quicConn.CloseWithError(0, "")
		}
		udpConn.Close()
		return nil, err
	}
	if response.StatusCode != protocol.StatusAuthOK {
		if quicConn != nil {
			quicConn.CloseWithError(0, "")
		}
		udpConn.Close()
		return nil, E.New("authentication failed, status code: ", response.StatusCode)
	}
	response.Body.Close()
	authResponse := protocol.AuthResponseFromHeader(response.Header)
	actualTx := authResponse.Rx
	if actualTx == 0 || actualTx > c.sendBPS {
		actualTx = c.sendBPS
	}
	if !authResponse.RxAuto && actualTx > 0 {
		quicConn.SetCongestionControl(congestion.NewBrutalSender(actualTx))
	} else {
		quicConn.SetCongestionControl(tuicCongestion.NewBBRSender(
			tuicCongestion.DefaultClock{},
			tuicCongestion.GetInitialPacketSize(quicConn.RemoteAddr()),
			tuicCongestion.InitialCongestionWindow*tuicCongestion.InitialMaxDatagramSize,
			tuicCongestion.DefaultBBRMaxCongestionWindow*tuicCongestion.InitialMaxDatagramSize,
		))
	}
	conn := &clientQUICConnection{
		quicConn:    quicConn,
		rawConn:     udpConn,
		connDone:    make(chan struct{}),
		udpDisabled: c.udpDisabled || !authResponse.UDPEnabled,
		udpConnMap:  make(map[uint32]*udpPacketConn),
	}
	if !c.udpDisabled {
		go c.loopMessages(conn)
	}
	c.conn = conn
	return conn, nil
}

func (c *Client) DialConn(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	conn, err := c.offer(ctx)
	if err != nil {
		return nil, err
	}
	stream, err := conn.quicConn.OpenStream()
	if err != nil {
		return nil, err
	}
	return &clientConn{
		Stream:      stream,
		destination: destination,
	}, nil
}

func (c *Client) ListenPacket(ctx context.Context) (net.PacketConn, error) {
	if c.udpDisabled {
		return nil, os.ErrInvalid
	}
	conn, err := c.offer(ctx)
	if err != nil {
		return nil, err
	}
	if conn.udpDisabled {
		return nil, E.New("UDP disabled by server")
	}
	var sessionID uint32
	clientPacketConn := newUDPPacketConn(ctx, conn.quicConn, func() {
		conn.udpAccess.Lock()
		delete(conn.udpConnMap, sessionID)
		conn.udpAccess.Unlock()
	})
	conn.udpAccess.Lock()
	sessionID = conn.udpSessionID
	conn.udpSessionID++
	conn.udpConnMap[sessionID] = clientPacketConn
	conn.udpAccess.Unlock()
	clientPacketConn.sessionID = sessionID
	return clientPacketConn, nil
}

func (c *Client) CloseWithError(err error) error {
	conn := c.conn
	if conn != nil {
		conn.closeWithError(err)
	}
	return nil
}

type clientQUICConnection struct {
	quicConn     quic.Connection
	rawConn      io.Closer
	closeOnce    sync.Once
	connDone     chan struct{}
	connErr      error
	udpDisabled  bool
	udpAccess    sync.RWMutex
	udpConnMap   map[uint32]*udpPacketConn
	udpSessionID uint32
}

func (c *clientQUICConnection) active() bool {
	select {
	case <-c.quicConn.Context().Done():
		return false
	default:
	}
	select {
	case <-c.connDone:
		return false
	default:
	}
	return true
}

func (c *clientQUICConnection) closeWithError(err error) {
	c.closeOnce.Do(func() {
		c.connErr = err
		close(c.connDone)
		c.quicConn.CloseWithError(0, "")
	})
}

type clientConn struct {
	quic.Stream
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func (c *clientConn) NeedHandshake() bool {
	return !c.requestWritten
}

func (c *clientConn) Read(p []byte) (n int, err error) {
	if c.responseRead {
		return c.Stream.Read(p)
	}
	status, errorMessage, err := protocol.ReadTCPResponse(c.Stream)
	if err != nil {
		return
	}
	if !status {
		err = E.New("remote error: ", errorMessage)
		return
	}
	c.responseRead = true
	return c.Stream.Read(p)
}

func (c *clientConn) Write(p []byte) (n int, err error) {
	if !c.requestWritten {
		buffer := protocol.WriteTCPRequest(c.destination.String(), p)
		defer buffer.Release()
		_, err = c.Stream.Write(buffer.Bytes())
		if err != nil {
			return
		}
		c.requestWritten = true
		return len(p), nil
	}
	return c.Stream.Write(p)
}

func (c *clientConn) LocalAddr() net.Addr {
	return M.Socksaddr{}
}

func (c *clientConn) RemoteAddr() net.Addr {
	return M.Socksaddr{}
}
