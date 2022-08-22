package v2raygrpc

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing/common/rw"
)

var _ net.Conn = (*GRPCConn)(nil)

type GRPCConn struct {
	GunService
	cancel context.CancelFunc
	cache  []byte
}

func NewGRPCConn(service GunService, cancel context.CancelFunc) *GRPCConn {
	if client, isClient := service.(GunService_TunClient); isClient {
		service = &clientConnWrapper{client}
	}
	return &GRPCConn{
		GunService: service,
		cancel:     cancel,
	}
}

func (c *GRPCConn) Read(b []byte) (n int, err error) {
	if len(c.cache) > 0 {
		n = copy(b, c.cache)
		c.cache = c.cache[n:]
		return
	}
	hunk, err := c.Recv()
	err = wrapError(err)
	if err != nil {
		return
	}
	n = copy(b, hunk.Data)
	if n < len(hunk.Data) {
		c.cache = hunk.Data[n:]
	}
	return
}

func (c *GRPCConn) Write(b []byte) (n int, err error) {
	err = wrapError(c.Send(&Hunk{Data: b}))
	if err != nil {
		return
	}
	return len(b), nil
}

func (c *GRPCConn) Close() error {
	c.cancel()
	return nil
}

func (c *GRPCConn) LocalAddr() net.Addr {
	return nil
}

func (c *GRPCConn) RemoteAddr() net.Addr {
	return nil
}

func (c *GRPCConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *GRPCConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *GRPCConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *GRPCConn) Upstream() any {
	return c.GunService
}

var _ rw.WriteCloser = (*clientConnWrapper)(nil)

type clientConnWrapper struct {
	GunService_TunClient
}

func (c *clientConnWrapper) CloseWrite() error {
	return c.CloseSend()
}

func wrapError(err error) error {
	// grpc uses stupid internal error types
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "EOF") {
		return io.EOF
	}
	return err
}
