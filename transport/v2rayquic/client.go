package v2rayquic

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/jobberrt/sing-box/adapter"
	"github.com/jobberrt/sing-box/common/tls"
	C "github.com/jobberrt/sing-box/constant"
	"github.com/jobberrt/sing-box/option"
	"github.com/jobberrt/sing-box/transport/hysteria"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr
	tlsConfig  *tls.STDConfig
	quicConfig *quic.Config
	connAccess sync.Mutex
	conn       quic.Connection
	rawConn    net.Conn
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayQUICOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	quicConfig := &quic.Config{
		DisablePathMTUDiscovery: !C.IsLinux && !C.IsWindows,
	}
	stdConfig, err := tlsConfig.Config()
	if err != nil {
		return nil, err
	}
	if len(stdConfig.NextProtos) == 0 {
		stdConfig.NextProtos = []string{"h2", "http/1.1"}
	}
	return &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		tlsConfig:  stdConfig,
		quicConfig: quicConfig,
	}, nil
}

func (c *Client) offer() (quic.Connection, error) {
	conn := c.conn
	if conn != nil && !common.Done(conn.Context()) {
		return conn, nil
	}
	c.connAccess.Lock()
	defer c.connAccess.Unlock()
	conn = c.conn
	if conn != nil && !common.Done(conn.Context()) {
		return conn, nil
	}
	conn, err := c.offerNew()
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *Client) offerNew() (quic.Connection, error) {
	udpConn, err := c.dialer.DialContext(c.ctx, "udp", c.serverAddr)
	if err != nil {
		return nil, err
	}
	var packetConn net.PacketConn
	packetConn = bufio.NewUnbindPacketConn(udpConn)
	quicConn, err := quic.Dial(packetConn, udpConn.RemoteAddr(), c.serverAddr.AddrString(), c.tlsConfig, c.quicConfig)
	if err != nil {
		packetConn.Close()
		return nil, err
	}
	c.conn = quicConn
	c.rawConn = udpConn
	return quicConn, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	conn, err := c.offer()
	if err != nil {
		return nil, err
	}
	stream, err := conn.OpenStream()
	if err != nil {
		return nil, err
	}
	return &hysteria.StreamWrapper{Conn: conn, Stream: stream}, nil
}

func (c *Client) Close() error {
	return common.Close(c.conn, c.rawConn)
}
