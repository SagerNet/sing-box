//go:build go1.20

package dialer

import (
	"context"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/metacubex/tfo-go"
)

type slowOpenConn struct {
	dialer      *tfo.Dialer
	ctx         context.Context
	network     string
	destination M.Socksaddr
	conn        net.Conn
	create      chan struct{}
	access      sync.Mutex
	err         error
}

func DialSlowContext(dialer *tcpDialer, ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if dialer.DisableTFO || N.NetworkName(network) != N.NetworkTCP {
		switch N.NetworkName(network) {
		case N.NetworkTCP, N.NetworkUDP:
			return dialer.Dialer.DialContext(ctx, network, destination.String())
		default:
			return dialer.Dialer.DialContext(ctx, network, destination.AddrString())
		}
	}
	return &slowOpenConn{
		dialer:      dialer,
		ctx:         ctx,
		network:     network,
		destination: destination,
		create:      make(chan struct{}),
	}, nil
}

func (c *slowOpenConn) Read(b []byte) (n int, err error) {
	if c.conn == nil {
		select {
		case <-c.create:
			if c.err != nil {
				return 0, c.err
			}
		case <-c.ctx.Done():
			return 0, c.ctx.Err()
		}
	}
	return c.conn.Read(b)
}

func (c *slowOpenConn) Write(b []byte) (n int, err error) {
	if c.conn != nil {
		return c.conn.Write(b)
	}
	c.access.Lock()
	defer c.access.Unlock()
	select {
	case <-c.create:
		if c.err != nil {
			return 0, c.err
		}
		return c.conn.Write(b)
	default:
	}
	c.conn, err = c.dialer.DialContext(c.ctx, c.network, c.destination.String(), b)
	if err != nil {
		c.conn = nil
		c.err = E.Cause(err, "dial tcp fast open")
	}
	n = len(b)
	close(c.create)
	return
}

func (c *slowOpenConn) Close() error {
	return common.Close(c.conn)
}

func (c *slowOpenConn) LocalAddr() net.Addr {
	if c.conn == nil {
		return M.Socksaddr{}
	}
	return c.conn.LocalAddr()
}

func (c *slowOpenConn) RemoteAddr() net.Addr {
	if c.conn == nil {
		return M.Socksaddr{}
	}
	return c.conn.RemoteAddr()
}

func (c *slowOpenConn) SetDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetDeadline(t)
}

func (c *slowOpenConn) SetReadDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetReadDeadline(t)
}

func (c *slowOpenConn) SetWriteDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetWriteDeadline(t)
}

func (c *slowOpenConn) Upstream() any {
	return c.conn
}

func (c *slowOpenConn) ReaderReplaceable() bool {
	return c.conn != nil
}

func (c *slowOpenConn) WriterReplaceable() bool {
	return c.conn != nil
}

func (c *slowOpenConn) LazyHeadroom() bool {
	return c.conn == nil
}

func (c *slowOpenConn) NeedHandshake() bool {
	return c.conn == nil
}

func (c *slowOpenConn) WriteTo(w io.Writer) (n int64, err error) {
	if c.conn == nil {
		select {
		case <-c.create:
			if c.err != nil {
				return 0, c.err
			}
		case <-c.ctx.Done():
			return 0, c.ctx.Err()
		}
	}
	return bufio.Copy(w, c.conn)
}
