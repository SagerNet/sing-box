package v2raygrpc

import (
	"net"
	"os"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/baderror"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ net.Conn = (*GRPCConn)(nil)

type GRPCConn struct {
	GunService
	cancel common.ContextCancelCauseFunc
	cache  []byte
}

func NewGRPCConn(service GunService, cancel common.ContextCancelCauseFunc) *GRPCConn {
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
	err = baderror.WrapGRPC(err)
	if err != nil {
		c.cancel(err)
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
		c.cancel(err)
		return
	}
	return len(b), nil
}

func (c *GRPCConn) Close() error {
	c.cancel(net.ErrClosed)
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

var _ N.WriteCloser = (*clientConnWrapper)(nil)

type clientConnWrapper struct {
	GunService_TunClient
}

func (c *clientConnWrapper) CloseWrite() error {
	return c.CloseSend()
}
