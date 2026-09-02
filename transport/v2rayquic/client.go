//go:build with_quic

package v2rayquic

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-quic"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr
	tlsConfig  tls.Config
	quicConfig *quic.Config
	connAccess sync.Mutex
	conn       common.TypedValue[*clientConnection]
	rawConn    net.Conn
}

type clientConnection struct {
	*quic.Conn
	access   sync.Mutex
	streams  int
	draining bool
}

func (c *clientConnection) releaseStream() {
	c.access.Lock()
	c.streams--
	drained := c.draining && c.streams == 0
	c.access.Unlock()
	if drained {
		c.CloseWithError(0, "")
	}
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayQUICOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	quicConfig := &quic.Config{
		DisablePathMTUDiscovery: !C.IsLinux && !C.IsWindows,
	}
	if len(tlsConfig.NextProtos()) == 0 {
		tlsConfig.SetNextProtos([]string{http3.NextProtoH3})
	}
	return &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		tlsConfig:  tlsConfig,
		quicConfig: quicConfig,
	}, nil
}

func (c *Client) offer() (*clientConnection, error) {
	conn := c.conn.Load()
	if conn != nil && !common.Done(conn.Context()) {
		return conn, nil
	}
	c.connAccess.Lock()
	defer c.connAccess.Unlock()
	conn = c.conn.Load()
	if conn != nil && !common.Done(conn.Context()) {
		return conn, nil
	}
	conn, err := c.offerNew()
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *Client) offerNew() (*clientConnection, error) {
	udpConn, err := c.dialer.DialContext(c.ctx, "udp", c.serverAddr)
	if err != nil {
		return nil, err
	}
	quicConn, err := qtls.Dial(c.ctx, udpConn, c.tlsConfig, c.quicConfig)
	if err != nil {
		udpConn.Close()
		return nil, err
	}
	// quic-go does not take ownership of the conn passed to Dial:
	// when the connection ends it only stops reading.
	go func() {
		<-quicConn.Context().Done()
		udpConn.Close()
	}()
	conn := &clientConnection{Conn: quicConn}
	c.conn.Store(conn)
	c.rawConn = udpConn
	return conn, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	conn, err := c.offer()
	if err != nil {
		return nil, err
	}
	conn.access.Lock()
	conn.streams++
	conn.draining = false
	conn.access.Unlock()
	stream, err := conn.OpenStream()
	if err != nil {
		conn.releaseStream()
		return nil, err
	}
	return &StreamWrapper{Conn: conn.Conn, Stream: stream, onClose: conn.releaseStream}, nil
}

func (c *Client) CloseIdleConnections() {
	conn := c.conn.Load()
	if conn == nil {
		return
	}
	conn.access.Lock()
	conn.draining = true
	drained := conn.streams == 0
	conn.access.Unlock()
	if drained {
		conn.CloseWithError(0, "")
	}
}

func (c *Client) Close() error {
	c.connAccess.Lock()
	defer c.connAccess.Unlock()
	conn := c.conn.Swap(nil)
	if conn != nil {
		conn.CloseWithError(0, "")
	}
	if c.rawConn != nil {
		c.rawConn.Close()
	}
	c.rawConn = nil
	return nil
}
