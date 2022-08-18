package hysteria

import (
	"net"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/lucas-clemente/quic-go"
)

var _ net.Conn = (*ClientConn)(nil)

type ClientConn struct {
	quic.Stream
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func NewClientConn(stream quic.Stream, destination M.Socksaddr) *ClientConn {
	return &ClientConn{
		Stream:      stream,
		destination: destination,
	}
}

func (c *ClientConn) Read(b []byte) (n int, err error) {
	if !c.responseRead {
		var response *ServerResponse
		response, err = ReadServerResponse(c.Stream)
		if err != nil {
			return
		}
		if !response.OK {
			return 0, E.New("remote error: " + response.Message)
		}
		c.responseRead = true
	}
	return c.Stream.Read(b)
}

func (c *ClientConn) Write(b []byte) (n int, err error) {
	if !c.requestWritten {
		err = WriteClientRequest(c.Stream, ClientRequest{
			UDP:  false,
			Host: c.destination.AddrString(),
			Port: c.destination.Port,
		}, b)
		if err != nil {
			return
		}
		c.requestWritten = true
		return len(b), nil
	}
	return c.Stream.Write(b)
}

func (c *ClientConn) LocalAddr() net.Addr {
	return nil
}

func (c *ClientConn) RemoteAddr() net.Addr {
	return c.destination.TCPAddr()
}

func (c *ClientConn) Upstream() any {
	return c.Stream
}
