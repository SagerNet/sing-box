package anytls

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"net"
	"time"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/transport/anytls/padding"
	"github.com/sagernet/sing-box/transport/anytls/session"
	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type ClientConfig struct {
	Password                 string
	IdleSessionCheckInterval time.Duration
	IdleSessionTimeout       time.Duration
	Server                   M.Socksaddr
	Dialer                   N.Dialer
	TLSConfig                tls.Config
	Logger                   logger.ContextLogger
}

type Client struct {
	passwordSha256 []byte
	tlsConfig      tls.Config
	dialer         N.Dialer
	server         M.Socksaddr
	sessionClient  *session.Client
	padding        atomic.TypedValue[*padding.PaddingFactory]
}

func NewClient(ctx context.Context, config ClientConfig) (*Client, error) {
	pw := sha256.Sum256([]byte(config.Password))
	c := &Client{
		passwordSha256: pw[:],
		tlsConfig:      config.TLSConfig,
		dialer:         config.Dialer,
		server:         config.Server,
	}
	// Initialize the padding state of this client
	padding.UpdatePaddingScheme(padding.DefaultPaddingScheme, &c.padding)
	c.sessionClient = session.NewClient(ctx, c.CreateOutboundTLSConnection, &c.padding, config.IdleSessionCheckInterval, config.IdleSessionTimeout)
	return c, nil
}

func (c *Client) CreateProxy(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	conn, err := c.sessionClient.CreateStream(ctx)
	if err != nil {
		return nil, err
	}
	err = M.SocksaddrSerializer.WriteAddrPort(conn, destination)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func (c *Client) CreateOutboundTLSConnection(ctx context.Context) (net.Conn, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.server)
	if err != nil {
		return nil, err
	}

	b := buf.NewPacket()
	defer b.Release()

	b.Write(c.passwordSha256)
	var paddingLen int
	if pad := c.padding.Load().GenerateRecordPayloadSizes(0); len(pad) > 0 {
		paddingLen = pad[0]
	}
	binary.BigEndian.PutUint16(b.Extend(2), uint16(paddingLen))
	if paddingLen > 0 {
		b.WriteZeroN(paddingLen)
	}

	conn, err = tls.ClientHandshake(ctx, conn, c.tlsConfig)
	if err != nil {
		return nil, err
	}

	_, err = b.WriteTo(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (h *Client) Close() error {
	return h.sessionClient.Close()
}
