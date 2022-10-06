package shadowtls

import (
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

var _ N.VectorisedWriter = (*ClientConn)(nil)

type ClientConn struct {
	*Conn
	hashConn *HashReadConn
}

func NewClientConn(hashConn *HashReadConn) *ClientConn {
	return &ClientConn{NewConn(hashConn.Conn), hashConn}
}

func (c *ClientConn) Write(p []byte) (n int, err error) {
	if c.hashConn != nil {
		sum := c.hashConn.Sum()
		c.hashConn = nil
		_, err = bufio.WriteVectorised(c.Conn, [][]byte{sum, p})
		if err == nil {
			n = len(p)
		}
		return
	}
	return c.Conn.Write(p)
}

func (c *ClientConn) WriteVectorised(buffers []*buf.Buffer) error {
	if c.hashConn != nil {
		sum := c.hashConn.Sum()
		c.hashConn = nil
		return c.Conn.WriteVectorised(append([]*buf.Buffer{buf.As(sum)}, buffers...))
	}
	return c.Conn.WriteVectorised(buffers)
}
