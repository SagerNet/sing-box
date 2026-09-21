package http

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing/common/baderror"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type clientStreamConn struct {
	reader     io.ReadCloser
	writer     net.Conn
	cancel     context.CancelFunc
	localAddr  net.Addr
	remoteAddr net.Addr
	closed     atomic.Bool
}

func (c *clientStreamConn) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	return n, c.wrapError(err)
}

func (c *clientStreamConn) Write(p []byte) (int, error) {
	n, err := c.writer.Write(p)
	return n, c.wrapError(err)
}

func (c *clientStreamConn) wrapError(err error) error {
	if err == nil {
		return nil
	}
	if c.closed.Load() || strings.Contains(err.Error(), "client connection force closed") {
		return net.ErrClosed
	}
	return baderror.WrapH2(err)
}

func (c *clientStreamConn) CloseWrite() error {
	return c.writer.Close()
}

func (c *clientStreamConn) Close() error {
	c.closed.Store(true)
	c.writer.Close()
	c.reader.Close()
	c.cancel()
	return nil
}

func (c *clientStreamConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *clientStreamConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *clientStreamConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *clientStreamConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *clientStreamConn) SetWriteDeadline(t time.Time) error {
	return c.writer.SetWriteDeadline(t)
}

func (c *clientStreamConn) NeedAdditionalReadDeadline() bool {
	return true
}

type clientTunnelConn struct {
	N.ExtendedConn
	streamConn *clientStreamConn
}

func newClientTunnelConn(streamConn *clientStreamConn) *clientTunnelConn {
	return &clientTunnelConn{ExtendedConn: deadline.NewConn(streamConn), streamConn: streamConn}
}

func (c *clientTunnelConn) CloseWrite() error {
	return c.streamConn.CloseWrite()
}

func (c *clientTunnelConn) SetDeadline(t time.Time) error {
	return E.Errors(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}

func (c *clientTunnelConn) Upstream() any {
	return c.ExtendedConn
}

var (
	_ net.Conn      = (*clientStreamConn)(nil)
	_ N.WriteCloser = (*clientStreamConn)(nil)
	_ N.WriteCloser = (*clientTunnelConn)(nil)
)
