package hysteria

import (
	"net"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/lucas-clemente/quic-go"
)

var (
	_ net.Conn        = (*ServerConn)(nil)
	_ N.HandshakeConn = (*ServerConn)(nil)
)

type ServerConn struct {
	quic.Stream
	destination     M.Socksaddr
	responseWritten bool
}

func NewServerConn(stream quic.Stream, destination M.Socksaddr) *ServerConn {
	return &ServerConn{
		Stream:      stream,
		destination: destination,
	}
}

func (c *ServerConn) LocalAddr() net.Addr {
	return nil
}

func (c *ServerConn) RemoteAddr() net.Addr {
	return c.destination.TCPAddr()
}

func (c *ServerConn) Write(b []byte) (n int, err error) {
	if !c.responseWritten {
		err = WriteServerResponse(c.Stream, ServerResponse{
			OK: true,
		}, b)
		c.responseWritten = true
		return len(b), nil
	}
	return c.Stream.Write(b)
}

func (c *ServerConn) ReaderReplaceable() bool {
	return true
}

func (c *ServerConn) WriterReplaceable() bool {
	return c.responseWritten
}

func (c *ServerConn) HandshakeFailure(err error) error {
	if c.responseWritten {
		return nil
	}
	return WriteServerResponse(c.Stream, ServerResponse{
		Message: err.Error(),
	}, nil)
}

func (c *ServerConn) Upstream() any {
	return c.Stream
}
