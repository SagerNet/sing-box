package v2raygrpc

import (
	"context"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing/common/baderror"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"google.golang.org/grpc"
)

var _ net.Conn = (*GRPCConn)(nil)

type GRPCConn struct {
	GunService
	cache     []byte
	cancel    context.CancelCauseFunc
	closeOnce sync.Once
}

func NewGRPCConn(service GunService, cancel context.CancelCauseFunc) net.Conn {
	conn := &GRPCConn{
		GunService: service,
		cancel:     cancel,
	}
	if client, isClient := service.(grpc.ClientStream); isClient {
		return &GRPCClientConn{GRPCConn: conn, client: client}
	}
	return conn
}

func (c *GRPCConn) Read(b []byte) (n int, err error) {
	if len(c.cache) > 0 {
		n = copy(b, c.cache)
		c.cache = c.cache[n:]
		return
	}
	hunk, err := c.Recv()
	err = baderror.WrapGRPC(err)
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
	err = baderror.WrapGRPC(c.Send(&Hunk{Data: b}))
	if err != nil {
		return
	}
	return len(b), nil
}

func (c *GRPCConn) Close() error {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel(nil)
		}
	})
	return nil
}

func (c *GRPCConn) LocalAddr() net.Addr {
	return M.Socksaddr{}
}

func (c *GRPCConn) RemoteAddr() net.Addr {
	return M.Socksaddr{}
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

func (c *GRPCConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *GRPCConn) Upstream() any {
	return c.GunService
}

var _ N.WriteCloser = (*GRPCClientConn)(nil)

type GRPCClientConn struct {
	*GRPCConn
	client grpc.ClientStream
}

func (c *GRPCClientConn) CloseWrite() error {
	return c.client.CloseSend()
}
