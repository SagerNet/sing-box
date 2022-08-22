package v2rayquic

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/hysteria"
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
	tlsConfig  *tls.Config
	quicConfig *quic.Config
	conn       quic.Connection
	connAccess sync.Mutex
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayQUICOptions, tlsConfig *tls.Config) adapter.V2RayClientTransport {
	quicConfig := &quic.Config{
		DisablePathMTUDiscovery: !C.IsLinux && !C.IsWindows,
	}
	if len(tlsConfig.NextProtos) == 0 {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}
	return &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		tlsConfig:  tlsConfig,
		quicConfig: quicConfig,
	}
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
	c.conn = conn
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
