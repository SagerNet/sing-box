package wsc

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

var _ net.PacketConn = &clientPacketConn{}

type clientPacketConn struct {
	net.Conn
	reader *wsutil.Reader
	mu     sync.Mutex
}

func (cli *Client) newPacketConn(ctx context.Context, network string, endpoint string) (*clientPacketConn, error) {
	conn, err := cli.newWSConn(ctx, network, endpoint)
	if err != nil {
		return nil, err
	}
	reader := wsutil.NewReader(conn, ws.StateClientSide)
	return &clientPacketConn{
		Conn:   conn,
		reader: reader,
	}, nil
}

func (packetConn *clientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return 0, nil, nil
}

func (packetConn *clientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return 0, nil
}

// func (c *clientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
// 	destination := M.SocksaddrFromNet(addr)
// 	buffer := buf.NewSize(M.SocksaddrSerializer.AddrPortLen(destination) + len(p))
// 	defer buffer.Release()
// 	if err = M.SocksaddrSerializer.WriteAddrPort(buffer, destination); err != nil {
// 		return 0, err
// 	}
// 	if _, err = buffer.Write(p); err != nil {
// 		return 0, err
// 	}
// 	c.mu.Lock()
// 	defer c.mu.Unlock()
// 	if err = wsutil.WriteClientBinary(c.Conn, buffer.Bytes()); err != nil {
// 		return 0, err
// 	}
// 	return len(p), nil
// }

func (packetConn *clientPacketConn) Close() error {
	packetConn.mu.Lock()
	defer packetConn.mu.Unlock()
	_ = wsutil.WriteClientMessage(packetConn.Conn, ws.OpClose, nil)
	return packetConn.Conn.Close()
}

/*
package wsc

import (
	"bytes"
	"context"
	"net"
	"net/url"
	"sync"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

// clientPacketConn implements net.PacketConn over WebSocket.
type clientPacketConn struct {
	net.Conn
	mu sync.Mutex
}

// newPacketConn dials a WebSocket endpoint for packet based communications.
func (cli *Client) newPacketConn(ctx context.Context, network string, endpoint string) (*clientPacketConn, error) {
	scheme := "ws"
	if cli.TLS != nil {
		scheme = "wss"
	}

	pURL := url.URL{
		Scheme:   scheme,
		Host:     cli.Host,
		Path:     cli.Path,
		RawQuery: "",
	}
	pQuery := pURL.Query()
	pQuery.Set("auth", cli.Auth)
	if network != "" {
		pQuery.Set("net", network)
	}
	if endpoint != "" {
		pQuery.Set("ep", endpoint)
	}
	pURL.RawQuery = pQuery.Encode()

	dialer := ws.Dialer{
		NetDial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := cli.Dialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddr(addr))
			if err != nil {
				return nil, err
			}
			if cli.TLS != nil {
				conn, err = tls.ClientHandshake(ctx, conn, cli.TLS)
				if err != nil {
					return nil, err
				}
			}
			return conn, nil
		},
	}
	conn, _, _, err := dialer.Dial(ctx, pURL.String())
	if err != nil {
		return nil, err
	}
	return &clientPacketConn{Conn: conn}, nil
}

// ListenPacket creates a packet-oriented WebSocket connection.
func (cli *Client) ListenPacket(ctx context.Context, network string, endpoint string) (net.PacketConn, error) {
	return cli.newPacketConn(ctx, network, endpoint)
}

func (c *clientPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	msg, err := wsutil.ReadServerBinary(c.Conn)
	if err != nil {
		return M.Socksaddr{}, err
	}
	reader := bytes.NewReader(msg)
	destination, err := M.SocksaddrSerializer.ReadAddrPort(reader)
	if err != nil {
		return M.Socksaddr{}, err
	}
	_, err = buffer.Write(msg[len(msg)-reader.Len():])
	if err != nil {
		return M.Socksaddr{}, err
	}
	return destination, nil
}

func (c *clientPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	header := buf.With(buffer.ExtendHeader(M.SocksaddrSerializer.AddrPortLen(destination)))
	if err := M.SocksaddrSerializer.WriteAddrPort(header, destination); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return wsutil.WriteClientBinary(c.Conn, buffer.Bytes())
}

func (c *clientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	msg, err := wsutil.ReadServerBinary(c.Conn)
	if err != nil {
		return 0, nil, err
	}
	reader := bytes.NewReader(msg)
	destination, err := M.SocksaddrSerializer.ReadAddrPort(reader)
	if err != nil {
		return 0, nil, err
	}
	n = copy(p, msg[len(msg)-reader.Len():])
	if destination.IsFqdn() {
		addr = destination
	} else {
		addr = destination.UDPAddr()
	}
	return
}

func (c *clientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	destination := M.SocksaddrFromNet(addr)
	buffer := buf.NewSize(M.SocksaddrSerializer.AddrPortLen(destination) + len(p))
	defer buffer.Release()
	if err = M.SocksaddrSerializer.WriteAddrPort(buffer, destination); err != nil {
		return 0, err
	}
	if _, err = buffer.Write(p); err != nil {
		return 0, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err = wsutil.WriteClientBinary(c.Conn, buffer.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *clientPacketConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = wsutil.WriteClientMessage(c.Conn, ws.OpClose, nil)
	return c.Conn.Close()
}

func (c *clientPacketConn) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}
*/
