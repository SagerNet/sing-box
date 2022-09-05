package wireguard

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.zx2c4.com/wireguard/conn"
)

var _ conn.Bind = (*ClientBind)(nil)

type ClientBind struct {
	ctx        context.Context
	dialer     N.Dialer
	peerAddr   M.Socksaddr
	connAccess sync.Mutex
	conn       *wireConn
}

func NewClientBind(ctx context.Context, dialer N.Dialer, peerAddr M.Socksaddr) *ClientBind {
	return &ClientBind{
		ctx:      ctx,
		dialer:   dialer,
		peerAddr: peerAddr,
	}
}

func (c *ClientBind) connect() (*wireConn, error) {
	serverConn := c.conn
	if serverConn != nil {
		select {
		case <-serverConn.done:
			serverConn = nil
		default:
			return serverConn, nil
		}
	}
	c.connAccess.Lock()
	defer c.connAccess.Unlock()
	serverConn = c.conn
	if serverConn != nil {
		select {
		case <-serverConn.done:
			serverConn = nil
		default:
			return serverConn, nil
		}
	}
	udpConn, err := c.dialer.DialContext(c.ctx, "udp", c.peerAddr)
	if err != nil {
		return nil, &wireError{err}
	}
	c.conn = &wireConn{
		Conn: udpConn,
		done: make(chan struct{}),
	}
	return c.conn, nil
}

func (c *ClientBind) Open(port uint16) (fns []conn.ReceiveFunc, actualPort uint16, err error) {
	return []conn.ReceiveFunc{c.receive}, 0, nil
}

func (c *ClientBind) receive(b []byte) (n int, ep conn.Endpoint, err error) {
	udpConn, err := c.connect()
	if err != nil {
		err = &wireError{err}
		return
	}
	n, err = udpConn.Read(b)
	if err != nil {
		udpConn.Close()
		err = &wireError{err}
	}
	ep = Endpoint(c.peerAddr)
	return
}

func (c *ClientBind) Close() error {
	c.connAccess.Lock()
	defer c.connAccess.Unlock()
	common.Close(common.PtrOrNil(c.conn))
	return nil
}

func (c *ClientBind) SetMark(mark uint32) error {
	return nil
}

func (c *ClientBind) Send(b []byte, ep conn.Endpoint) error {
	udpConn, err := c.connect()
	if err != nil {
		return err
	}
	_, err = udpConn.Write(b)
	if err != nil {
		udpConn.Close()
	}
	return err
}

func (c *ClientBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	return Endpoint(c.peerAddr), nil
}

func (c *ClientBind) Endpoint() conn.Endpoint {
	return Endpoint(c.peerAddr)
}

type wireConn struct {
	net.Conn
	access sync.Mutex
	done   chan struct{}
}

func (w *wireConn) Close() error {
	w.access.Lock()
	defer w.access.Unlock()
	select {
	case <-w.done:
		return net.ErrClosed
	default:
	}
	w.Conn.Close()
	close(w.done)
	return nil
}
